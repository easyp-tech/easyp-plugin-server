#!/usr/bin/env sh
# Spec-Driven Dev Pipeline — state machine (POSIX sh, zero dependencies)
# Usage: sh pipeline.sh [--feature <name>] <command> [args]
#
# Shell compatibility: requires sh with `local` support (bash, dash, ash, zsh).
#
# Global flags:
#   --feature <name>      Specify which feature to operate on (required when
#                         multiple pipelines are active simultaneously)
#
# Commands:
#   init [--branch|--no-branch] <feature-name>
#                         Start a new pipeline for a feature
#                         --branch: create git branch <prefix><name> (prefix from config, default: feature/)
#                         --no-branch: skip branch creation even if auto_branch is set in config
#   status                Show current phase, feature, and artifacts
#   approve               Advance to next phase (requires artifact)
#   artifact [path]       Register artifact for current phase
#   history               Show all features and their status
#   revisions [phase]     Show revision history for current or specified phase
#   docs-check            Check project documentation status
#   task <T-N>            Mark implementation task as completed (resume tracking)
#   version               Show version
#   help                  Show this help message

set -e

PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FEATURES_DIR="$PROJECT_ROOT/.spec/features"
CONFIG_FILE="$PROJECT_ROOT/.spec/config.yaml"

# --- helpers ---

VERSION="1.2.0"
EXPLICIT_FEATURE=""

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "→ $*"; }
warn() { echo "⚠ $*" >&2; }

iso_now() {
  date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ"
}

iso_now_compact() {
  date -u +"%Y-%m-%dT%H-%M-%SZ" 2>/dev/null || date +"%Y-%m-%dT%H-%M-%SZ"
}

# Escape a string for safe embedding in JSON values (RFC 8259)
json_escape() {
  printf '%s' "$1" | sed \
    -e 's/\\/\\\\/g' \
    -e 's/"/\\"/g' \
    -e 's/	/\\t/g' \
    -e 's/\r/\\r/g' | tr '\n' ' '
}

# Read a value from .spec/config.yaml (simple grep-based, no YAML parser)
# Usage: read_config <key> [default]
# Returns the value or default (empty string if no default)
read_config() {
  local key="$1" default="${2:-}"
  if [ -f "$CONFIG_FILE" ]; then
    local val
    val="$(grep "^${key}:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed "s/^${key}:[[:space:]]*//" | sed 's/[[:space:]]*$//')"
    if [ -n "$val" ]; then
      printf '%s' "$val"
      return
    fi
  fi
  printf '%s' "$default"
}

# --- per-feature state ---

# Current feature paths (set by set_feature_context / resolve_feature)
FEATURE_DIR=""
STATE_FILE=""
KV_FILE=""
REVISIONS_DIR=""
APPROVED_DIR=""

set_feature_context() {
  # set_feature_context <feature-name> — sets global paths for the feature
  FEATURE_DIR="$FEATURES_DIR/$1"
  KV_FILE="$FEATURE_DIR/pipeline.kv"
  STATE_FILE="$FEATURE_DIR/pipeline.json"
  REVISIONS_DIR="$FEATURE_DIR/revisions"
  APPROVED_DIR="$FEATURE_DIR/approved"
}

ensure_feature_dir() {
  # ensure_feature_dir <feature-name> — creates feature directory structure
  local fdir="$FEATURES_DIR/$1"
  mkdir -p "$fdir" "$fdir/revisions" "$fdir/approved"
}

read_field() {
  [ -f "$KV_FILE" ] || return 1
  grep "^$1=" "$KV_FILE" 2>/dev/null | head -1 | cut -d'=' -f2-
}

validate_kv() {
  # Verify required fields exist in KV store; die with diagnostic on failure
  [ -f "$KV_FILE" ] || die "Pipeline state file missing: $KV_FILE"
  local missing=""
  for field in feature phase created_at; do
    grep -q "^${field}=" "$KV_FILE" 2>/dev/null || missing="$missing $field"
  done
  if [ -n "$missing" ]; then
    die "Corrupted pipeline state ($KV_FILE): missing fields:$missing. Fix the file manually or remove and re-init."
  fi
  # Verify every line matches key=value format (key: lowercase + digits + underscore)
  local line_num=0
  while IFS= read -r line || [ -n "$line" ]; do
    line_num=$((line_num + 1))
    case "$line" in
      "") continue ;;  # skip blank lines
      [a-z_]*=*) ;;    # valid key=value
      *) die "Corrupted pipeline state ($KV_FILE): invalid line $line_num: $line" ;;
    esac
  done < "$KV_FILE"
}

# Escape a value for safe use in sed replacement string
kv_escape_sed() {
  printf '%s' "$1" | sed -e 's/[&\\/]/\\&/g'
}

