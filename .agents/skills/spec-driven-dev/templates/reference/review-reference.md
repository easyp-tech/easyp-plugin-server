# Review Phase — Extended Reference

Severity definitions and Fix Plan structure for the Review phase. Referenced from `review.md`.

---

## Severity Definitions

| Severity | Meaning |
|----------|---------|
| `critical` | Must fix before merge — security hole, data loss risk, core requirement unmet |
| `major` | Must fix before merge — missing test, design deviation, significant quality issue |
| `minor` | Should fix — naming, minor style issue, missing edge case test |
| `nit` | Optional — cosmetic, preference-based suggestion |

---

## Fix Plan Structure

When verdict is `NEEDS_CHANGES` or `BLOCK`, the review document MUST include a Fix Plan section. The fix plan uses the same Commands and Test Style Source from the approved task plan (`history[3].artifact`) — do NOT discover them again.

### Fix tasks for `critical` / `major` findings

For each finding with severity `critical` or `major`, create a **TDD Fix Task**:

```markdown
### Fix F-N: <finding description>

*_Finding: F-N_*
*_Requirements: REQ-X.Y_* (if the finding is linked to a requirement)

**Exploration Test (RED)**
Write a test demonstrating the finding. MUST FAIL on the current code.
Follow the Test Style Source from the task plan.
- [ ] 1. Write test in `<file>` — `<test command>`
- [ ] 2. Run test — confirm FAILS

**Fix**
CRITICAL: one subtask = one file.
- [ ] 3. <Action in file> — `<test command>`

**Re-test (GREEN)**
- [ ] 4. Run exploration test — confirm PASSES
- [ ] 5. Run full test suite — confirm no regressions
```

IMPORTANT: If a finding is not testable (naming, dead code, unused imports, debug artifacts), mark it `NOTE: no test applicable` and provide a flat fix subtask instead:

```markdown
### Fix F-N: <finding description>

*_Finding: F-N_*
NOTE: no test applicable

- [ ] 1. <Action in file> — `<lint/build command>`
```

### Fix tasks for `minor` / `nit` findings

Flat list — no exploration test required:

```markdown
### Fix F-N: <finding description>

*_Finding: F-N_*

- [ ] 1. <Action in file> — `<test/lint command>`
```
