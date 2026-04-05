---
name: spec-driven-dev
description: >
  Spec-driven development pipeline with 4 phases: Explore, Requirements,
  Design, Implementation Plan. Enforces human approval gates between
  phases. Use when user wants structured feature development, spec-first
  approach, or says "I want to add feature X", "new feature", "implement",
  "build". Keywords: spec, requirements, design document, TDD plan, pipeline,
  approval gates, WHEN/SHALL, implementation plan.
---

# Spec-Driven Development

You are operating in **spec-driven development mode**.
This project uses a 4-phase pipeline with human approval gates between each phase.

## Pipeline

```
Explore → [APPROVE] → Requirements → [APPROVE] → Design → [APPROVE] → Implementation → [APPROVE] → Done
```

Each phase has a dedicated prompt template. Read the template for the **current** phase before generating any output.

## Phases

| # | Phase          | Template                      | Produces                        |
|---|----------------|-------------------------------|---------------------------------|
| 1 | Explore        | `./templates/explore.md`      | Exploration & research document |
| 2 | Requirements   | `./templates/requirements.md` | Formal requirements document    |
| 3 | Design         | `./templates/design.md`       | Architecture & design document  |
| 4 | Implementation | `./templates/implementation.md` | TDD implementation plan       |

## State Machine

The pipeline state is managed via a shell script. Use these commands:

```sh
# Check current phase and progress
sh ./scripts/pipeline.sh status

# Start a new feature pipeline
sh ./scripts/pipeline.sh init <feature-name>

# Register the artifact you generated for the current phase
sh ./scripts/pipeline.sh artifact <path-to-artifact>

# Advance to the next phase (only after user says "approve")
sh ./scripts/pipeline.sh approve

# Return to the previous phase (undo an approval)
sh ./scripts/pipeline.sh rollback

# View history of completed phases and archived pipelines
sh ./scripts/pipeline.sh history

# Archive and reset the pipeline
sh ./scripts/pipeline.sh reset

# Publish approved artifacts to committable directory
sh ./scripts/pipeline.sh publish

# Check project documentation status
sh ./scripts/pipeline.sh docs-check
```

## Project Configuration

If the file `.spec-driven-dev/config.yaml` exists in the project root, read it before starting any phase.

- **`context`** — project-wide background (tech stack, conventions, repo structure). Treat as extra context for ALL phases.
- **`rules.<phase>`** — phase-specific rules that supplement (not replace) the template instructions.
- **`test_skill`** (optional) — name of an installed skill for test generation. If present, delegate test specification (Design §2.8) and test task creation (Implementation) to this skill. Pass Correctness Properties and Coverage Matrix as input.
- **`test_reference`** (optional) — glob or file paths pointing to representative test files. If present, use these as the style reference for all generated tests. If absent, auto-discover adjacent tests.
- **`docs_dir`** (optional) — directory for project documentation, default: `.spec`. The agent reads documentation from this directory for project context and writes generated docs here.
- **`doc_freshness_days`** (optional) — number of days after which a generated doc is considered stale, default: `30`. Used by `pipeline.sh docs-check` to flag outdated documentation.
- **`rules.docs`** (optional) — rules for documentation generation, analogous to `rules.explore` etc. Example: `"Skip FILES.md — no file storage"`, `"Always include Mermaid diagrams in ARCHITECTURE.md"`.

Injection order: **context → phase rules → template instructions.**

If the file does not exist, skip this step.

## Pre-flight Checklist

Before starting any pipeline work, follow these steps in order:

1. **Check pipeline state**: run `pipeline.sh status`.
   - If an active pipeline exists → resume from the current phase. Do NOT run `init` again.
   - If no active pipeline → proceed to step 2.
2. **Read project config**: check if `.spec-driven-dev/config.yaml` exists.
   - If yes → read it, apply `context` to all phases, note `rules.*` for each phase.
   - If no → proceed without config (defaults apply).
3. **Check documentation**: run `pipeline.sh docs-check`.
   - **Docs directory missing** → suggest: *"Project documentation (<docs_dir>/) not found. I can generate it to better understand your codebase. Say 'generate docs' or 'skip'."* This is a soft suggestion — the pipeline works without documentation.
   - **Docs exist, stale files found** → suggest: *"Some docs are outdated (<file>: <N> days old). Regenerate before starting? Say 'update docs' or 'skip'."* If user agrees, follow the Stale doc regeneration workflow below.
   - **Docs exist, all fresh** → use as supplementary context for ALL phases. Read `<docs_dir>/README.md` for the documentation map.
4. **Start pipeline**: run `pipeline.sh init <feature-name>`.

## Documentation Context

