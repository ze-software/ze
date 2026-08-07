---
kind: note
level:
stage:
---
Both `monitor traceroute | log` and `monitor ping | log` bypass `ApplyPipes`
and render directly from hop/ping stats; they now apply `resolve`/`origin`
to their legend addresses via the shared `enrichAddr` helper
(`internal/component/cli/model_enrich.go`). Functional coverage:
`test/ui/monitor-ping-pipe-resolve-log.ci` drives the headless TUI with
`option=monitor:ping=fake` (deterministic ping factory + PTR/origin fakes in
`internal/component/cli/testing/fake_monitor.go`).
