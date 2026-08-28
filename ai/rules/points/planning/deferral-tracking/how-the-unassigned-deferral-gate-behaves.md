---
kind: note
level:
stage:
---
The commit gate `deferral_unassigned_problems` (`internal/le/commit`)
folds over every shard in `plan/deferrals/` and WARNS, it surfaces, it does not
block, on any LIVE deferral (any non-terminal Status, see Status Vocabulary) that
names no destination or names a spec file that does not exist, and on any row it
cannot parse. It is routed through
`commit_gate_warnings`, not `commit_gate_problems`: the message prints to stderr
and the commit proceeds. This is advisory by design, for the reason in the banner
above (an unhomed row is harmless to software; blocking unrelated and other-session
commits on it was too aggressive). Homing stays mandatory as an obligation on the
author; the warning is what keeps an unhomed or unparseable row visible so it is
not silently lost.