# Validate that a value is safe for the KV store (no =, |, or newlines)
kv_validate_value() {
  case "$1" in
    *'='*) die "KV value must not contain '=': $1" ;;
    *'|'*) die "KV value must not contain '|': $1" ;;
  esac
  # Check for newlines by comparing line count (portable across POSIX shells)
  local line_count
  line_count="$(printf '%s' "$1" | wc -l)"
  if [ "$line_count" -ne 0 ]; then
    die "KV value must not contain newlines: $1"
  fi
}

write_field() {
  kv_validate_value "$2"
  if [ -f "$KV_FILE" ] && grep -q "^$1=" "$KV_FILE" 2>/dev/null; then
    local tmp="$KV_FILE.tmp"
    local escaped
    escaped="$(kv_escape_sed "$2")"
    sed "s|^$1=.*|$1=$escaped|" "$KV_FILE" > "$tmp" && mv "$tmp" "$KV_FILE"
  else
    echo "$1=$2" >> "$KV_FILE"
  fi
}

detect_active_feature() {
  # Scan all features, return the one with phase != done
  [ -d "$FEATURES_DIR" ] || return 0
  local active=""
  local count=0
  for kv in "$FEATURES_DIR"/*/pipeline.kv; do
    [ -f "$kv" ] || continue
    local phase
    phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    if [ -n "$phase" ] && [ "$phase" != "done" ]; then
      local fname
      fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      active="$fname"
      count=$((count + 1))
    fi
  done
  if [ "$count" -gt 1 ]; then
    warn "Multiple active pipelines found:"
    for kv in "$FEATURES_DIR"/*/pipeline.kv; do
      [ -f "$kv" ] || continue
      local phase fname
      phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
      if [ -n "$phase" ] && [ "$phase" != "done" ]; then
        echo "  - $fname (phase: $phase)" >&2
      fi
    done
    return 1
  fi
  [ -n "$active" ] && echo "$active"
}

resolve_feature() {
  if [ -n "$EXPLICIT_FEATURE" ]; then
    # Validate that the explicitly specified feature exists
    if [ ! -f "$FEATURES_DIR/$EXPLICIT_FEATURE/pipeline.kv" ]; then
      die "Feature '$EXPLICIT_FEATURE' not found. Run 'pipeline.sh history' to list features."
    fi
    set_feature_context "$EXPLICIT_FEATURE"
    validate_kv
    return 0
  fi
  local feat
  feat="$(detect_active_feature)" || { warn "Hint: use --feature <name> to select one."; return 1; }
  if [ -z "$feat" ]; then
    return 1
  fi
  set_feature_context "$feat"
  validate_kv
  return 0
}

next_phase() {
  case "$1" in
    explore)        echo "requirements" ;;
    requirements)   echo "design" ;;
    design)         echo "task-plan" ;;
    task-plan)      echo "implementation" ;;
    implementation) echo "review" ;;
    review)         echo "done" ;;
    done)           echo "" ;;
    *)              echo "" ;;
  esac
}

phase_number() {
  case "$1" in
    explore)        echo "1" ;;
    requirements)   echo "2" ;;
    design)         echo "3" ;;
    task-plan)      echo "4" ;;
    implementation) echo "5" ;;
    review)         echo "6" ;;
    done)           echo "✓" ;;
    *)              echo "?" ;;
  esac
}

# Rebuild the JSON file from KV store (for agents to read)
# Uses atomic write (tmp + mv) to prevent corruption on interruption
rebuild_json() {
  validate_kv
  local feature phase created artifact
  feature="$(json_escape "$(read_field feature)")"
  phase="$(read_field phase)"
  created="$(read_field created_at)"
  artifact="$(read_field current_artifact)"
  local history_count
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  local tmp_file="$STATE_FILE.tmp"
  {
    printf '{\n'
    printf '  "feature": "%s",\n' "$feature"
    printf '  "phase": "%s",\n' "$phase"
    printf '  "created_at": "%s",\n' "$created"
    if [ -n "$artifact" ]; then
      printf '  "current_artifact": "%s",\n' "$(json_escape "$artifact")"
    else
      printf '  "current_artifact": null,\n'
    fi
    printf '  "history": [\n'

    local i=0
    while [ "$i" -lt "$history_count" ]; do
      local h_phase h_artifact h_approved
      h_phase="$(read_field "history_${i}_phase")"
      h_artifact="$(json_escape "$(read_field "history_${i}_artifact")")"
      h_approved="$(read_field "history_${i}_approved_at")"
      [ "$i" -gt 0 ] && printf ',\n'
      printf '    {"phase": "%s", "artifact": "%s", "approved_at": "%s"}' \
        "$h_phase" "$h_artifact" "$h_approved"
      i=$((i + 1))
    done

    printf '\n  ],\n'

    # Include review_base_commit if set
    local rbc
    rbc="$(read_field review_base_commit 2>/dev/null || echo "")"
    if [ -n "$rbc" ]; then
      printf '  "review_base_commit": "%s",\n' "$(json_escape "$rbc")"
    else
      printf '  "review_base_commit": null,\n'
    fi

    # Include branch if set
    local br
    br="$(read_field branch 2>/dev/null || echo "")"
    if [ -n "$br" ]; then
      printf '  "branch": "%s",\n' "$(json_escape "$br")"
    else
      printf '  "branch": null,\n'
    fi

    # Include last_completed_task if set
    local lct
    lct="$(read_field last_completed_task 2>/dev/null || echo "")"
    if [ -n "$lct" ]; then
      printf '  "last_completed_task": "%s"\n' "$(json_escape "$lct")"
    else
      printf '  "last_completed_task": null\n'
    fi

    printf '}\n'
  } > "$tmp_file"
  mv -f "$tmp_file" "$STATE_FILE"
}

