# Plugin Process Boundary

**When:** Writing or reviewing a plugin that calls another in-process package's plain exported function directly (not through DirectBridge/DispatchCommand) to register a callback, fetch a live backend handle, or otherwise touch that package's process-local state.
**Severity:** advisory

## The problem

A Ze plugin can run **internal** (a goroutine sharing the daemon's own process, wired via `internal/component/plugin/process/process.go`'s `startInternal`) or **external** (a forked subprocess talking only over TLS/RPC, via `startExternal`). Plugin code is supposed to reach the engine only through the SDK's RPC layer, which handles this difference transparently.

A plugin that instead calls a plain exported Go function in another `internal/component/*` package -- reaching straight into that package's shared, process-local state -- works perfectly when the plugin happens to run internal (same memory), and silently does nothing useful when it runs external: the call mutates the *subprocess's own disconnected copy* of that package's state. No error, no panic, no log line. The feature just quietly never works.

Five confirmed instances, all fixed:

| Plugin | Same-process-effect call | Severity | Fix |
|--------|---------------------------|----------|-----|
| `as112` | `iface.RegisterOwnedAddresses`/`UnregisterOwnedAddresses` | total feature loss | refuse to start external |
| `cos` | `iface.GetBackend` | partial (dynamic QoS updates only) | warn |
| `traffic-usage` | `iface.SubscribeCollectNotify` | total feature loss (only attach mechanism) | refuse to start external |
| `flow-export` | `iface.SubscribeCollectNotify` | total feature loss (only data source) | refuse to start external |
| `ddos-detect` | `iface.SubscribeCollectNotify` + `trafficstat.EnsureGlobal`/`Global` | total feature loss (both paths affected) | warn |

## The rule

If your plugin calls a same-process-effect function directly, check `sdk.Plugin.IsInternal()` (`pkg/plugin/sdk/sdk.go`) right after `sdk.NewWithConn(...)` and choose severity by how much of the plugin's value survives running external:

- **The call is the plugin's core purpose** (nothing useful happens without it) -> hard refuse: log an error naming the specific call and why, `return 1` before doing anything else. See `internal/plugins/as112/register.go`, `internal/plugins/trafficusage/register.go`, `internal/plugins/flowexport/register.go`.
- **The plugin still provides real value external** (only one feature degrades) -> warn: a `warnIfExternal(isInternal bool)` helper, called once after `sdk.NewWithConn`, logging what breaks and what still works. See `internal/plugins/cos/register.go`, `internal/plugins/ddos/detect/register.go`.

Do not copy-paste the severity choice between plugins -- judge each one on what actually survives.

## The mechanical check

`make ze-plugin-boundary-check` (wired into `ze-verify`/`ze-verify-changed`) runs `scripts/checks/plugin_process_boundary.go`: it scans every package under the generator's plugin search roots -- derived at runtime from `scripts/codegen/plugin_imports.go`'s `pluginDirs` + `nestedPluginDomains` (13 namespaces today, including `internal/component/l2tp/plugins/` and `internal/component/firewall/plugins/`), never a second hardcoded list -- for calls to a maintained dangerous-call list, and fails if a plugin package contains one with no `.IsInternal()`/`warnIfExternal(` call anywhere in that same package. `--print-roots` shows the derived set. This is a presence heuristic (it does not prove the guard actually covers the call at runtime), the same rigor level as the sibling `ze-iface-resolution-check`.
<!-- source: scripts/checks/plugin_process_boundary.go -- loadScanRootsFrom -->

Add a new entry to `scripts/checks/plugin_process_boundary.go`'s `dangerousCalls` list whenever a new instance of this class is found and fixed, so the check stays current. Add a new `allowlist` entry only for a package's own legitimate calls to its own function.
