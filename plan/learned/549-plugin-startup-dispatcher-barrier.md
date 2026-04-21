# 549 -- Plugin Startup Dispatcher Barrier

## Context

`bgp-rpki` calls `DispatchCommand("adj-rib-in enable-validation")` at startup to
arm the Adj-RIB-In validation gate so only valid/not-found routes reach the RIB.
The call was in `OnStarted`, which fires after the plugin's own 5-stage
handshake but potentially BEFORE other plugins in later startup phases are
loaded. In the reproducer `test/plugin/prefix-filter-accept.ci`, `bgp-rpki`
auto-loaded in Phase 1 (via `ConfigRoots: ["bgp"]`) while `bgp-adj-rib-in`
loaded in Phase 2 (explicit `--plugin ze.bgp-adj-rib-in`). Phase 1 finished all
its plugins through `StageRunning` before Phase 2 even started, so
`OnStarted` hit a dispatcher command registry that did not yet contain
`adj-rib-in enable-validation`, failed with `unknown command`, returned error,
and the plugin exited with code 1. Every subsequent `validate-open` RPC on
bgp-rpki hung, every peer session stayed in OpenConfirm, and every `.ci` test
that happened to auto-load bgp-rpki silently failed upstream. Commit
`1fc98747` shipped a graceful-degrade workaround (log a warning, return nil)
that kept the tests green at the cost of silently disabling end-to-end
enforcement whenever the race triggered.

## Decisions

- **Added a new callback `ze-plugin-callback:post-startup`** sent by the engine
  after `signalStartupComplete` has frozen both the plugin registry and the
  dispatcher command registry. At that point every plugin from every phase has
  registered every command, so cross-plugin dispatch is guaranteed to resolve.
  Chose this over a barrier in the startup coordinator because the coordinator
  only spans a single tier of a single phase -- it cannot close a cross-phase
  gap by construction.
- **Added `OnAllPluginsReady(fn func() error)` to the SDK** that registers a
  handler for the new callback in the callbacks map. Chose this over delaying
  `OnStarted` itself because external plugins run `OnStarted` synchronously
  inside `Run()` before the event loop starts; waiting for an engine callback
  inside `OnStarted` would deadlock (no event loop to deliver the callback).
  Dispatching via the event loop means `OnAllPluginsReady` fires AFTER
  `OnStarted` has returned, with no timing contract for plugins that use both.
- **Best-effort fan-out**: the engine sends post-startup in a per-plugin
  goroutine with a bounded 10s timeout, Debug-level error logging. Chose this
  over a synchronous serial fan-out because a single slow handler must not
  delay notification to the rest, and a dead connection must not block any
  engine bookkeeping. A plugin that died before post-startup simply never
  receives the callback, which matches the advisory nature of the signal.
- **bgp-rpki restored error-returning semantics.** The graceful workaround
  (`return nil` on dispatch failure) is deleted. If
  `OnAllPluginsReady` now fails, the engine sees a real error, which is
  correct because the only path to failure is `bgp-adj-rib-in` genuinely
  missing (not a timing race).
- **Rejected: removing `bgp-rpki`'s `ConfigRoots`.** Tempting narrow fix (stop
  auto-loading bgp-rpki from any `bgp { }` section) but it fails any plugin
  author who adds an inter-plugin startup dispatch later. The generic
  `OnAllPluginsReady` primitive fixes them all.
- **Rejected: blocking in `OnStarted` with a `WaitForAllPlugins` primitive.**
  External plugins deadlock: no event loop running to deliver the release
  signal.

## Consequences

- Any plugin that needs to issue a cross-plugin `DispatchCommand` at startup
  now has a well-defined, documented place to put it (`OnAllPluginsReady`),
  regardless of which startup phase the plugin happens to load in.
- `OnStarted` is now explicitly documented (in
  `docs/architecture/api/process-protocol.md`, `ai/rules/plugin-design.md`,
  and the godoc on `OnStarted`) as "local setup only; do NOT dispatch to other
  plugins from here". Future inter-plugin dispatches from `OnStarted` will be
  caught at review time, not by silent race.