# --- commands ---

cmd_init() {
  # Parse init-specific flags
  local do_branch=""
  local feature=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --branch)    do_branch="yes"; shift ;;
      --no-branch) do_branch="no"; shift ;;
      -*)          die "Unknown flag for init: $1" ;;
      *)
        [ -n "$feature" ] && die "Unexpected argument: $1"
        feature="$1"; shift
        ;;
    esac
  done

  [ -z "$feature" ] && die "Usage: pipeline.sh init [--branch|--no-branch] <feature-name>"

  # Validate feature name (kebab-case)
  case "$feature" in
    *[!a-z0-9-]*) die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    -*|*-)        die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    *--*)         die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    [!a-z]*)      die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
  esac

  if [ ${#feature} -gt 64 ]; then
    die "Feature name too long (max 64 chars): $feature"
  fi

  # Resolve branch creation: flag > config > default (no branch)
  if [ -z "$do_branch" ]; then
    local auto_branch
    auto_branch="$(read_config auto_branch "false")"
    case "$auto_branch" in
      true|yes|1) do_branch="yes" ;;
      *)          do_branch="no" ;;
    esac
  fi

  local branch_name=""
  if [ "$do_branch" = "yes" ]; then
    # Verify git is available
    if ! command -v git >/dev/null 2>&1; then
      die "Git not found. Cannot create branch."
    fi
    if ! git rev-parse --git-dir >/dev/null 2>&1; then
      die "Not a git repository. Cannot create branch."
    fi

    local prefix
    prefix="$(read_config branch_prefix "feature/")"
    branch_name="${prefix}${feature}"

    # Check if branch already exists
    if git rev-parse --verify "$branch_name" >/dev/null 2>&1; then
      die "Branch '$branch_name' already exists."
    fi

    # Warn about dirty working tree
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
      warn "Working tree has uncommitted changes."
    fi

    git checkout -b "$branch_name" || die "Failed to create branch '$branch_name'."
    info "Created branch: $branch_name"
  fi

  local fdir="$FEATURES_DIR/$feature"

  # Check if feature already exists
  if [ -f "$fdir/pipeline.kv" ]; then
    local existing_phase
    existing_phase="$(grep "^phase=" "$fdir/pipeline.kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    if [ "$existing_phase" = "done" ]; then
      die "Feature '$feature' already completed. Choose a different name."
    else
      warn "Active pipeline for '$feature' exists (phase: $existing_phase)"
      die "Complete or choose a different feature name."
    fi
  fi

  ensure_feature_dir "$feature"
  set_feature_context "$feature"

  # Initialize KV store
  {
    echo "feature=$feature"
    echo "phase=explore"
    echo "created_at=$(iso_now)"
    echo "current_artifact="
    echo "history_count=0"
    if [ -n "$branch_name" ]; then
      echo "branch=$branch_name"
    fi
  } > "$KV_FILE"

  rebuild_json
  info "Pipeline initialized for '$feature'"
  if [ -n "$branch_name" ]; then
    info "Branch: $branch_name"
  fi
  info "Phase: [1/6] explore"
  info "Artifacts: .spec/features/$feature/"
  info "Read template: ./templates/explore.md"
}

