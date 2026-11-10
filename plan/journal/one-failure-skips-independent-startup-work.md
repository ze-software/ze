# One failure skips independent startup work

A startup sequence stops at the first failing step, and the steps after it
never run although none of them depends on the step that failed. When the
process then keeps running, it runs with part of its startup undone and
nothing an operator reads says which part.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-10 | spec-ddos-blind-detector-diagnostic (review round 2) | plugin startup (`Server.runPluginStartup` phase 2) | Read, not run: a failed explicit-plugin phase logs ERROR and returns, so auto-load phases 3 to 5 never run; unlike phase 1 it sets no `startupErr`, so ze runs half-started | open: `spec-doctor-reports-failed-plugin-startup.md`; the fail-closed owner ruling keeps the skip; stop ze as phase 1 does, or doctor names the phases not run |
| 2026-11-10 | spec-doctor-coverage-11-hub-engine | daemon boot, management listeners (`cmd/ze/hub/main.go` `runYANGConfig`) | A REST or gRPC listener that fails to start stops the whole daemon (`start rest api`, exit 1), while SSH, web, MCP, gNMI and the looking glass that fail to bind are skipped and the daemon keeps routing; one bind failure decides two different outcomes | open: every management listener failure keeps the daemon running and raises a coded report (spec-doctor-coverage-11-hub-engine AC-6) |
| 2026-11-10 | spec-doctor-coverage-13-routing-protocols | IS-IS start (`engine.openCircuits`, `internal/plugins/isis/server.go`; `OnStarted` in `register.go`) | Read, not run: `openCircuits` returns the first `openCircuit` error, so later circuits never open and connected prefixes and the own LSP are never originated; `OnStarted` returns it, so one circuit on a missing device stops all of IS-IS. OSPF `openInterfaces` already logs and continues | open: log the circuit at Warn with its doctor code, keep it enabled for the rescan, open the rest (spec-doctor-coverage-13-routing-protocols AC-80) |
