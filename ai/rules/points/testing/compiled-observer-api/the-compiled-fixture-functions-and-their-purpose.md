---
kind: table
level:
stage:
---
| Function | Purpose |
|----------|---------|
| `fixture.Register(name, driver)` | Register one compiled fixture command |
| `fixture.Run(args)` | Dispatch `ze-test fixture <name> [args...]` |
| `fixture.Observe(...)` | Connect through the SDK, complete startup, run the scenario after all plugins are ready, then request shutdown |
| `fixture.ObserveConfigured(...)` | Install callbacks before startup, then run the same observer lifecycle |
| `fixture.Dispatch(...)` | Send one command and decode its JSON answer into a Go value |
| `fixture.Poll(...)` | Retry a predicate until success, exhaustion, or context cancellation |
| `fixture.ReportFailure(err)` | Emit the observer-failure sentinel the runner treats as authoritative |
| `sdk.Plugin.DispatchCommand(...)` | Send a typed command request through the plugin connection |