cmd_status() {
  if ! resolve_feature; then
    info "No active pipeline."
    # Show completed features if any
    if [ -d "$FEATURES_DIR" ]; then
      local has_completed=0
      for kv in "$FEATURES_DIR"/*/pipeline.kv; do
        [ -f "$kv" ] || continue
        local phase
        phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
        if [ "$phase" = "done" ]; then
          if [ "$has_completed" -eq 0 ]; then
            echo ""
            echo "Completed features:"
            has_completed=1
          fi
          local fname
          fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
          printf "  ✓ %s\n" "$fname"
        fi
      done
    fi
    echo ""
    info "Run: pipeline.sh init <feature-name>"
    return 0
  fi

  local feature phase artifact history_count
  feature="$(read_field feature)"
  phase="$(read_field phase)"
  artifact="$(read_field current_artifact)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  echo ""
  echo "┌─────────────────────────────────────────────┐"
  printf "│ Feature: %-35s│\n" "$feature"
  printf "│ Phase:   [%s/6] %-30s│\n" "$(phase_number "$phase")" "$phase"
  if [ -n "$artifact" ]; then
    printf "│ Artifact: %-34s│\n" "$artifact"
  else
    printf "│ Artifact: %-34s│\n" "(none — register before approve)"
  fi
  # Show last completed task during implementation phase
  if [ "$phase" = "implementation" ]; then
    local lct
    lct="$(read_field last_completed_task 2>/dev/null || echo "")"
    if [ -n "$lct" ]; then
      printf "│ Last task: %-33s│\n" "$lct"
    fi
  fi
  echo "├─────────────────────────────────────────────┤"

  # Show pipeline progress
  local e_mark="○" r_mark="○" d_mark="○" t_mark="○" i_mark="○" rev_mark="○"
  case "$phase" in
    explore)        e_mark="●" ;;
    requirements)   e_mark="✓"; r_mark="●" ;;
    design)         e_mark="✓"; r_mark="✓"; d_mark="●" ;;
    task-plan)      e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="●" ;;
    implementation) e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="●" ;;
    review)         e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="✓"; rev_mark="●" ;;
    done)           e_mark="✓"; r_mark="✓"; d_mark="✓"; t_mark="✓"; i_mark="✓"; rev_mark="✓" ;;
  esac
  printf "│ %s Ex → %s Rq → %s Ds → %s Tp → %s Im → %s Rv │\n" "$e_mark" "$r_mark" "$d_mark" "$t_mark" "$i_mark" "$rev_mark"
  echo "└─────────────────────────────────────────────┘"

  # Show history
  if [ "$history_count" -gt 0 ]; then
    echo ""
    echo "Completed phases:"
    local i=0
    while [ "$i" -lt "$history_count" ]; do
      local h_phase h_artifact h_approved
      h_phase="$(read_field "history_${i}_phase")"
      h_artifact="$(read_field "history_${i}_artifact")"
      h_approved="$(read_field "history_${i}_approved_at")"
      printf "  [%s] %-15s → %s (approved: %s)\n" "$((i+1))" "$h_phase" "$h_artifact" "$h_approved"
      i=$((i + 1))
    done
  fi

  # Hint for next action
  echo ""
  if [ "$phase" = "done" ]; then
    info "Pipeline complete."
  elif [ -z "$artifact" ]; then
    info "Next: register artifact with 'pipeline.sh artifact <path>'"
    info "Then: 'pipeline.sh approve' after user approval"
  else
    info "Artifact registered. Ask user to approve, then run 'pipeline.sh approve'"
  fi
  echo ""
}

cmd_artifact() {
  resolve_feature || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "done" ] && die "Pipeline is complete. Nothing to register."

  local path="$1"

  # If no path given, use the default: .spec/features/<feature>/<phase>.md
  if [ -z "$path" ]; then
    path="$FEATURE_DIR/${phase}.md"
  fi

  # Validate artifact path: reject traversal, control characters
  case "$path" in
    *..*)  die "Artifact path must not contain '..' traversal" ;;
  esac
  if printf '%s' "$path" | grep -q '[[:cntrl:]]' 2>/dev/null; then
    die "Artifact path must not contain control characters"
  fi

  # Validate artifact file exists
  [ -f "$path" ] || die "Artifact file does not exist: $path"

  # Save a snapshot of the artifact being registered (revision tracking)
  local rev_count
  rev_count="$(read_field "revision_count_${phase}")"
  [ -z "$rev_count" ] && rev_count=0
  rev_count=$((rev_count + 1))
  local rev_name="${phase}-rev-${rev_count}-$(iso_now_compact).md"
  cp "$path" "$REVISIONS_DIR/$rev_name"
  write_field "revision_count_${phase}" "$rev_count"
  if [ "$rev_count" -gt 1 ]; then
    info "Revision $rev_count saved: $rev_name"
  fi

  write_field current_artifact "$path"
  rebuild_json
  info "Artifact registered for phase '$phase': $path"
}

cmd_approve() {
  resolve_feature || die "No active pipeline."

  local phase artifact history_count
  phase="$(read_field phase)"
  artifact="$(read_field current_artifact)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  [ "$phase" = "done" ] && die "Pipeline already complete."
  [ -z "$artifact" ] && die "No artifact registered for phase '$phase'. Run 'pipeline.sh artifact <path>' first."
  [ -f "$artifact" ] || die "Artifact file no longer exists: $artifact. Re-register with 'pipeline.sh artifact <path>'."

  # Snapshot artifact contents
  cp "$artifact" "$APPROVED_DIR/${phase}.md"

  # Record base commit for review phase (git diff source)
  if [ "$phase" = "task-plan" ]; then
    local base_commit
    base_commit="$(git rev-parse HEAD 2>/dev/null || echo "")"
    write_field review_base_commit "$base_commit"
  fi

  # Record in history
  write_field "history_${history_count}_phase" "$phase"
  write_field "history_${history_count}_artifact" "$artifact"
  write_field "history_${history_count}_approved_at" "$(iso_now)"
  history_count=$((history_count + 1))
  write_field history_count "$history_count"

  # Advance phase
  local next
  next="$(next_phase "$phase")"
  write_field phase "$next"
  write_field current_artifact ""

  # Clear task tracking when leaving implementation
  if [ "$phase" = "implementation" ]; then
    write_field last_completed_task ""
  fi

  rebuild_json

  if [ "$next" = "done" ]; then
    echo ""
    echo "✅ Pipeline complete!"
    echo ""
    echo "All artifacts:"
    local i=0
    while [ "$i" -lt "$history_count" ]; do
      printf "  [%s] %s → %s\n" "$((i+1))" \
        "$(read_field "history_${i}_phase")" \
        "$(read_field "history_${i}_artifact")"
      i=$((i + 1))
    done
    echo ""
    local feat
    feat="$(read_field feature)"
    info "Artifacts saved in: .spec/features/$feat/"
  else
    info "Phase '$phase' approved."
    info "Advanced to: [$(phase_number "$next")/6] $next"
    info "Read template: ./templates/${next}.md"
  fi
}

cmd_task() {
  local task_id="$1"
  [ -z "$task_id" ] && die "Usage: pipeline.sh task <T-N>"

  resolve_feature || die "No active pipeline."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "implementation" ] || die "Task tracking is only available during implementation phase (current: $phase)."

  write_field last_completed_task "$task_id"
  rebuild_json
  info "Task $task_id marked complete"
}

cmd_history() {
  if [ ! -d "$FEATURES_DIR" ]; then
    info "No features found."
    return 0
  fi

  local found=0
  for kv in "$FEATURES_DIR"/*/pipeline.kv; do
    [ -f "$kv" ] || continue
    found=1
    local fname phase created
    fname="$(grep "^feature=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    phase="$(grep "^phase=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
    created="$(grep "^created_at=" "$kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"

    local status_icon
    if [ "$phase" = "done" ]; then
      status_icon="✓"
    else
      status_icon="●"
    fi

    printf "  %s %-25s [%s/6] %-15s (created: %s)\n" \
      "$status_icon" "$fname" "$(phase_number "$phase")" "$phase" "$created"
  done

  if [ "$found" -eq 0 ]; then
    info "No features found."
  fi
}

