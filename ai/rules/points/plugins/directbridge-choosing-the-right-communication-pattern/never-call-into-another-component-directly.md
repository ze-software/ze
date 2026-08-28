---
kind: directive
level: MUST
stage:
---
**Anti-pattern:** A plugin MUST NOT call a plain exported function in another
`internal/component/*` package directly to register a callback or reach shared
engine state; it MUST go through DirectBridge/DispatchCommand instead. This
compiles and works when the plugin happens to run internal, then silently
no-ops when it runs external (the call mutates the subprocess's own
disconnected copy of that package's state). See "Process Boundary" below,
gated by `./le plugin-boundary check`.