- The engine now emits an extra engine-to-plugin RPC per plugin at
  `signalStartupComplete` time. Bounded by the number of loaded plugins, one
  goroutine each, 10s ceiling per call, Debug-level errors: a non-concern at
  realistic fleet sizes. Reload's `autoLoadForNewConfigPaths` path also
  benefits because it calls `signalStartupComplete` after its own phase.
- Plugins can now rely on the guarantee "at the moment
  `OnAllPluginsReady` fires, every command declared by every plugin in the
  current daemon is in the dispatcher command registry and the registry is
  frozen". This is the same invariant that `CommandRegistry.Freeze()` already
  enforces for dispatch-side lookups; we just expose it to plugin authors.

## Gotchas

- The handover framed the fix as "a barrier in the plugin-startup coordinator
  between Stage 5 (Ready) and `OnStarted`". That framing is wrong. The
  per-tier barrier in `startup_coordinator.go` already serializes command
  registration within a single tier. The failure is CROSS-PHASE: Phase 1 fully
  completes (OnStarted on every Phase 1 plugin) before Phase 2 even starts, so
  no coordinator barrier can close the gap. Future sessions reading a handover
  that proposes a "coordinator barrier" should reach for a post-Freeze event
  instead.
- The reproducer (`prefix-filter-accept.ci`) does NOT obviously involve
  bgp-rpki: the config has no `bgp { rpki { } }` section and the `--plugin`
  flags only mention `ze.bgp-filter-prefix` and `ze.bgp-adj-rib-in`. bgp-rpki
  gets pulled in by Phase 1 config-path auto-load because its `ConfigRoots`
  is the string `"bgp"` which matches any config with a top-level bgp section.
  If you are debugging a flaky test that touches BGP config, always check
  whether an auto-loaded plugin is in flight even when the test author didn't
  ask for it. `make ze-verify 2>&1 | grep "auto-loading plugin for config
  path"` is a fast sanity check.
- `OnAllPluginsReady` takes a `func() error`, not `func(context.Context) error`.
  The callback handler signature in the SDK `callbackHandler` type is
  context-less; the event loop does not thread its context through. If the user
  function needs a context they must create one (typically
  `context.WithTimeout(context.Background(), ...)`) inside the handler. bgp-rpki
  demonstrates the pattern.
- External plugins' `OnAllPluginsReady` runs in the MuxConn event loop, internal
  bridge plugins' runs in the bridge event loop, and both dispatch through the
  same `p.callbacks` map. If you ever add a bridge-specific fast path for
  post-startup, keep the callbacks map as the single source of truth for
  handler lookup -- the SDK's generic-callback invariant
  (`rules/plugin-design.md` -- "SDK Is Generic") should not slip.
- `signalStartupComplete` is called on error paths too. `sendPostStartupToAll`
  fires on those paths as well, which is intentional: surviving plugins still
  benefit from the notification, and failed plugins silently skip it via the
  `proc.Running()` check.

## Files

- `pkg/plugin/sdk/sdk_dispatch.go` -- new `callbackPostStartup` constant
- `pkg/plugin/sdk/sdk.go` -- new `onAllPluginsReady` field
- `pkg/plugin/sdk/sdk_callbacks.go` -- new `OnAllPluginsReady` method, new
  default no-op entry for `callbackPostStartup`
- `pkg/plugin/sdk/sdk_test.go` -- three new unit tests
- `internal/component/plugin/ipc/rpc.go` -- new `SendPostStartup` method
- `internal/component/plugin/server/startup.go` -- `sendPostStartupToAll` +
  `postStartupTimeout`, invoked from `signalStartupComplete`
- `internal/component/bgp/plugins/rpki/rpki.go` -- dispatch moved from
  `OnStarted` to `OnAllPluginsReady`; workaround deleted
- `docs/architecture/api/process-protocol.md` -- Post row in the stage table,
  "Post-Startup Callback" paragraph, "Cross-Plugin DispatchCommand from Startup"
  rule
- `ai/rules/plugin-design.md` -- Post row in the stage table, new
  "OnStarted vs OnAllPluginsReady (BLOCKING)" section
