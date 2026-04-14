# Antipatterns Reference

Antipattern tables for each pipeline phase. Referenced from the corresponding phase template.

---

## Explore

| Antipattern | WRONG ❌ | RIGHT ✓ | Why |
|---|---|---|---|
| Premature requirements | "WHEN token expires, system SHALL refresh" | "One option is to auto-refresh tokens" | WHEN/SHALL belongs in requirements phase |
| Solution attachment | "We should use Redis" | "Option A: Redis, Option B: in-memory — trade-offs:..." | Must show alternatives before committing |
| Ignoring existing code | "I suggest adding a new auth module" | "Existing `src/auth` uses X pattern; we can extend it" | Always read codebase first |
| Scope creep | "We should also add rate limiting and logging" | "Rate limiting could be v2; focus on core auth first" | Help user narrow, not expand |
| Analysis paralysis | 5 options with no recommendation | "Option B is best because...; A is fallback if..." | Recommend clearly when path is evident |

---

## Requirements

| Antipattern | WRONG ❌ | RIGHT ✓ | Why |
|---|---|---|---|
| Architectural solution | "Use a Redis cache for token storage" | "WHEN cache miss occurs, SHALL return fresh data within 100ms" | Prescribes HOW, not WHAT |
| Code or pseudocode | `if token.expired { refresh() }` | "WHEN token is expired, SHALL attempt refresh" | Implementation detail |
| Diagram | Mermaid sequence diagram | Prose description of the flow | Belongs in design phase |
| Vague wording | "The system should handle errors gracefully" | "WHEN refresh fails, SHALL return 401 with error code" | Not verifiable |
| Combined SHALLs | "…SHALL refresh the token and log the event" | Two REQs: one for refresh, one for logging | Must be one SHALL per REQ |
| Unconfirmed requirement | Agent adds REQ-3.1 user never mentioned | Only requirements confirmed by user in interview | Hallucinated scope |
| Technology lock-in | "Use JWT with RS256 signing" | "WHEN issuing tokens, SHALL use cryptographic signing" | Constrains design without user mandate |

---

## Design

| Antipattern | WRONG ❌ | RIGHT ✓ | Why |
|---|---|---|---|
| Function bodies | `func refresh() { cache.Get(key)... }` | `func refresh(token Token) (Token, error)` | Interfaces show signatures only |
| Task lists | "Step 1: create file, Step 2: add tests" | Architecture + interfaces + properties | This is design, not work breakdown |
| Skipping unchanged files | "Files NOT Requiring Changes" table is empty | Explicitly list files in scope that won't change | Omission suggests scope not fully considered |
| Existential properties | "There exists a case where refresh works" | "For all expired tokens, refresh returns valid token" | Properties must use universal quantifier |
| Unlinked properties | "Property 3: tokens are secure" | "Property 3: ... Validates: REQ-1.2" | Every property must trace to a requirement |
| Vague modification scope | `[MODIFIED]` — "various authentication changes" | `[MODIFIED]` — "adds refreshToken(), modifies authenticate() return type" | Must state what exactly changes |
| Scope creep | Designing rate limiting not in requirements | Only design what requirements specify | Stay within approved requirements |
| Silent assumption | Choosing a caching strategy without stating why | `[ASSUMPTION: write-through preferred]` — ask user or mark explicitly | Unstated beliefs cause surprises during implementation |
| Ignoring existing test patterns | Agent invents new test structure (e.g., flat assertions) | Agent follows existing pattern: `auth/token_test.go:TestRefresh` uses table-driven subtests | Tests must be consistent with the project's existing style |

---

## Task Plan

| Antipattern | Why it's harmful |
|---|---|
| Writing code in the plan | The plan is instructions for an agent, not the implementation itself |
| Designing architecture in the plan | Architecture is fixed in the approved design document — do not revise it here |
| Skipping exploration tests for bug fixes | Without a failing test, you cannot prove the bug existed or was fixed |
| Tasks without `*_Requirements:_*` | Untraced tasks cannot be reviewed or rejected with precision |
| Multi-file subtasks | Atomic subtasks are the unit of change — mixing files makes review and fixing harder |
| Forgetting re-test tasks | A fix without a passing re-test is unverified |
| Placing the checkpoint before all tasks complete | The checkpoint is a gate, not a milestone |
| Vague task descriptions | "Update the code" is not a task. "Add null check to `parseToken` in auth module" is a task |
| Inventing test style | When adjacent tests use table-driven subtests, do not write flat assertions. Follow existing project test patterns discovered in Test Infrastructure Discovery |

---

## Implementation

| Antipattern | WRONG ❌ | RIGHT ✓ | Why |
|---|---|---|---|
| Skipping exploration test | Write code first, then test | Write test first (RED), then code (GREEN) | TDD requires RED→GREEN cycle |
| Modifying preservation tests | Change test to match new code | Change code to satisfy both old and new tests | Preservation tests lock correct behavior |
| Bulk implementation | Implement all tasks at once, test at the end | Implement one task, test, mark done, repeat | Atomic execution finds issues early |
| Silent skip | Skip a failing task without noting it | Note the issue, ask user, document decision | Transparency over speed |
| Plan modification | "This task doesn't make sense, I'll do it differently" | Execute as planned; note concerns for review phase | The plan was approved — execute it faithfully |
| Silently leaving broken state | Continue to next task while tests are failing | Stop after 3 failed attempts, document state, ask user to decide | User must stay in control when implementation is stuck |

---

## Review

| Antipattern | Why it's harmful |
|---|---|
| Skipping the implementation report | The implementation report (history[4]) shows which tasks were completed — always verify against it |
| Reviewing the entire codebase | Scope is limited to files changed since `review_base_commit` |
| Approving with `major` findings open | Major findings must be resolved before `PASS` verdict |
| Changing the design in the review | Design is locked in the approved design document — open a new pipeline for design changes |
| Skipping the traceability matrix | Without it, requirements coverage cannot be verified |
| Inventing requirements during review | Review checks against approved requirements only — new requirements need a new pipeline |
| Flagging style preferences as `major` | Style preferences are `nit` at most; reserve `major` for real issues |
| Infinite fix loop | 3 fix cycles maximum — if not resolved, escalate to user instead of looping forever |
