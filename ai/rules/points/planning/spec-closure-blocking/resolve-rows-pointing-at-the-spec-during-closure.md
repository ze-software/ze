---
kind: directive
level:
stage:
---
**Closure resolves the spec's deferral rows.** Before commit B, grep
`plan/deferrals/` for this spec's filename (a row naming it as Destination may live in
any source's shard, not only this spec's own). Every row naming it as **Destination**
must be resolved inside commit A: set Status `done` and Destination to the learned
summary (`plan/learned/NNN-<name>.md`), which is where the knowledge now lives. This
is separate from the shard removal in commit B, above: this resolves rows that POINT
AT the spec, which closure must do because it is deleting their destination. It does
NOT retire the rows the spec SOURCED. Those are governed by commit B's condition:
a sourced row homed at another spec stays live, and its shard outlives this closure
("Deferral Tracking", below). Only an all-terminal shard is removed.
