# Anti-Rationalization

**BLOCKING:** These rules preempt known failure modes where the agent rationalizes skipping discipline.

## Why This Exists

Rules are only as good as adherence under pressure. When tests fail, deadlines loom, or code is already written, the temptation is to rationalize shortcuts. These tables pre-address specific excuses so there is no judgment call — the answer is always "no."

## TDD Rationalizations

| Excuse | Reality |
|--------|---------|
| "Too simple to need a test" | Simple code has bugs. Test it. |
| "I'll write tests after" | You won't. And post-hoc tests validate your implementation, not the requirements. |
| "The test would just duplicate the code" | Then the code is too trivial to exist, or your test is wrong. |
| "TDD will slow me down" | Rework from bugs is slower. |
| "Being pragmatic vs dogmatic" | Pragmatic = doing it right the first time. |
| "It's just a refactor, behavior doesn't change" | Then existing tests should pass. If none exist, write them first. |
| "I know this works, I've written it before" | Prove it. Write the test. |

## Test Failure Rationalizations

| Excuse | Reality |
|--------|---------|
| "Transient resource contention" | A test either passes or it doesn't. Investigate. |
| "Parallel execution noise" | Tests must be deterministic under parallel execution. Fix the race. |
| "Not related to our changes" | Prove it. Read the test, trace the failure, show evidence. |
| "Passed on retry" | Retry is not evidence. A failure happened. Why? |
| "Only fails under heavy load" | Production IS heavy load. Fix it. |
| "Timing-dependent" | Then the code has a race condition. Fix it. |

## Completion Rationalizations

| Excuse | Reality |
|--------|---------|
| "Should work" | Run it. Paste output. |
| "Probably fine" | Prove it. Show evidence. |
| "Seems to pass" | "Seems" is not evidence. Paste actual output. |
| "Tests passed earlier" | Run them again now. State changes. |
| "Only cosmetic differences" | Show the diff. Let the user decide. |

## The 3-Fix Escalation Rule

**BLOCKING:** If 3 fix attempts for the same problem have failed, **STOP.**

Do not attempt fix #4. Instead:

1. Report all 3 failed approaches to the user
2. Explain what each attempt assumed and why it failed
3. Question whether the approach or architecture is wrong
4. Ask the user how to proceed

**Why:** Three failures suggest a wrong mental model. More attempts in the same direction waste time. Step back, reassess, discuss.

## Review Response Rules

When receiving code review feedback or corrections from the user:

| Banned | Use Instead |
|--------|-------------|
| "You're absolutely right!" | "Fixed. [Brief description]" |
| "Great point!" | "Good catch — [specific issue]. Fixed in [location]." |
| "Thanks for the feedback!" | (just fix it) |
| "That makes total sense!" | (fix it, explain what you changed) |

**Why:** Performative agreement wastes tokens and signals sycophancy, not understanding. Fix the problem, describe what you changed, move on.

## When Reviewing Own Work

Default posture: **skeptical.** Assume your implementation report is optimistic.

Before claiming anything is complete:
- Assume you missed something
- Re-read the spec requirements one more time
- Run the verification commands fresh (not from memory)
- Check that every acceptance criterion has evidence, not just a checkbox
