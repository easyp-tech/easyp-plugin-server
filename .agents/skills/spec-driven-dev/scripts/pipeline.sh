#!/usr/bin/env sh
# Spec-Driven Dev Pipeline — state machine (POSIX sh, zero dependencies)
# Usage: sh pipeline.sh <command> [args]
#
# Shell compatibility: requires sh with `local` support (bash, dash, ash, zsh).
#
# Commands:
#   init <feature-name>   Start a new pipeline for a feature
#   status                Show current phase, feature, and artifacts
#   approve               Advance to next phase (requires artifact)
#   artifact <path>       Register artifact for current phase
#   reset                 Reset the pipeline (with confirmation)
#   rollback              Return to previous phase
#   history               Show full history of completed phases
#   docs-check            Check project documentation status
#   version               Show version
#   help                  Show this help message

set -e

PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
STATE_DIR="$PROJECT_ROOT/.spec-driven-dev/state"
STATE_FILE="$STATE_DIR/pipeline.json"
HISTORY_DIR="$STATE_DIR/archive"
SPECS_DIR="$PROJECT_ROOT/.spec-driven-dev/specs"

# --- helpers ---

VERSION="1.11.0"

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

ensure_state_dir() {
  mkdir -p "$STATE_DIR" "$HISTORY_DIR"
}

# Minimal JSON helpers — no jq required
# We store state as a simple key=value file + a JSON mirror for agents to read

read_field() {
  # read_field <key> — reads from the KV store
  local kv_file="$STATE_DIR/pipeline.kv"
  [ -f "$kv_file" ] || return 1
  grep "^$1=" "$kv_file" 2>/dev/null | head -1 | cut -d'=' -f2-
}

write_field() {
  # write_field <key> <value>
  local kv_file="$STATE_DIR/pipeline.kv"
  if [ -f "$kv_file" ] && grep -q "^$1=" "$kv_file" 2>/dev/null; then
    # Replace existing
    local tmp="$kv_file.tmp"
    sed "s|^$1=.*|$1=$2|" "$kv_file" > "$tmp" && mv "$tmp" "$kv_file"
  else
    echo "$1=$2" >> "$kv_file"
  fi
}

pipeline_active() {
  [ -f "$STATE_DIR/pipeline.kv" ] && [ -n "$(read_field feature)" ]
}

next_phase() {
  case "$1" in
    explore)        echo "requirements" ;;
    requirements)   echo "design" ;;
    design)         echo "implementation" ;;
    implementation) echo "done" ;;
    done)           echo "" ;;
    *)              echo "" ;;
  esac
}

phase_number() {
  case "$1" in
    explore)        echo "1" ;;
    requirements)   echo "2" ;;
    design)         echo "3" ;;
    implementation) echo "4" ;;
    done)           echo "✓" ;;
    *)              echo "?" ;;
  esac
}

# Rebuild the JSON file from KV store (for agents to read)
# Uses atomic write (tmp + mv) to prevent corruption on interruption
rebuild_json() {
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

    printf '\n  ]\n'
    printf '}\n'
  } > "$tmp_file"
  mv -f "$tmp_file" "$STATE_FILE"
}

# --- commands ---

cmd_init() {
  local feature="$1"
  [ -z "$feature" ] && die "Usage: pipeline.sh init <feature-name>"

  # Validate feature name (kebab-case)
  case "$feature" in
    *[!a-z0-9-]*) die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    -*|*-)        die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    *--*)         die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
    [!a-z]*)      die "Feature name must be kebab-case (e.g. grpc-streaming-support)" ;;
  esac

  ensure_state_dir

  if pipeline_active; then
    local existing
    existing="$(read_field feature)"
    warn "Active pipeline exists: '$existing' (phase: $(read_field phase))"
    printf "Reset and start new? [y/N] "
    read -r answer
    case "$answer" in
      [yY]*) cmd_reset_force ;;
      *)     die "Aborted. Use 'pipeline.sh reset' first." ;;
    esac
  fi

  # Initialize KV store
  cat > "$STATE_DIR/pipeline.kv" <<EOF
feature=$feature
phase=explore
created_at=$(iso_now)
current_artifact=
history_count=0
EOF

  rebuild_json
  info "Pipeline initialized for '$feature'"
  info "Phase: [1/4] explore"
  info "Read template: ./templates/explore.md"
}

