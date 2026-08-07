---
kind: note
level:
stage:
---
The registration discipline applies to the **CLI client model**, not only the
daemon's command/schema tree. The daemon registers streaming views generically
(`pluginserver.RegisterMonitorProvider(MonitorProvider{Prefix, CreateFn})` plus
`RegisterStreamingHandler(prefix, handler)`, resolved by longest-prefix
`matchesPrefix` in `internal/component/plugin/server/handler.go`); the Bubble Tea
client mirrors this with its own view registry and must not regress into
per-feature hardcoding.
