# Documentation Workflows

This file contains all documentation generation, staleness checking, and post-pipeline maintenance workflows. Referenced from `SKILL.md`.

---

## Documentation Context

The skill supports a self-documenting mechanic via a project documentation directory (default: `.spec/`, configurable via `docs_dir` in `config.yaml`).

> **Directory separation:** Project documentation files (README.md, ARCHITECTURE.md, DOMAIN.md, etc.) live in `<docs_dir>/` (default: `.spec/`). Pipeline phase artifacts (explore.md, requirements.md, design.md, etc.) live in `.spec/features/<feature>/`. **Never place project documentation into the feature directory or vice versa.**

### Pre-pipeline check

When running `pipeline.sh init <feature-name>`, before starting the Explore phase:

1. Determine the docs directory: read `docs_dir` from `.spec/config.yaml`. If not set, default to `.spec`.
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
- **Freshness metadata validation**: after saving a generated doc, verify that line 1 matches the pattern `<!-- generated: YYYY-MM-DD, template: <name>.md -->`. If the metadata is missing or malformed, fix it immediately — `docs-check` will silently skip files without valid metadata.
- **Content-aware staleness**: `pipeline.sh docs-check` uses scope metadata from templates (`<!-- scope: ... -->` first line) combined with `git log --since=<generated_date>` to determine staleness. A doc is marked stale only if (a) files matching its template's scope patterns were changed since generation **and** (b) the doc exceeds the freshness threshold. Docs whose scope shows no changes remain fresh regardless of age. If a template has no scope line, the check falls back to pure age-based staleness. The JSON output includes a `scope_changed` field (`true`/`false`/`null`) per file.

---

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
