# 1325 -- The review loop had no terminating state

## Context

Agents were unable to CLOSE finished work. `ai/rules/critical-review.md` said
"loop until a pass finds nothing". `ai/skills/ze-review.md` said every pass must
re-examine the full diff. Together those never stop. A full-diff pass on any
real change always finds something, so each round produced fixes and the fixes
earned another round. The agent then reached for the only two exits it can see:
reduce scope, or declare done anyway. Thomas asked for a fix that does not rest
on the agent's judgement in the moment, because that judgement is what fails at
the end of a long task.

## Decisions

- Bounded the SCOPE, not the number of passes. Round 1 covers the whole diff, round N+1 covers only round N's fixes and what they touched. The bound is structural: round N+1's scope is fixed by round N's fixes before either round runs.
- Required the round's scope to be written down BEFORE the round runs. Unwritten, "what those fixes touched" is chosen after the findings are known and shrinks to whatever produces a clean round.
- Tested DEPENDENCY, never causation, for an out-of-scope finding. `ai/rules/no-parking.md` already asks whether the goal still holds if you leave it. A defect this change did not introduce is in scope the moment the work depends on that path.
- Listed eight classes that are always in scope whatever round finds them, each with its own authority. Every one passes a "no wrong result, no red gate" screen because its failure mode is silence.
- Made skills POINT at the rule instead of restating its tests, after a restatement in `ai/rules/planning.md` and another in `ai/skills/ze-review.md` each carried a superseded version.

## Consequences

- The loop now has a terminating state, so a gate that agents used to bypass can be satisfied honestly.
- Effort is declared before it is spent. Pass count and lenses are named before the first agent is spawned. Two lenses is the floor on round 1, and three for an "audit this" ask.
- `ai/rules/critical-review.md` owns the disposition tests. Four skills reference them and none restates them.

## Gotchas

- **A clause that licenses closure is where every under-qualification lands.** The first draft claimed its three tests "ARE" no-parking's question. They were causation, not dependency, so a pre-existing defect on a depended-on path would have been homed. That sentence was a parking hatch inside the rule written to prevent one.
- **The first prescribed home was deleted by the closure it authorised.** Findings were sent to the spec's deferral shard, which `ai/rules/deferral-tracking.md` `git rm`s at closure.
- **Severity was an unguarded exit.** Until an always-in-scope finding was barred from being a NOTE, tagging it down retired it, and severity is the reviewer's own call.
- **Fixing the rule is a third of the job.** Six rounds, and four of them found the corrected text sitting one hop from a stale copy in `planning.md`, `ze-review.md`, `ze-implement.md`, `ze-review-spec.md` or `ze-review-deep.md`. The skill that drives the loop matters more than the rule that describes it.

## Files

- `ai/rules/critical-review.md` -- "Bounding the loop", "State the review effort before you spend it"
- `ai/rules/planning.md` -- Review Gate points at the bound
- `ai/skills/{ze-review,ze-review-deep,ze-implement,ze-review-spec}.md`
