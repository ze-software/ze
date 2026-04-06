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
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | Fix it. If it needs more than 10 minutes, log to `.claude/known-failures.md` and continue |

**Every test failure gets fixed, not reported.** Investigate, fix, move on. Pre-existing failures
are fixed as a separate commit (not mixed with feature work). Do not block current work on a
failure you didn't cause -- but DO fix it in the same session, after completing the primary task.
If a pre-existing failure requires deep investigation beyond the session's scope, add it to
`.claude/known-failures.md` with the failure output and root-cause hypothesis so the next session
picks it up. Never skip a failure without either fixing it or logging it.

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

## 3-Fix Rule

**BLOCKING:** 3 failed fixes → STOP. Report all 3 approaches. Question the mental model. Ask user.

## Posture

No performative agreement. Fix it, describe what changed, move on.
Assume your implementation report is optimistic. Re-read spec, re-run verification fresh.