cmd_status() {
  if ! pipeline_active; then
    info "No active pipeline."
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
  printf "│ Phase:   [%s/4] %-30s│\n" "$(phase_number "$phase")" "$phase"
  if [ -n "$artifact" ]; then
    printf "│ Artifact: %-34s│\n" "$artifact"
  else
    printf "│ Artifact: %-34s│\n" "(none — register before approve)"
  fi
  echo "├─────────────────────────────────────────────┤"

  # Show pipeline progress
  local e_mark="○" r_mark="○" d_mark="○" i_mark="○"
  case "$phase" in
    explore)        e_mark="●" ;;
    requirements)   e_mark="✓"; r_mark="●" ;;
    design)         e_mark="✓"; r_mark="✓"; d_mark="●" ;;
    implementation) e_mark="✓"; r_mark="✓"; d_mark="✓"; i_mark="●" ;;
    done)           e_mark="✓"; r_mark="✓"; d_mark="✓"; i_mark="✓" ;;
  esac
  printf "│ %s Explore → %s Req → %s Design → %s Impl   │\n" "$e_mark" "$r_mark" "$d_mark" "$i_mark"
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
    info "Pipeline complete. Run 'pipeline.sh reset' to start a new feature."
  elif [ -z "$artifact" ]; then
    info "Next: register artifact with 'pipeline.sh artifact <path>'"
    info "Then: 'pipeline.sh approve' after user approval"
  else
    info "Artifact registered. Ask user to approve, then run 'pipeline.sh approve'"
  fi
  echo ""
}

cmd_artifact() {
  local path="$1"
  [ -z "$path" ] && die "Usage: pipeline.sh artifact <path>"

  # Validate artifact path: reject traversal, control characters
  case "$path" in
    *..*)  die "Artifact path must not contain '..' traversal" ;;
  esac
  # Reject control characters (ASCII 0x00-0x1F except tab)
  if printf '%s' "$path" | grep -q '[[:cntrl:]]' 2>/dev/null; then
    die "Artifact path must not contain control characters"
  fi

  # Validate artifact file exists
  [ -f "$path" ] || die "Artifact file does not exist: $path"

  pipeline_active || die "No active pipeline. Run 'pipeline.sh init <feature>' first."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "done" ] && die "Pipeline is complete. Nothing to register."

  write_field current_artifact "$path"
  rebuild_json
  info "Artifact registered for phase '$phase': $path"
}

cmd_approve() {
  pipeline_active || die "No active pipeline."

  local phase artifact history_count
  phase="$(read_field phase)"
  artifact="$(read_field current_artifact)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  [ "$phase" = "done" ] && die "Pipeline already complete."
  [ -z "$artifact" ] && die "No artifact registered for phase '$phase'. Run 'pipeline.sh artifact <path>' first."

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
    info "Run 'pipeline.sh reset' when ready for a new feature."
  else
    info "Phase '$phase' approved."
    info "Advanced to: [$(phase_number "$next")/4] $next"
    info "Read template: ./templates/${next}.md"
  fi
}

cmd_history() {
  if ! pipeline_active; then
    # Check archive
    if [ -d "$HISTORY_DIR" ] && [ "$(find "$HISTORY_DIR" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
      echo "Archived pipelines:"
      ls -1 "$HISTORY_DIR"
    else
      info "No pipeline history."
    fi
    return 0
  fi

  local feature history_count
  feature="$(read_field feature)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  echo "Pipeline: $feature"
  echo "Phase:    $(read_field phase)"
  echo ""

  if [ "$history_count" -eq 0 ]; then
    info "No completed phases yet."
    return 0
  fi

  echo "Completed phases:"
  local i=0
  while [ "$i" -lt "$history_count" ]; do
    local h_phase h_artifact h_approved
    h_phase="$(read_field "history_${i}_phase")"
    h_artifact="$(read_field "history_${i}_artifact")"
    h_approved="$(read_field "history_${i}_approved_at")"
    printf "  Phase %s: %-15s\n" "$((i+1))" "$h_phase"
    printf "    Artifact:  %s\n" "$h_artifact"
    printf "    Approved:  %s\n" "$h_approved"
    i=$((i + 1))
  done
}

cmd_reset_force() {
  # Archive current pipeline before reset
  if pipeline_active; then
    local feature
    feature="$(read_field feature)"
    local ts
    ts="$(iso_now_compact)"
    local archive_name="${ts}-${feature}"
    local archive_path="$HISTORY_DIR/$archive_name"

    # Handle collision: append counter if directory exists
    if [ -d "$archive_path" ]; then
      local n=2
      while [ -d "${archive_path}-${n}" ]; do
        n=$((n + 1))
      done
      archive_path="${archive_path}-${n}"
    fi

    if [ -f "$STATE_FILE" ] || [ -f "$STATE_DIR/pipeline.kv" ]; then
      mkdir -p "$archive_path"
      cp "$STATE_FILE" "$archive_path/pipeline.json" 2>/dev/null || true
      cp "$STATE_DIR/pipeline.kv" "$archive_path/pipeline.kv" 2>/dev/null || true
    fi
  fi

  rm -f "$STATE_DIR/pipeline.kv" "$STATE_FILE"
}

cmd_reset() {
  if ! pipeline_active; then
    info "No active pipeline to reset."
    return 0
  fi

  local feature phase
  feature="$(read_field feature)"
  phase="$(read_field phase)"

  warn "This will archive and reset pipeline '$feature' (phase: $phase)"
  printf "Continue? [y/N] "
  read -r answer
  case "$answer" in
    [yY]*) ;;
    *)     die "Aborted." ;;
  esac

  cmd_reset_force
  info "Pipeline archived and reset."
  info "Run 'pipeline.sh init <feature-name>' to start a new pipeline."
}