The skill supports a self-documenting mechanic via a project documentation directory (default: `.spec/`, configurable via `docs_dir` in `config.yaml`).

### Pre-pipeline check

When running `pipeline.sh init <feature-name>`, before starting the Explore phase:

1. Determine the docs directory: read `docs_dir` from `.spec-driven-dev/config.yaml`. If not set, default to `.spec`.
2. Run `pipeline.sh docs-check` to determine if documentation exists and check freshness.
3. If the docs directory **exists and contains `README.md`**:
   - Read `README.md` for the documentation map
   - Use available docs (`ARCHITECTURE.md`, `PACKAGES.md`, etc.) as supplementary context for ALL phases
   - This is richer than `config.yaml` context and reduces the file-read budget in Explore phase
   - Check the `stale` array in docs-check output. If stale files exist, suggest: *"Some docs are outdated (<file>: <N> days old). Regenerate before starting? Say 'update docs' or 'skip'."*
4. If the docs directory **does not exist**:
   - Suggest to the user: *"Project documentation (<docs_dir>/) not found. I can generate it to better understand your codebase. Say 'generate docs' or 'skip'."*
   - If user says **"generate docs"**: read `./templates/docs/README.md` (manifest), then execute each template sequentially, saving results to `<docs_dir>/`
   - If user says **"skip"**: proceed with the pipeline normally — documentation is NOT required
   - **This is a soft suggestion, not a blocker.** The pipeline works without documentation.

### Stale doc regeneration workflow

When `pipeline.sh docs-check` reports stale files (or the user requests a doc update), follow these steps:

1. Parse the `docs-check` JSON output — read the `stale` array.
2. For each stale file, extract the `template` field from its freshness metadata.
3. Group stale files by template (one template may generate multiple files).
4. For each affected template:
   a. Read the template from `./templates/docs/<template>.md`.
   b. Read the existing generated file(s) as baseline — preserve project-specific content where possible.
   c. Regenerate following the template instructions.
   d. Update the freshness metadata: `<!-- generated: YYYY-MM-DD, template: <name>.md -->`.
5. Present updated files to the user for review before saving.
6. **Never auto-overwrite.** Always confirm with the user.

Use this lookup table to find the owner template for any generated file:

| Generated file | Owner template |
|----------------|----------------|
| `README.md`, `agent-rules.md` | `bootstrap.md` |
| `AGENTS.md` | `agents-index.md` |
| `ARCHITECTURE.md`, `PACKAGES.md`, `DOMAIN.md`, `CODE_STYLE.md` | `core.md` |
| `TOOLS.md`, `TESTING.md`, `FILES.md` | `development.md` |
| `ERRORS.md` | `errors.md` |
| `AUTH.md`, `OAUTH.md` | `auth.md` |
| `DATABASE.md` | `database.md` |
| `API.md` | `api.md` |
| `DEPLOYMENT.md` | `deployment.md` |
| `SECURITY.md` | `security.md` |
| `CLIENTS.md` + per-client docs | `clients.md` |
| `FEATURE_FLAGS.md` | `feature-flags.md` |
| `BACKGROUND_JOBS.md` | `background-jobs.md` |
| `<COMPONENT>.md` (infra) | `infrastructure.md` |

### Documentation generation templates

Templates for generating project documentation are in `./templates/docs/`. Read the manifest (`./templates/docs/README.md`) to discover available templates. When generating docs:
- Apply `rules.docs` from `config.yaml` (if present) as additional rules
- Apply `context` from `config.yaml` as background knowledge
- Each template is self-contained and generates one or more files in `<docs_dir>/`
- **Freshness metadata**: when generating or updating any file in `<docs_dir>/`, MUST add `<!-- generated: YYYY-MM-DD, template: <template-name>.md -->` as the **first line** of the file (before the title). This enables `pipeline.sh docs-check` to track documentation age and detect stale files.

## Rules

1. **MUST check status first.** Run `pipeline.sh status` before doing anything. Never generate phase output without checking status.
2. **Never skip phases.** Follow the order: explore → requirements → design → implementation.
3. **Never auto-approve.** Wait for the user to explicitly say "approve" or equivalent.
4. **Read the template.** Before generating output for a phase, read the corresponding template file.
5. **Save artifacts.** Save generated documents to `.spec-driven-dev/state/` and register them with `pipeline.sh artifact`.
6. **Each phase produces one artifact** that becomes input for the next phase.
7. **Artifacts are cumulative.** Each phase reads all prior artifacts.
8. **Revision limit.** If the user rejects the same artifact 3 times in a row, stop generating and ask: "We've gone through 3 revisions — could you clarify what's missing or what direction you'd prefer?" Do not continue revising without explicit guidance.
9. **Surface uncertainty.** If you are unsure about intent, scope, or technical approach — say so explicitly. State the assumption you would make and ask the user to confirm or correct it. Never silently assume.

