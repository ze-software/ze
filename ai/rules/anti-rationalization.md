# Anti-Rationalization

**When:** when you catch yourself explaining why a test, a gate, or a completion standard does not apply this time
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
| "Only fails under load" / "passes in isolation" | That is the diagnosis, not an excuse: the test asserts on elapsed time. Make it wait on the condition (`ai/rules/fix-dont-record.md`) |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | Fix it. "Pre-existing" says when it started, not whose it is. You are the entry point that reached it |

**Every test failure gets FIXED. BLOCKING.** Logging is not an alternative outcome
(owner directive 2026-07-23; full rule: `ai/rules/no-parking.md` "Recording is not
fixing"). A `plan/known-failures/` shard is the running record of an investigation you
are still driving, never a place to leave a defect.

1. **Fix it** as a separate commit (not mixed with feature work). Do not block current work on a
   failure you didn't cause, but DO fix it in the same session after completing the primary task.
2. **A shard is allowed for ONE case only: a failure whose MECHANISM you could not
   determine.** Deterministic reds, structural gates, anything with a reproduction
   command, and anything host load explains are fixed, never sharded. When the exception
   does apply, add
   `plan/known-failures/<make-target>-<test-name>.md` with: failure output, the
   reproduction attempt and its result, evidence gathered, and the next step. Label a
   root cause you have not verified against source a HYPOTHESIS, so the next agent does
   not inherit it as fact.
3. **Mechanical check before session end:** every failure your session encountered is
   fixed, or is a non-reproducible one whose shard names the next step. An unfixed
   deterministic failure is a violation regardless of what was written down.

| Banned | Why |
|--------|-----|
| "Pre-existing, not my changes" | Acknowledging a failure without fixing it means the next session hits the same wall |
| "Known issue with the netlink API" | Known to whom? And "known" is not "fixed" |
| Mentioning a failure only in response text | Response text is ephemeral, and describing a bug does not fix it |
| "The only failures are..." (then moving on) | Enumeration without action is rationalization |
| "Tracked in `plan/known-failures/`" offered as the outcome | Tracking is not fixing. The product is still broken. See `ai/rules/no-parking.md` |
| Adding a shard for a failure that reproduces on demand | A reproduction command IS the start of the fix, not a substitute for it |

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
