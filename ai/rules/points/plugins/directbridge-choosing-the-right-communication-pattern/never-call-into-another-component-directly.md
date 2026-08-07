---
kind: directive
level:
stage:
---
**Anti-pattern:** A plugin calling a plain exported function in another
`internal/component/*` package directly, instead of through DirectBridge/
DispatchCommand, to register a callback or reach shared engine state. This
compiles and works when the plugin happens to run internal, then silently
no-ops when it runs external (the call mutates the subprocess's own
disconnected copy of that package's state). See "Process Boundary" below,
gated by `make ze-plugin-boundary-check`.
