---
kind: directive
level: MUST
stage:
---
**You MUST NOT let a red persist.** Scope-to-changed is a temporary bridge while the
global suite is being cleared, not a standing mode. A `ze-verify` that stays red
across sessions hides newly-introduced breakage under the existing red -- that is
exactly how an import cycle, a YANG typedef gap, and stale registry snapshots all
landed under one persistent red without any gate firing. You MUST log the failing stage in
`plan/known-failures/` with who owns clearing it; if nobody does, clearing it
MUST come before stacking more changes on top.
