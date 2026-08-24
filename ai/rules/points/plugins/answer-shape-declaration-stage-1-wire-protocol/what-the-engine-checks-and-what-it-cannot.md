---
kind: note
level: MUST
stage:
---
**A declared column name MUST be a key the plugin's handler writes, in the same
spelling.** The engine never sees the payload. So `validateShapeDecls` checks
the shape spelling, the presence of a command path and the two bounds, and it
cannot check that a name names anything. A functional test over the rendered
answer is what checks that half.

One bad entry refuses the whole list and fails stage 1. The message names the
command and the offending value, clamped to 64 bytes so a plugin cannot write an
unbounded string into the daemon log.
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls, validateDeclaredFieldName, clampDeclared -->
