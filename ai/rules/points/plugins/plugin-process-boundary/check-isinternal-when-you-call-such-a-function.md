---
kind: directive
level: MUST
stage:
---
- **A plugin that calls a same-process-effect function in another `internal/component/*` package directly MUST check `sdk.Plugin.IsInternal()` right after `sdk.NewWithConn(...)`.** Such a call works when the plugin runs internal and silently no-ops when it runs external, with no error, no panic and no log line. Why it is silent is `docs/architecture/plugin/plugin-system.md`, "The process boundary".