cmd_revisions() {
  resolve_feature || die "No active pipeline."

  local phase
  phase="$(read_field phase)"

  local target_phase="${1:-$phase}"
  # Validate target phase
  case "$target_phase" in
    explore|requirements|design|task-plan|implementation|review|all) ;;
    *) die "Unknown phase: $target_phase. Use: explore, requirements, design, task-plan, implementation, review, or all." ;;
  esac

  local found=0
  local tmp
  tmp="$(mktemp)"
  if [ "$target_phase" = "all" ]; then
    echo "All revisions:"
    for p in explore requirements design task-plan implementation review; do
      find "$REVISIONS_DIR" -name "${p}-rev-*" 2>/dev/null | sort > "$tmp"
      while IFS= read -r f; do
        printf "  [%s] %s\n" "$p" "$(basename "$f")"
        found=1
      done < "$tmp"
    done
  else
    echo "Revisions for phase '$target_phase':"
    find "$REVISIONS_DIR" -name "${target_phase}-rev-*" 2>/dev/null | sort > "$tmp"
    while IFS= read -r f; do
      printf "  %s\n" "$(basename "$f")"
      found=1
    done < "$tmp"
  fi
  rm -f "$tmp"

  if [ "$found" -eq 0 ]; then
    info "No revisions recorded yet."
  fi
}

cmd_version() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
}

