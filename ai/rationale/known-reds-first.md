# Known Reds First Rationale

Why: `ai/rules/pre-release.md`, "Fix known reds by targeted reruns, and verify the whole tree once"

## Owner Decision

Owner decision, 2026-10-17, verbatim:

> as we are fixing more issues, we should stop the global test and focus only
> on what we know fails until it all pass ... then rerun the full test ... make
> sure it is our approach to testing always

## Why

A full verify run while known reds are still open, or while other agents load
the machine, spends hours re-measuring what is already known. Every red it
reports is either one already on the list, or noise from the load beside it.
The targeted rerun answers the only open question, whether this fix made this
test pass, in seconds.

The habit it replaces: after fixing one failure, an agent reran lint, every
unit test and the functional gate "to make sure nothing else broke". That run
re-reports every other known red, which the agent then reads again, and the
fix it was checking waits behind the whole tree.
