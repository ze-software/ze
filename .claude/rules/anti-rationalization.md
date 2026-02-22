# Anti-Rationalization

**BLOCKING:** The answer is always "no."
Rationale: `.claude/rationale/anti-rationalization.md`

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
| "Not related to our changes" | Prove it with evidence |
| "Passed on retry" | Retry is not evidence |
| "Timing-dependent" | Race condition. Fix it |

## Completion

| Excuse | Answer |
|--------|--------|
| "Should work" / "Probably fine" | Run it, paste output |
| "Tests passed earlier" | Run again now |
| "Only cosmetic differences" | Show diff, let user decide |

## 3-Fix Rule

**BLOCKING:** 3 failed fixes → STOP. Report all 3 approaches. Question the mental model. Ask user.

## Posture

No performative agreement. Fix it, describe what changed, move on.
Assume your implementation report is optimistic. Re-read spec, re-run verification fresh.