# check_file_staleness <file> <templates_dir> <freshness_days> <now_epoch>
# Outputs tab-separated: generated template age_days stale scope_changed
check_file_staleness() {
  local f="$1" templates_dir="$2" freshness_days="$3" now_epoch="$4"
  local generated="null" template="null" age_days="null" stale="false" scope_changed="null"

  local first_line
  first_line="$(head -1 "$f" 2>/dev/null)"
  case "$first_line" in
    *"<!-- generated:"*"template:"*"-->"*)
      local gen_date gen_tmpl
      gen_date="$(echo "$first_line" | sed 's/.*<!-- generated: \([0-9-]*\),.*/\1/')"
      gen_tmpl="$(echo "$first_line" | sed 's/.*template: \([^ ]*\) -->.*/\1/')"
      if [ -n "$gen_date" ]; then
        generated="\"$gen_date\""
        template="\"$gen_tmpl\""
        local gen_epoch
        gen_epoch="$(date -j -f '%Y-%m-%d' "$gen_date" '+%s' 2>/dev/null || date -d "$gen_date" '+%s' 2>/dev/null || echo 0)"
        if [ "$gen_epoch" -gt 0 ] && [ "$now_epoch" -gt 0 ]; then
          age_days=$(( (now_epoch - gen_epoch) / 86400 ))

          local tmpl_file="$templates_dir/$gen_tmpl"
          if [ -f "$tmpl_file" ]; then
            local scope_line
            scope_line="$(head -1 "$tmpl_file" 2>/dev/null)"
            case "$scope_line" in
              "<!-- scope:"*"-->")
                local patterns
                patterns="$(echo "$scope_line" | sed 's/<!-- scope: //' | sed 's/ -->//' | sed 's/,[[:space:]]*/\n/g' | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
                if [ -n "$patterns" ]; then
                  local git_hits
                  # shellcheck disable=SC2086
                  git_hits="$(cd "$PROJECT_ROOT" && git log --oneline --since="$gen_date" -- $patterns 2>/dev/null | head -1)"
                  if [ -n "$git_hits" ]; then
                    scope_changed="true"
                    if [ "$age_days" -gt "$freshness_days" ]; then
                      stale="true"
                    fi
                  else
                    scope_changed="false"
                  fi
                fi
                ;;
              *)
                if [ "$age_days" -gt "$freshness_days" ]; then
                  stale="true"
                fi
                ;;
            esac
          else
            if [ "$age_days" -gt "$freshness_days" ]; then
              stale="true"
            fi
          fi
        fi
      fi
      ;;
  esac
  printf '%s\t%s\t%s\t%s\t%s' "$generated" "$template" "$age_days" "$stale" "$scope_changed"
}

cmd_docs_check() {
  local config_file="$CONFIG_FILE"
  local docs_dir=".spec"
  local freshness_days=30

  # Read docs_dir and doc_freshness_days from config.yaml if it exists
  if [ -f "$config_file" ]; then
    local configured_dir
    configured_dir="$(grep '^docs_dir:' "$config_file" 2>/dev/null | head -1 | sed 's/^docs_dir:[[:space:]]*//' | sed 's/[[:space:]]*$//')"
    if [ -n "$configured_dir" ]; then
      docs_dir="$configured_dir"
    fi
    local configured_days
    configured_days="$(grep '^doc_freshness_days:' "$config_file" 2>/dev/null | head -1 | sed 's/^doc_freshness_days:[[:space:]]*//' | sed 's/[[:space:]]*$//')"
    if [ -n "$configured_days" ]; then
      freshness_days="$configured_days"
    fi
  fi

  local full_path="$PROJECT_ROOT/$docs_dir"
  local templates_dir="$SKILL_DIR/templates/docs"
  local now_epoch
  now_epoch="$(date +%s 2>/dev/null || echo 0)"

  if [ -d "$full_path" ]; then
    printf '{"exists": true, "dir": "%s", "freshness_days": %d, "files": [' "$(json_escape "$docs_dir")" "$freshness_days"
    local first=1
    local stale_names=""
    find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort | while IFS= read -r f; do
      local fname result generated template age_days stale scope_changed
      fname="$(basename "$f")"
      result="$(check_file_staleness "$f" "$templates_dir" "$freshness_days" "$now_epoch")"

      generated="$(printf '%s' "$result" | cut -f1)"
      template="$(printf '%s' "$result" | cut -f2)"
      age_days="$(printf '%s' "$result" | cut -f3)"
      stale="$(printf '%s' "$result" | cut -f4)"
      scope_changed="$(printf '%s' "$result" | cut -f5)"

      if [ "$first" -eq 1 ]; then
        first=0
      else
        printf ', '
      fi
      printf '{"name": "%s", "generated": %s, "template": %s, "age_days": %s, "stale": %s, "scope_changed": %s}' \
        "$(json_escape "$fname")" "$generated" "$template" "$age_days" "$stale" "$scope_changed"

      if [ "$stale" = "true" ]; then
        stale_names="$stale_names $fname"
      fi
    done
    printf '], "stale": ['
    # Re-scan for stale files (subshell above cannot export stale_names)
    local sfirst=1
    find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort | while IFS= read -r f; do
      local result stale fname
      result="$(check_file_staleness "$f" "$templates_dir" "$freshness_days" "$now_epoch")"
      stale="$(printf '%s' "$result" | cut -f4)"
      if [ "$stale" = "true" ]; then
        fname="$(basename "$f")"
        if [ "$sfirst" -eq 1 ]; then
          sfirst=0
        else
          printf ', '
        fi
        printf '"%s"' "$(json_escape "$fname")"
      fi
    done
    printf ']}\n'
  else
    printf '{"exists": false, "dir": "%s", "freshness_days": %d, "files": [], "stale": []}\n' "$(json_escape "$docs_dir")" "$freshness_days"
  fi
}