## Error Recovery

- **Revising an artifact:** Overwrite the file, re-register with `pipeline.sh artifact <path>`, and present the updated version to the user.
- **Incorrect approval:** Run `pipeline.sh rollback` to return to the previous phase with the artifact restored. Revise and re-approve. Note: rollback restores the artifact *path*, not the file contents. If you overwrote the artifact file at that path, retrieve the previous version from git history.
- **Starting over:** Run `pipeline.sh reset` followed by `pipeline.sh init <feature-name>` to begin a new pipeline.

## Publishing Artifacts

After the pipeline is complete (`phase=done`), run `pipeline.sh publish` to copy all approved artifacts to `.spec-driven-dev/specs/<feature>/`. These files live outside the gitignored `state/` directory and can be committed to version control, creating a persistent record of decisions for future reference.

## Documentation Maintenance

After the pipeline reaches `phase=done` and artifacts are published, check if project documentation needs updating.

### Step 1: Identify affected docs

Read the design document §2.3 ("Files Requiring Changes" table). Match changed file paths against this pattern table:

| Changed file pattern | Affected doc | Owner template |
|----------------------|-------------|----------------|
| `*domain*`, `models/*`, `types/*`, `*entity*` | `DOMAIN.md` | `core.md` |
| new directory under `internal/`, `pkg/` | `PACKAGES.md` | `core.md` |
| `cmd/*`, new service, layer changes | `ARCHITECTURE.md` | `core.md` |
| `*_test*`, `__tests__/`, test config files | `TESTING.md` | `development.md` |
| `Makefile`, `Taskfile`, `scripts/*`, CI tool changes | `TOOLS.md` | `development.md` |
| `*error*`, `*errs*`, error codes, error types | `ERRORS.md` | `errors.md` |
| `*auth*`, `*oauth*`, `*login*`, `*session*` | `AUTH.md` / `OAUTH.md` | `auth.md` |
| `migrations/*`, `schema*`, `*_repo*`, `*_store*` | `DATABASE.md` | `database.md` |
| `*handler*`, `*route*`, `*endpoint*`, `*.proto`, `openapi*` | `API.md` | `api.md` |
| `Dockerfile`, `.github/workflows/*`, `k8s/*`, `docker-compose*` | `DEPLOYMENT.md` | `deployment.md` |
| `*redis*`, `*kafka*`, `*traefik*`, `*prometheus*`, `*nats*` | `<COMPONENT>.md` | `infrastructure.md` |
| `*client*`, `*frontend*`, `*mobile*` | `CLIENTS.md` | `clients.md` |
| `*cors*`, `*csrf*`, `*rate_limit*`, `*security*`, `*helmet*` | `SECURITY.md` | `security.md` |
| `*feature_flag*`, `*toggle*`, `*experiment*` | `FEATURE_FLAGS.md` | `feature-flags.md` |
| `*worker*`, `*job*`, `*queue*`, `*cron*`, `*scheduler*` | `BACKGROUND_JOBS.md` | `background-jobs.md` |
| new code style rule, naming convention change | `CODE_STYLE.md` | `core.md` |

### Step 2: Filter and suggest

1. Collect unique affected docs from the pattern matches.
2. **Filter**: only suggest docs that already exist in `<docs_dir>/`. Do not suggest creating new docs post-pipeline.
3. Present to user: *"This feature touched auth and database files. Update AUTH.md and DATABASE.md? Say 'update docs' or 'skip'."*
4. If user says **"update docs"**: for each affected doc, read its owner template from `./templates/docs/`, regenerate the doc, update freshness metadata.
5. If user says **"skip"**: done, no action.
6. If the docs directory does not exist at all, suggest full generation (same as Pre-flight Checklist step 3).
7. **Never auto-update documentation.** Always ask the user first.

## Quick Start (for the agent)

When the user says something like "I want to add feature X":

1. Follow the **Pre-flight Checklist** (status → config → docs-check → init)
2. Read `./templates/explore.md` — investigate the problem space (use `.spec/` docs as context if available)
3. Generate the exploration document → save to `.spec-driven-dev/state/<feature>-explore.md`
4. Run `pipeline.sh artifact .spec-driven-dev/state/<feature>-explore.md`
5. Present to user → wait for "approve"
6. Run `pipeline.sh approve` → phase advances to requirements
7. Read `./templates/requirements.md` → follow its interview process
8. Generate the requirements document → save, register artifact, present, wait for approve
9. Repeat for design and implementation phases
10. After implementation is approved → `pipeline.sh approve` → pipeline complete
11. Check if documentation needs updating (see Documentation Maintenance)
