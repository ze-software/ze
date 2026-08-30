---
kind: directive
level: MUST
stage:
---
- **A declared column name MUST be a key the plugin's handler writes, in the same spelling, and a functional test over the rendered answer MUST be what checks it.** The engine never sees the payload: `validateShapeDecls` checks the shape spelling, the presence of a command path and the two bounds, and it cannot check that a name names anything.
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls -->
