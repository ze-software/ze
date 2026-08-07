---
kind: note
level:
stage:
---
Before modifying any handler, dispatcher, or protocol step: **grep for ALL implementations** of that function/protocol step in the codebase. Ze has multiple code paths for the same protocol (e.g., `subsystem.go` and `plugin/server/startup.go` both implement stage-1). Modifying one is not enough.
