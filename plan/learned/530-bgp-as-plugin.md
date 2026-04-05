# 530 -- BGP as Config-Driven Plugin

## Context

BGP was always created as a subsystem in hub/main.go regardless of config content. This prevented ze from running as an interface-only manager or FIB-only route installer. The goal was to make BGP a config-driven plugin: if `bgp { }` is in config, BGP loads via ConfigRoots auto-loading; if not, ze runs without BGP. This also enables adding/removing BGP at runtime via config reload.

## Decisions

- Chose PluginCoordinator pattern over direct reactor access, to support reactor-optional operation. The coordinator delegates to the reactor when present, returns ErrBGPNotLoaded when absent.
- Chose re-reading config from disk in the reactor factory (over capturing initial LoadConfigResult) because reload-time BGP loading needs the current config, not the stale startup capture.
- Chose `slices.Contains(strings.Fields())` over `strings.Contains` for `hasConfiguredPlugin` -- the substring match caused `"ze plugin bgp-rib"` to falsely exclude the `"bgp"` plugin from auto-loading.
- Only the "bgp" plugin's config failure is fatal (exits process). Other internal plugin config failures (e.g., community filter with missing values) are non-fatal -- the plugin fails but ze continues.
- Removed `runBGPInProcess` and `subsystem.NewBGPSubsystem` entirely rather than keeping them behind a flag.
- Added SIGHUP handling to `runYANGConfig` and config loader setup for reload support.
- `SignalPluginStartupComplete` called after reload-time auto-load so `OnPostStartup` callbacks fire.

## Consequences

- ze can run without BGP (interface-only, FIB-only, or empty config). Enables modular deployment.
- BGP loads/unloads dynamically at config reload via SIGHUP. Generic mechanism works for any ConfigRoots plugin.
- `ConfigTypeUnknown` configs (environment-only) are accepted by the outer dispatcher. Previously rejected.
- The coordinator pattern adds one level of indirection for all reactor access. Performance impact is negligible (method delegation, not hot path).

## Gotchas

- `hasConfiguredPlugin` substring match was the root cause of 63/218 test failures. Debugging required tracing which plugins the auto-loader skipped, not which it loaded.
- The reactor factory's `sync.Once` prevented reload-time creation. Had to replace with mutex + nil check.
- `runYANGConfig` didn't handle SIGHUP -- only SIGINT/SIGTERM. Reload required adding signal routing and a config loader.
- `autoLoadForNewConfigPaths` didn't call `SignalPluginStartupComplete` after its `runPluginPhase`, so `OnPostStartup` (peer startup) never fired for reload-loaded BGP.
- iface `MigrateConfig` type was linux-only (`_linux.go`) with no darwin stub, causing macOS build failures unrelated to the BGP work. Fixed as prerequisite.

## Files

- `cmd/ze/hub/main.go` -- unified `runYANGConfig` path, SIGHUP reload, reactor factory
- `cmd/ze/main.go` -- accept `ConfigTypeUnknown` configs
- `internal/component/bgp/plugin/register.go` -- BGP plugin with ConfigRoots, reactor cleanup on exit
- `internal/component/plugin/coordinator.go` -- reactor-optional operation
- `internal/component/plugin/server/server.go` -- `hasConfiguredPlugin` fix, `startupErr`
- `internal/component/plugin/server/startup.go` -- error propagation
- `internal/component/plugin/server/startup_autoload.go` -- post-startup signal after reload auto-load
- `internal/component/plugin/registry/interfaces.go` -- `Stop()` on BGPReactorHandle
- `test/reload/reload-add-bgp.ci` -- AC-6 functional test
- `test/reload/reload-remove-bgp.ci` -- AC-7 functional test