cmd_rollback() {
  pipeline_active || die "No active pipeline."

  local phase history_count
  phase="$(read_field phase)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  [ "$history_count" -eq 0 ] && die "Nothing to roll back — no phases have been approved yet."

  # Determine previous phase from history
  local prev_index prev_phase prev_artifact
  prev_index=$((history_count - 1))
  prev_phase="$(read_field "history_${prev_index}_phase")"
  prev_artifact="$(read_field "history_${prev_index}_artifact")"

  warn "This will roll back from '$phase' to '$prev_phase' (artifact: $prev_artifact)"
  printf "Continue? [y/N] "
  read -r answer
  case "$answer" in
    [yY]*) ;;
    *)     die "Aborted." ;;
  esac

  # Restore previous phase and artifact
  write_field phase "$prev_phase"
  write_field current_artifact "$prev_artifact"

  # Remove history entry
  write_field history_count "$prev_index"
  # Note: orphaned history_N_ keys remain in KV but are ignored (history_count is the source of truth)

  rebuild_json
  info "Rolled back to phase '$prev_phase' with artifact: $prev_artifact"
  info "You can revise the artifact, re-register, and approve again."
}

cmd_publish() {
  pipeline_active || die "No active pipeline."

  local phase
  phase="$(read_field phase)"
  [ "$phase" = "done" ] || die "Pipeline is not complete (current phase: $phase). Finish all phases before publishing."

  local feature history_count
  feature="$(read_field feature)"
  history_count="$(read_field history_count)"
  [ -z "$history_count" ] && history_count=0

  local dest="$SPECS_DIR/$feature"
  mkdir -p "$dest"

  local i=0
  while [ "$i" -lt "$history_count" ]; do
    local h_phase h_artifact dest_name
    h_phase="$(read_field "history_${i}_phase")"
    h_artifact="$(read_field "history_${i}_artifact")"
    dest_name="$((i + 1))-${h_phase}.md"

    if [ -f "$h_artifact" ]; then
      cp "$h_artifact" "$dest/$dest_name"
      info "Published: $dest/$dest_name"
    else
      warn "Artifact not found, skipped: $h_artifact"
    fi
    i=$((i + 1))
  done

  echo ""
  info "Artifacts published to: $dest"
  info "These files are outside .spec-driven-dev/state/ and can be committed to git."
}

cmd_version() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
}

