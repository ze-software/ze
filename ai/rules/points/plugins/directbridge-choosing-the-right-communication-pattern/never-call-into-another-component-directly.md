---
kind: directive
level: MUST NOT
stage:
---
- **A plugin MUST NOT call a plain exported function in another `internal/component/*` package directly to register a callback or reach shared engine state; it MUST go through DirectBridge or `DispatchCommand`.** The direct call compiles and works when the plugin runs internal, then silently no-ops when it runs external, because it mutates the subprocess's own disconnected copy of that package's state. Section "Plugin Process Boundary" carries the guard, and `./le plugin boundary check` enforces it.
