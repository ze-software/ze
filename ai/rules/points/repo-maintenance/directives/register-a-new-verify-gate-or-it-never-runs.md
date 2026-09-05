---
kind: directive
level: MUST
stage:
---
**A new verification gate MUST be registered in `StagesForMode` (`internal/le/verify/engine/stages.go`), and a gate that runs in some modes only MUST be added to each one it belongs in: `fullStages`, `staticcheckStages` and `changedStages` are separate lists.** A gate absent from every list is code that compiles, passes its own unit test, and judges nothing, which is the fail-open shape `ai/rules/evidence.md` bans. Read the three functions and say which lists the gate joins, rather than adding it to the first one you find.
