# 184 — Plugin YANG Discovery

## Objective

Add `--plugin` CLI flag to `ze bgp server` and `--yang` introspection to plugin commands,
plus in-process execution of built-in plugins via goroutine + `io.Pipe`.

## Decisions

- `ze.X` prefix → in-process (goroutine + `io.Pipe`); path/cmd → fork. Same 5-stage protocol for both.
- CLI `--plugin` and config `plugin {}`/`process {}` are additive; dedup by name.
- `ze --plugin` (no args) lists available internal plugins for `--plugin auto` discovery.
- `Process` struct handles both internal and external — avoids separate code paths in server.
- `Stop()` closes stdin for internal plugins because context cancellation doesn't unblock a goroutine blocked on `io.Pipe.Read()`.
- Single registry in `inprocess.go` is the source of truth — no duplicate lists.

## Patterns

- `io.Pipe` gives same interface as OS pipes, so plugin implementations need zero changes for in-process execution.
- `Internal bool` field added to `PluginConfig` in three places (bgp.go, types.go, reactor.go).

## Gotchas

- Internal plugins ignored `Stop()` — only context cancellation was attempted; `io.Pipe` read blocks forever without stdin close.

## Files

- `internal/plugin/inprocess.go` — registry + `InternalPluginRunner` type
- `internal/plugin/process.go` — `startInternal()` / `startExternal()` split, `Stop()` fix
- `internal/plugin/resolve.go` — resolution rules (ze.X, path, cmd)
- `internal/config/loader.go` — `LoadReactorFileWithPlugins()`, `mergeCliPlugins()`
- `cmd/ze/bgp/server.go` — `--plugin` flag
- `cmd/ze/bgp/plugin_{rib,gr,rr}.go` — `--yang` flag
