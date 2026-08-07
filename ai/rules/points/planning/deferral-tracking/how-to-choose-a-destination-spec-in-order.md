---
kind: table
level:
stage:
---
| Order | Action | Detail |
|-------|--------|--------|
| 1 | Find an existing spec that already covers the topic | `grep -l "<topic>" plan/spec-*.md`, and scan `make ze-spec-status`. Prefer a `spec-finish-<subsystem>` / `spec-followup-<subsystem>` umbrella when one owns the area |
| 2 | If one exists, add the work to its `## Task` section | That spec is the home. Record the deferral with it as Destination, Status `deferred` |
| 3 | Only if no spec covers the topic, create a deferral spec | Named `plan/spec-<source>-deferred-<subtask>.md` (see below). Record the row with it as Destination, Status `deferred`, exactly as in step 2 |
