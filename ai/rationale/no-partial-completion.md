# Rationale: No Partial Completion

Sessions are independent. When one session claims "done," the next session
sees it as done and builds on top. If the first session shipped partial work,
the second session inherits broken foundations it doesn't know are broken.

The cost of discovering partial work downstream is 10x the cost of finishing
it now. A missing wiring test means the feature exists in code but no user
can reach it. A missing functional test means the feature works in isolation
but nobody proved the daemon exposes it. Both look "done" in git log.

"Deferred" is particularly dangerous. It looks like a plan while nothing
schedules the work. The learned summaries have multiple entries
(lg-overhaul, cmd-4) where "done with deferrals" meant "never finished."
That is why an item a spec does not do becomes its own spec, in the bucket
that item belongs to. A spec is counted, and a row is not
(`ai/rationale/unfinished-scope-becomes-a-spec.md`). Scope reduction
requires explicit user approval because only the user knows which
acceptance criteria are load-bearing vs nice-to-have.

The rule exists because every AI session's instinct is to wrap up and
present when tests pass. Tests passing is step 10 of 12. The remaining
steps (docs, audit, journal row, spec closure) are where partial work
is caught, and skipping them is how partial work ships.
