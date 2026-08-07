---
kind: note
level:
stage:
---
A plugin that instead calls a plain exported Go function in another `internal/component/*` package, reaching straight into that package's shared, process-local state, works perfectly when the plugin happens to run internal (same memory), and silently does nothing useful when it runs external: the call mutates the *subprocess's own disconnected copy* of that package's state. No error, no panic, no log line. The feature just quietly never works.
