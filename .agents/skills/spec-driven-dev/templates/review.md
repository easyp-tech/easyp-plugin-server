# Phase 6: Code Review

## Role

You are a **Code Reviewer**. Your task: accept all five approved artifacts (exploration, requirements, design, task plan, implementation report) and review the **written code** against them — verifying requirements traceability, design conformance, code quality, and security.

If the review finds `critical` or `major` issues, you **fix them yourself** using a TDD fix plan (exploration test → fix → re-test), then re-review. You repeat this cycle until the verdict is `PASS`, up to a maximum of **3 fix cycles**. If the verdict is still not `PASS` after 3 cycles, you stop and escalate to the user.

You do **NOT** change architecture or add new features. You fix findings against the existing requirements and design.

---

Read `./templates/_preamble.md` for Pipeline Integration and Project Context instructions.
- **Phase rule key:** `rules.review`
- **Input artifacts:** read `history[0..4].artifact` (exploration, requirements, design, task plan, implementation report). Also read `review_base_commit` — git commit hash recorded when task plan was approved (the baseline for `git diff`).
- **Output:** `.spec/features/<feature-name>/review.md`

---

### Fast-track mode

When reviewing a small bug fix (1–2 REQs, 1–3 changed files):

- **Change Set Discovery:** 1–3 rows (mostly ✅ Planned).
- **Requirements Traceability:** 1–2 rows (1–2 REQs).
- **Design Conformance:** brief per subsection. "No changes" for subsections with no impact.
- **Code Quality & Security:** brief. If the fix is internal logic only: "No new endpoints, no user input changes."
- **Findings:** 0–2 expected. Any `critical` findings still require a fix cycle.
- **Fix cycles:** maximum **1 fix cycle** before escalating to user (vs. 3 for full pipeline). A small fix that still fails after 1 cycle likely has a design issue.
- **Verification Evidence:** still required — actual stdout from test, build, lint.

Target artifact size: **≤ 1 page** (excluding stdout).

---

## Language

Write the review document in the **user's language** (detected from their first message). This includes:
- Section headers (translate "Change Set Discovery", "Requirements Traceability Audit", etc.)
- Finding descriptions and recommendations
- Verdict explanation

Keep in English (do not translate):
- Instruction keywords: `CRITICAL`, `IMPORTANT`, `NOTE`, `DO NOT`, `GOAL`
- Requirement IDs: `REQ-X.Y`, Task IDs: `T-N`, Finding IDs: `F-N`
- Correctness Property IDs: `CP-N`
- Code identifiers, file paths, shell commands
- Verdict labels: `PASS`, `NEEDS_CHANGES`, `BLOCK`
- Severity labels: `critical`, `major`, `minor`, `nit`

---

## Step 1: Change Set Discovery

Determine which files were changed during implementation.

### Primary source: git diff

If `review_base_commit` is present in `pipeline.json`:

```sh
git diff --name-only <review_base_commit>..HEAD
git diff --stat <review_base_commit>..HEAD
```

IMPORTANT: If `review_base_commit` is empty or git is unavailable, fall back to the secondary source.

### Secondary source: task plan

Extract the list of files from the task plan (CODE subtasks) and the design document §2.3 ("Files Requiring Changes").

### Cross-reference

1. **Planned vs Actual** — compare the file list from the task plan with the actual changed files.
2. **Unexpected files** — files changed but NOT in the plan. For each, determine: is it a justified dependency, scope creep, or accidental change?
3. **Not changed** — files in the plan that were NOT changed. For each, determine: was the task skipped, was it unnecessary, or is it an omission?

Output a table:

| File | Status | Notes |
|------|--------|-------|
| `path/to/file.go` | ✅ Planned | — |
| `path/to/other.go` | ⚠️ Unexpected | Explain why |
| `path/to/skipped.go` | ❌ Not Changed | Explain impact |

Also cross-reference with the implementation report (history[4]) to verify that all tasks marked `[x]` actually produced the expected file changes.

---

## Step 2: Requirements Traceability Audit

For each requirement in the requirements document, verify implementation:

1. **Read each `REQ-X.Y`** from the approved requirements document.
2. **Locate the test(s)** — find the test(s) that exercise this requirement (cross-reference with task plan task annotations `*_Requirements: X.Y_*`).
3. **Locate the code** — find the production code that implements this requirement.
4. **Verify correctness property** — check that the corresponding correctness property from the design document §2.6 holds in the implementation.

Output a traceability matrix:

| Requirement | Test(s) | Code | CP | Verdict |
|-------------|---------|------|----|---------|
| REQ-1.1 | `test_login_success` | `auth/handler.go:45` | CP-1 | ✅ |
| REQ-1.2 | (none found) | `auth/handler.go:78` | CP-2 | ❌ Missing test |
| REQ-2.1 | `test_token_refresh` | `auth/token.go:12` | CP-3 | ⚠️ Partial |

CRITICAL: Every requirement MUST appear in the matrix. A requirement without a corresponding test is a finding.

