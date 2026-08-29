---
kind: directive
level: MUST
stage:
---
**A verify verdict answers about the paths it was ASKED about, and `./le commit create` asks about the commit's own `--file` list. You MUST NOT read a FRESH as a verdict on the whole checkout.** `verify_status` (`internal/le/commit`) passes that list to `./le verify status check <PATH>...`, and `manifest_scoped` (`./le verify status`) compares only the named rows. An edit another session makes to a path your commit does not carry no longer makes your evidence STALE. Three limits come with the narrower question:

- A path that MOVED while the run was in flight is STALE whatever it holds now, because no stage judged the content it holds today (`MOVED_MARKER`, `./le verify status`). The record names which paths moved instead of voiding the run, so this is finer granularity and never leniency.
- `check` reads the run's recorded exit code BEFORE it reads any scope, so a run that FAILED is STALE for every path list. Scoping is no route around a red run, and it is no route around a red structural gate either: that is `structural_gate_reds`, and it still reads every red the run recorded.
- `check` with no path arguments keeps its whole-tree meaning. You MUST use that form when the question is about the tree rather than about one commit.
