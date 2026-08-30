---
kind: directive
level: MUST
stage:
---
**Logging MUST go through `slogutil.Logger("subsystem")` in the engine and `slogutil.PluginLogger("name", level)` in a plugin; `fmt.Printf` and the `log` package MUST NOT be used, and debug logging is permanent rather than temporary.** Levels are `disabled`, `debug`, `info`, `warn`, `err`, set per subsystem by the hierarchical `ze.log.<path>` env var or the `environment { log { ... } }` config, with a CLI flag beating an env var beating config beating the WARN default.
