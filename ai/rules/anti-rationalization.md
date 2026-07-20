# Anti-Rationalization

**When:** The answer is always "no."
**Severity:** blocking

## Directives

The answer is always "no."
Rationale: `ai/rationale/anti-rationalization.md`

## TDD

| Excuse | Answer |
|--------|--------|
| "Too simple to need a test" | Test it |
| "I'll write tests after" | Post-hoc tests validate implementation, not requirements |
| "TDD will slow me down" | Rework from bugs is slower |
| "Just a refactor" | Existing tests should pass. None exist? Write them first |

## Test Failures

| Excuse | Answer |
|--------|--------|
| "Transient" / "resource contention" | Investigate. A failure happened |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | Fix it or log it to `plan/known-failures.md`. A passing comment is not logging |

**Every test failure gets fixed or formally logged. BLOCKING.**

1. **Fix it** as a separate commit (not mixed with feature work). Do not block current work on a
   failure you didn't cause, but DO fix it in the same session after completing the primary task.
2. **If fixing requires deep investigation beyond session scope**, add a structured entry to
   `plan/known-failures.md` with: failure output, root-cause hypothesis, and reproduction command.
   This is not optional. A casual mention in your response is not logging.
3. **Mechanical check before session end:** if your session encountered any failure you did not fix,
   grep `plan/known-failures.md` for a matching entry dated today. No entry = violation.

| Banned | Why |
|--------|-----|
| "Pre-existing, not my changes" | Acknowledging a failure without fixing or logging it means the next session hits the same wall |
| "Known issue with the netlink API" | Known to whom? If it's not in `known-failures.md`, it's not known to the project |
| Mentioning a failure only in response text | Response text is ephemeral. `known-failures.md` persists across sessions |
| "The only failures are..." (then moving on) | Enumeration without action is rationalization |

## Completion

| Excuse | Answer |
|--------|--------|
| "Should work" / "Probably fine" | Run it, paste output |
| "Tests passed earlier" | Run again now |
| "Only cosmetic differences" | Show diff, let user decide |
| "Library and interface only" | Feature is not done — library without wiring is dead code |
| "Wiring will be done in next commit" | One commit = code + tests + wiring + summary. No partial deliveries |
| "The .ci test requires infrastructure" | Then the feature is blocked, not done |
| "Unit tests prove it works" | Unit tests prove the algorithm. .ci tests prove the user can reach it |
| "SetAuthorizer is called somewhere" | Show the .ci test where a user command is denied. No test = no proof |
| "Consistent with how other plugins do it" | Other plugins missing tests is a gap, not a precedent |
| "No test infrastructure for this path" | Build the infra or flag as BLOCKER. Never downgrade to NOTE |
| "Out of scope for this review" | Missing coverage is never out of scope. Report as ISSUE |

## 3-Fix Rule

3 failed fixes → STOP. Report all 3 approaches. Question the mental model. Ask user.

## Posture

No performative agreement. Fix it, describe what changed, move on.
Assume your implementation report is optimistic. Re-read spec, re-run verification fresh.