cmd_config_check() {
  [ -f "$CONFIG_FILE" ] || { info "No config file found: $CONFIG_FILE"; return 0; }

  local valid_keys=" context rules.explore rules.requirements rules.design rules.task-plan rules.implementation rules.review rules.docs test_skill test_reference docs_dir doc_freshness_days auto_branch branch_prefix "
  local errors=0

  info "Checking $CONFIG_FILE ..."

  # Extract keys and validate against whitelist
  local tmp
  tmp="$(mktemp)"
  grep '^[a-z]' "$CONFIG_FILE" 2>/dev/null > "$tmp" || true
  while IFS= read -r line; do
    local key
    key="$(printf '%s' "$line" | sed 's/:.*//')"
    case "$valid_keys" in
      *" $key "*) ;;
      *) warn "Unknown key: '$key'"; errors=$((errors + 1)) ;;
    esac
  done < "$tmp"
  rm -f "$tmp"

  # Type checks
  local val
  val="$(read_config doc_freshness_days "")"
  if [ -n "$val" ]; then
    case "$val" in
      *[!0-9]*) warn "doc_freshness_days must be numeric, got: '$val'"; errors=$((errors + 1)) ;;
    esac
  fi

  val="$(read_config auto_branch "")"
  if [ -n "$val" ]; then
    case "$val" in
      true|false|yes|no|1|0) ;;
      *) warn "auto_branch must be boolean (true/false/yes/no/1/0), got: '$val'"; errors=$((errors + 1)) ;;
    esac
  fi

  if [ "$errors" -eq 0 ]; then
    info "Config OK — all keys valid."
  else
    warn "$errors problem(s) found."
    return 1
  fi
}

cmd_inject() {
  local target_phase="${1:-}"
  local artifact_path="${2:-}"

  if [ -z "$target_phase" ] || [ -z "$artifact_path" ]; then
    die "Usage: pipeline.sh inject <phase> <path>"
  fi

  # Validate target phase
  case "$target_phase" in
    explore|requirements|design|task-plan|implementation|review) ;;
    *) die "Unknown phase: $target_phase. Use: explore, requirements, design, task-plan, implementation, review." ;;
  esac

  resolve_feature || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local current_phase
  current_phase="$(read_field phase)"
  [ "$current_phase" = "done" ] && die "Pipeline already complete."

  # Validate current phase <= target phase
  local current_num target_num
  current_num="$(phase_number "$current_phase")"
  target_num="$(phase_number "$target_phase")"
  if [ "$current_num" -gt "$target_num" ]; then
    die "Cannot inject backward: current phase is '$current_phase' ($current_num), target is '$target_phase' ($target_num)."
  fi

  # Validate artifact exists
  [ -f "$artifact_path" ] || die "Artifact file does not exist: $artifact_path"

  # Validate artifact path: reject traversal, control characters
  case "$artifact_path" in
    *..*)  die "Artifact path must not contain '..' traversal" ;;
  esac
  if printf '%s' "$artifact_path" | grep -q '[[:cntrl:]]' 2>/dev/null; then
    die "Artifact path must not contain control characters"
  fi

  # Lightweight content validation
  case "$target_phase" in
    requirements)
      if ! grep -q 'WHEN\|SHALL' "$artifact_path" 2>/dev/null; then
        warn "Requirements artifact should contain WHEN/SHALL keywords."
      fi
      ;;
    design)
      if ! grep -q 'Correctness\|Property' "$artifact_path" 2>/dev/null; then
        warn "Design artifact should contain Correctness Properties."
      fi
      ;;
  esac

  # Skip intermediate phases (record as injected in history)
  local p="$current_phase"
  local history_count
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  while [ "$p" != "$target_phase" ]; do
    write_field "history_${history_count}_phase" "$p"
    write_field "history_${history_count}_artifact" "(injected)"
    write_field "history_${history_count}_approved_at" "$(iso_now)"
    history_count=$((history_count + 1))
    p="$(next_phase "$p")"
  done

  # Set to target phase and register artifact
  write_field phase "$target_phase"
  write_field current_artifact "$artifact_path"
  write_field history_count "$history_count"

  # Save revision snapshot
  local rev_count
  rev_count="$(read_field "revision_count_${target_phase}")"
  [ -z "$rev_count" ] && rev_count=0
  rev_count=$((rev_count + 1))
  local rev_name
  rev_name="${target_phase}-rev-${rev_count}-$(iso_now_compact).md"
  cp "$artifact_path" "$REVISIONS_DIR/$rev_name"
  write_field "revision_count_${target_phase}" "$rev_count"

  rebuild_json

  local skipped=$((target_num - current_num))
  if [ "$skipped" -gt 0 ]; then
    info "$skipped phase(s) skipped to reach '$target_phase'."
  fi
  info "Artifact injected for phase '$target_phase': $artifact_path"
  info "Ask user to approve, then run 'pipeline.sh approve'"
}

