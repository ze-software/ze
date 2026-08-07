---
kind: note
level:
stage:
---
A Ze plugin can run **internal** (a goroutine sharing the daemon's own process, wired via `internal/component/plugin/process/process.go`'s `startInternal`) or **external** (a forked subprocess talking only over TLS/RPC, via `startExternal`). Plugin code is supposed to reach the engine only through the SDK's RPC layer, which handles this difference transparently.