---

## Step 3: Design Conformance

Verify the implementation matches the approved design document:

### 3.1 Architectural Boundaries

- Do new components reside in the correct layers/packages as specified in the design?
- Are dependencies between components flowing in the correct direction?
- Are there any unauthorized cross-layer imports?

### 3.2 Data Models

- Do struct/class definitions match the data models in the design document §2.2?
- Are field names, types, and constraints consistent?
- Are database migrations (if any) consistent with the schema in the design?

### 3.3 API Contracts

- Do endpoint signatures match the design document §2.5?
- Are request/response formats consistent?
- Are error codes and error formats as specified?

### 3.4 Error Handling

- Does the implementation follow the error handling strategy from the design document?
- Are errors wrapped/propagated as specified?
- Are all error paths from the design covered?

### 3.5 Correctness Properties

For each correctness property in the design document §2.6:
- **Equivalence** — are bidirectional transformations inverse?
- **Absence** — are negative conditions properly rejected?
- **Round-trip** — does serialize→deserialize (or similar) preserve data?
- **Propagation** — do changes propagate through the expected chain?
- **Exclusion** — are mutually exclusive states enforced?

NOTE: Not all categories apply to every feature. Only review properties listed in the design.

### 3.6 Documentation Consistency

- Do Mermaid diagrams in the design document (§2.2) match the actual code structure?
- Are component/package names in diagrams consistent with actual names in the codebase?
- If the design document describes data flows, do they match actual function calls / message passing?
- Are any new components introduced during implementation missing from diagrams?

---

## Step 4: Code Quality

Review the changed files for quality issues. Focus on the diff, not the entire codebase.

### 4.1 Naming & Clarity

- Do new identifiers follow project naming conventions?
- Are names descriptive and consistent with the existing codebase?

### 4.2 Dead Code & Debug Artifacts

- Are there commented-out code blocks, `TODO`s without tickets, or debug `print`/`log` statements?
- Are there unused imports, variables, or functions?

### 4.3 Scope Creep

- Does the implementation contain changes beyond what was specified in the requirements and design?
- Are there refactors, feature additions, or "improvements" not in the plan?

### 4.4 Test Quality

- Do tests actually assert the correct behavior (not just "no error")?
- Are test names descriptive?
- Do tests follow the patterns from Test Infrastructure Discovery (task plan)?
- Are edge cases from the requirements document covered?

---

## Step 5: Security Scan

IMPORTANT: Scope this scan to **changed files + the full request handling chain for any new public API endpoints** exposed by the changed code. This is NOT a full security audit — but new endpoints must be traced from routing through middleware, authentication, authorization, handler, and response.

Review changes against common vulnerability categories:

| Category | What to check |
|----------|---------------|
| Input validation | Are all external inputs validated/sanitized? |
| Authentication | Are auth checks present on new endpoints? |
| Authorization | Are permission checks correct for the user's role? |
| Injection | SQL injection, command injection, XSS in templates? |
| Secrets | Are secrets, tokens, or credentials hardcoded? |
| Data exposure | Are sensitive fields excluded from API responses/logs? |
| Error leakage | Do error messages expose internal details? |
| API chain audit | For new endpoints: verify the full routing → middleware → auth → handler → response chain is secure |

NOTE: For existing code, only flag issues present in changed files. For **new endpoints**, audit the full request chain even if some files in the chain were not modified (e.g., verify middleware applies to the new route).

---

## Review Document Structure

The final artifact must contain these sections:

```markdown
# Code Review: <feature-name>

## Verdict: <PASS | NEEDS_CHANGES | BLOCK>

<One-paragraph summary explaining the verdict.>

## Change Set

<Table from Phase 1>

## Requirements Traceability

<Matrix from Phase 2>

## Design Conformance

<Findings from Phase 3, organized by subsection>

## Code Quality

<Findings from Phase 4>

## Security

<Findings from Phase 5, or "No security issues found in changed files.">

## Verification Evidence

Actual (truncated) output of commands run during review. Do NOT replace with status assertions.

- **Tests:**
\`\`\`
<paste last 20 lines of test command output>
\`\`\`
- **Build:**
\`\`\`
<paste last 10 lines of build command output>
\`\`\`
- **Lint:**
\`\`\`
<paste last 10 lines of lint command output>
\`\`\`

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | critical | `path/file.go:42` | Description | REQ-1.1 |
| F-2 | major | `path/other.go:15` | Description | REQ-2.1 |
| F-3 | minor | `path/util.go:88` | Description | — |

## Recommendations

<Ordered list of recommended changes, grouped by severity.>

## Fix Plan

<!-- Only include this section when verdict ≠ PASS -->
<TDD Fix Tasks for critical/major findings, flat fix tasks for minor/nit — see Fix Plan Structure above.>
```

---

## Verdict Rules

| Verdict | Condition |
|---------|-----------|
| `PASS` | Zero `critical` or `major` findings. All requirements traced to tests and code. |
| `NEEDS_CHANGES` | One or more `major` findings, OR requirements with missing tests/code. No `critical` findings. |
| `BLOCK` | One or more `critical` findings (security vulnerability, missing core requirement, architectural violation). |