cmd_abandon() {
  local feature="${1:-}"

  # If --feature was specified globally, use it
  if [ -z "$feature" ] && [ -n "$EXPLICIT_FEATURE" ]; then
    feature="$EXPLICIT_FEATURE"
  fi

  # If still empty, try to resolve active feature
  if [ -z "$feature" ]; then
    feature="$(detect_active_feature)" || die "Multiple active pipelines. Use: pipeline.sh abandon <feature-name>"
    [ -z "$feature" ] && die "No active pipeline to abandon."
  fi

  local fdir="$FEATURES_DIR/$feature"
  [ -f "$fdir/pipeline.kv" ] || die "Feature '$feature' not found."

  local phase
  phase="$(grep "^phase=" "$fdir/pipeline.kv" 2>/dev/null | head -1 | cut -d'=' -f2-)"
  [ "$phase" = "done" ] && die "Feature '$feature' is already completed."

  set_feature_context "$feature"
  write_field phase "done"
  write_field abandoned_at "$(iso_now)"
  rebuild_json

  info "Feature '$feature' abandoned (was in phase: $phase)."
  info "Artifacts remain in: .spec/features/$feature/"
}

cmd_help() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
  echo ""
  echo "Usage: sh pipeline.sh [--feature <name>] <command> [args]"
  echo ""
  echo "Global flags:"
  echo "  --feature <name>  Select feature (needed when multiple are active)"
  echo ""
  echo "Commands:"
  echo "  init [--branch|--no-branch] <feature>"
  echo "                    Start a new pipeline (kebab-case name)"
  echo "                    --branch: create git branch <prefix><name>"
  echo "                    --no-branch: skip auto-branch from config"
  echo "  status            Show current phase, artifacts, progress"
  echo "  artifact [path]   Register output artifact for current phase"
  echo "  approve           Advance to next phase (needs artifact)"
  echo "  revisions [phase] Show revision history (current phase or specify: explore, all)"
  echo "  history           Show all features and their status"
  echo "  docs-check        Check project documentation status (JSON)"
  echo "  task <T-N>        Mark implementation task as completed (resume tracking)"
  echo "  config-check      Validate .spec/config.yaml keys and types"
  echo "  inject <phase> <path>"
  echo "                    Inject pre-written artifact and skip to that phase"
  echo "  abandon [feature] Abandon an active pipeline (marks as done)"
  echo "  version           Show version"
  echo "  help              Show this message"
  echo ""
  echo "Workflow (6 phases):"
  echo "  1. init my-feature"
  echo "  2. (agent reads templates/explore.md, investigates)"
  echo "  3. artifact  ← writes .spec/features/my-feature/explore.md"
  echo "  4. approve   ← user confirms"
  echo "  5. (agent reads templates/requirements.md, generates doc)"
  echo "  6. artifact  ← writes .spec/features/my-feature/requirements.md"
  echo "  7. approve   ← user confirms"
  echo "  8. (agent reads templates/design.md, generates doc)"
  echo "  9. artifact  ← writes .spec/features/my-feature/design.md"
  echo " 10. approve   ← user confirms"
  echo " 11. (agent reads templates/task-plan.md, creates TDD plan)"
  echo " 12. artifact  ← writes .spec/features/my-feature/task-plan.md"
  echo " 13. approve   ← user confirms"
  echo " 14. (agent reads templates/implementation.md, executes TDD plan)"
  echo " 15. artifact  ← writes .spec/features/my-feature/implementation.md"
  echo " 16. approve   ← user confirms"
  echo " 17. (agent reads templates/review.md, reviews code)"
  echo " 18. artifact  ← writes .spec/features/my-feature/review.md"
  echo " 19. approve   ← user confirms → done!"
  echo ""
  echo "All artifacts are saved permanently in .spec/features/<feature>/ and tracked by git."
  echo "Tip: use 'revisions' to see previous versions of an artifact within a phase."
}

# --- main ---

# Parse global flags before command dispatch
while [ $# -gt 0 ]; do
  case "$1" in
    --feature)
      [ -n "$2" ] || die "--feature requires a value"
      EXPLICIT_FEATURE="$2"
      shift 2
      ;;
    *) break ;;
  esac
done

case "${1:-help}" in
  init)     shift; cmd_init "$@" ;;
  status)   cmd_status ;;
  artifact) cmd_artifact "$2" ;;
  approve)  cmd_approve ;;
  revisions) cmd_revisions "$2" ;;
  history)  cmd_history ;;
  docs-check) cmd_docs_check ;;
  task)     cmd_task "$2" ;;
  config-check) cmd_config_check ;;
  inject)   shift; cmd_inject "$@" ;;
  abandon)  shift; cmd_abandon "$@" ;;
  version|--version|-v) cmd_version ;;
  help|--help|-h) cmd_help ;;
  *)        die "Unknown command: $1. Run 'pipeline.sh help' for usage." ;;
esac