cmd_docs_check() {
  local config_file="$PROJECT_ROOT/.spec-driven-dev/config.yaml"
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
  local now_epoch
  now_epoch="$(date +%s 2>/dev/null || echo 0)"
  local threshold=$((freshness_days * 86400))

  if [ -d "$full_path" ]; then
    printf '{"exists": true, "dir": "%s", "freshness_days": %d, "files": [' "$(json_escape "$docs_dir")" "$freshness_days"
    local first=1
    for f in $(find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort); do
      local fname generated template age_days stale
      fname="$(basename "$f")"

      # Parse freshness comment: <!-- generated: YYYY-MM-DD, template: name.md -->
      generated="null"
      template="null"
      age_days="null"
      stale="false"
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
            # Compute age in days
            local gen_epoch
            gen_epoch="$(date -j -f '%Y-%m-%d' "$gen_date" '+%s' 2>/dev/null || date -d "$gen_date" '+%s' 2>/dev/null || echo 0)"
            if [ "$gen_epoch" -gt 0 ] && [ "$now_epoch" -gt 0 ]; then
              age_days=$(( (now_epoch - gen_epoch) / 86400 ))
              if [ "$age_days" -gt "$freshness_days" ]; then
                stale="true"
              fi
            fi
          fi
          ;;
      esac

      if [ "$first" -eq 1 ]; then
        first=0
      else
        printf ', '
      fi
      printf '{"name": "%s", "generated": %s, "template": %s, "age_days": %s, "stale": %s}' \
        "$(json_escape "$fname")" "$generated" "$template" "$age_days" "$stale"
    done
    printf '], "stale": ['
    # Collect stale file names
    local sfirst=1
    for f in $(find "$full_path" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort); do
      local first_line fname
      fname="$(basename "$f")"
      first_line="$(head -1 "$f" 2>/dev/null)"
      case "$first_line" in
        *"<!-- generated:"*"template:"*"-->"*)
          local gen_date gen_epoch
          gen_date="$(echo "$first_line" | sed 's/.*<!-- generated: \([0-9-]*\),.*/\1/')"
          gen_epoch="$(date -j -f '%Y-%m-%d' "$gen_date" '+%s' 2>/dev/null || date -d "$gen_date" '+%s' 2>/dev/null || echo 0)"
          if [ "$gen_epoch" -gt 0 ] && [ "$now_epoch" -gt 0 ]; then
            local file_age=$(( (now_epoch - gen_epoch) / 86400 ))
            if [ "$file_age" -gt "$freshness_days" ]; then
              if [ "$sfirst" -eq 1 ]; then
                sfirst=0
              else
                printf ', '
              fi
              printf '"%s"' "$(json_escape "$fname")"
            fi
          fi
          ;;
      esac
    done
    printf ']}\n'
  else
    printf '{"exists": false, "dir": "%s", "freshness_days": %d, "files": [], "stale": []}\n' "$(json_escape "$docs_dir")" "$freshness_days"
  fi
}

cmd_help() {
  echo "Spec-Driven Dev Pipeline v${VERSION}"
  echo ""
  echo "Usage: sh pipeline.sh <command> [args]"
  echo ""
  echo "Commands:"
  echo "  init <feature>    Start a new pipeline (kebab-case name)"
  echo "  status            Show current phase, artifacts, progress"
  echo "  artifact <path>   Register output artifact for current phase"
  echo "  approve           Advance to next phase (needs artifact)"
  echo "  rollback          Return to previous phase"
  echo "  history           Show completed phases and archived pipelines"
  echo "  reset             Archive current pipeline and reset"
  echo "  publish           Copy approved artifacts to .spec-driven-dev/specs/ (committable)"
  echo "  docs-check        Check project documentation status (JSON)"
  echo "  version           Show version"
  echo "  help              Show this message"
  echo ""
  echo "Workflow:"
  echo "  1. init my-feature"
  echo "  2. (agent reads templates/explore.md, investigates)"
  echo "  3. artifact state/explore.md"
  echo "  4. approve  ← user confirms"
  echo "  5. (agent reads templates/requirements.md, generates doc)"
  echo "  6. artifact state/requirements.md"
  echo "  7. approve  ← user confirms"
  echo "  8. (agent reads templates/design.md, generates doc)"
  echo "  9. artifact state/design.md"
  echo " 10. approve  ← user confirms"
  echo " 11. (agent reads templates/implementation.md, generates plan)"
  echo " 12. artifact state/implementation.md"
  echo " 13. approve  ← user confirms → done!"
}

# --- main ---

ensure_state_dir

case "${1:-help}" in
  init)     cmd_init "$2" ;;
  status)   cmd_status ;;
  artifact) cmd_artifact "$2" ;;
  approve)  cmd_approve ;;
  rollback) cmd_rollback ;;
  history)  cmd_history ;;
  reset)    cmd_reset ;;
  publish)  cmd_publish ;;
  docs-check) cmd_docs_check ;;
  version|--version|-v) cmd_version ;;
  help|--help|-h) cmd_help ;;
  *)        die "Unknown command: $1. Run 'pipeline.sh help' for usage." ;;
esac