### Severity Definitions

Severity definitions and Fix Plan Structure: read `./templates/reference/review-reference.md`.

---

## Iteration Workflow (Self-Healing Loop)

GOAL: the agent autonomously fixes all findings and re-reviews until the code is clean.

### Initial review

1. Execute Phases 1–5 (Change Set Discovery through Security Scan).
2. Generate the review document with verdict, findings table, recommendations, and fix plan (if verdict ≠ `PASS`).
3. If verdict is `PASS` → skip to step 8.

### Fix cycle (verdict ≠ `PASS`)

4. **Execute the fix plan:**
   - For each TDD Fix Task: write exploration test (confirm RED) → apply fix → re-test (confirm GREEN).
   - For flat fix tasks: apply fix → run lint/build.
   - Use Commands from the task plan. DO NOT invent new commands.
5. **Re-review:** re-execute Phases 1–5 completely.
   - Change Set Discovery: use the same `review_base_commit` baseline (the diff grows as fixes accumulate).
   - Check each previous finding: resolved or still present.
   - Check for new findings introduced by fixes.
   - Re-run test/build/lint and capture fresh stdout for the Verification Evidence section.
6. **Generate a new revision** of the review document. Register: `sh ./scripts/pipeline.sh artifact <path>`
7. **Check iteration count.** If this is the **3rd fix cycle** and verdict is still not `PASS` → stop the loop and go to step 8b.
8. **Repeat** from step 4 until verdict is `PASS` or iteration limit is reached.

### Completion (verdict `PASS`)

9. Present the final review document (verdict `PASS`) to the user.
10. Wait for the user to say "approve".
11. Run: `sh ./scripts/pipeline.sh approve`

### Escalation (3 fix cycles exhausted without `PASS`)

8b. **Stop the self-healing loop.** Present the current review document to the user with a summary:
   - How many fix cycles were completed (3)
   - Which findings were resolved across iterations
   - Which findings remain open (with severity and description)
   - Ask the user: *"3 fix cycles completed but N findings remain. Options: (a) I'll look at the remaining issues and guide you, (b) continue with 1 more cycle, (c) approve as-is with known issues."*
   - Wait for user guidance before proceeding.

CRITICAL: The agent does NOT ask the user to fix code. The agent fixes code itself.
CRITICAL: Each iteration is saved as a revision. Use `pipeline.sh revisions review` to see past iterations.
CRITICAL: Maximum **3 fix cycles** (initial review + 3 fix→re-review iterations = up to 4 review documents total). After 3 cycles, escalate to user.
DO NOT skip the re-review after fixes. Every fix cycle MUST end with a full re-review.

---

## Quality Control Checklist

Before delivering the review, verify:

- [ ] Change Set Discovery is complete — all changed files are listed with planned/unexpected/missing status.
- [ ] Requirements Traceability matrix is complete — every `REQ-X.Y` from the requirements document has an entry.
- [ ] Every requirement has at least one test linked. Missing tests are flagged as findings.
- [ ] Design conformance is checked: architectural boundaries, data models, API contracts, error handling, correctness properties.
- [ ] Code quality is reviewed: naming, dead code, scope creep, test quality.
- [ ] Security scan covers changed files for input validation, auth, injection, secrets, data exposure, error leakage. New endpoint chains are audited end-to-end.
- [ ] All findings have an ID (`F-N`), severity, file reference, and description.
- [ ] Verdict is correct per the verdict rules.
- [ ] If verdict ≠ `PASS`: Fix Plan is present with TDD exploration tests for each testable `critical`/`major` finding.
- [ ] If verdict ≠ `PASS`: all fix tasks reference their finding ID (`F-N`).
- [ ] If verdict ≠ `PASS`: fix tasks use Commands and Test Style Source from the task plan (no new discovery).
- [ ] Each fix iteration is saved as a revision via `pipeline.sh artifact <path>`.
- [ ] Final verdict is `PASS` before presenting to user for approval.
- [ ] Verification Evidence section contains actual command output (stdout), not assertions.
- [ ] Artifact is registered via `pipeline.sh artifact <path>`.

---

## Done when

Do NOT suggest approval until **every** condition is true:

1. Change set is fully documented (planned, unexpected, missing files).
2. Requirements traceability matrix covers all `REQ-X.Y` entries.
3. Design conformance review is complete for all applicable subsections (§3.1–§3.6).
4. Code quality review is complete.
5. Security scan of changed files is complete.
6. Findings table lists all issues with ID, severity, file, and description.
7. Verdict is `PASS` — zero `critical` or `major` findings remain.
8. Verification Evidence section contains real command output (stdout) for test, build, and lint.
9. Artifact is registered via `pipeline.sh artifact <path>`.

---

## Antipatterns — Never Do These

Antipatterns for this phase: read `./templates/reference/antipatterns.md` § Review.
