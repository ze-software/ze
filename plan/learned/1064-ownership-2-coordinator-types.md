# 1064 -- ownership-2-coordinator-types

## Context

The plugin registration hooks and the coordinator's cross-plugin state bag passed
everything as `any`: `Registration.ConfigureEventBus/ConfigureMetrics/ConfigurePluginServer`
were `func(any)` (forcing every plugin to type-assert in its callback), the backing
registry globals were `any`, and the coordinator carried a string-keyed
`extra map[string]any` for 9 BGP-bootstrap values (each with a known concrete type
at both writer and reader). DESIGN-REVIEW finding #1 flagged this `any`-typed
plumbing. Research found only `ConfigurePluginServer`'s concrete type is cycle-forced
(answered by the existing leaf `PluginServerAccessor` interface); the rest are `any`
by convention and can be typed today.

## Decisions

- Typed the 3 hooks to `ze.EventBus` / `metrics.Registry` / `registry.PluginServerAccessor`
  over leaving them `any`: `pkg/ze` is a zero-codeberg-dep leaf and `core/metrics` is
  below `component`, so `registry`->both is downward and cycle-free (`go list -deps`
  confirmed, `make ze-tier-check` green). This converted ~65 per-plugin runtime
  assertions into compile-time-checked signatures across 47 `register.go` files.
- Replaced the 9-key `extra map[string]any` with a typed `BGPBootstrap` struct in the
  leaf `registry` package (not a bgp package), so both the hub (writer) and bgp/config
  (reader) name it without a cycle. Chose to type `Store storage.Storage` (the one field
  needing a new `registry -> config/storage` lateral import) over leaving it `any`:
  `config/storage` depends only on `pkg/zefs`, so no cycle. AC-3 achieved with zero
  residual `any`.
- Left genuinely cycle-forced `any` alone (documented): reactor `SetEventBusAny`/
  `SetPluginServerAny`, `PeerLifecycleCallback`/`MessageCallback` `peer any` params,
  the `manager.metricsRegistry` own field, and the `reactors map[string]any` (kept
  generic for OSPF/IS-IS). Did NOT add a `BGPReactor()` accessor (optional in spec).

## Consequences

- Any future plugin that mis-shapes a Configure callback is now a hard compile error,
  not a silent runtime assertion failure. The compiler is the sweep tool: retyping the
  hook surfaced every implementer as a build error (the worklist).
- The registry leaf now has one component->component lateral edge (`config/storage`).
  It is cycle-free but is the first such edge; future additions to `BGPBootstrap` that
  reference component types must re-check the cycle.
- `GetMetricsRegistry()`/`GetEventBus()`/`GetPluginServer()` now return typed values, so
  Get-side callers no longer need `.(T)` (6 redundant assertions removed; 5 files lost
  their now-unused `metrics` import).

## Gotchas

- Typed-nil vs interface-nil: the writers store interface-typed values
  (`var PeerLifecycleCallback registry.PeerLifecycleCallback`; `SetMetricsRegistry` takes
  `metrics.Registry`), so an unset value is a true nil interface. Dropping the old
  `if x != nil` write-guard in main.go and relying on the reader's `!= nil` is therefore
  behavior-preserving. If a writer ever passed a typed-nil pointer, `!= nil` would differ
  from the old `.(T),ok` -- but no writer does.
- `check_cross_package_wiring` (make ze-validate) re-audits ALL exported symbols in a
  touched file, so editing `registry.go` surfaces pre-existing dead exports
  (`AttrModHandlerFor`, `PipelineSnapshot`, several reactor methods). Those are not P2
  regressions; do not "fix" them under this spec.
- The `plugin/all` wire-methods golden can fail from ANOTHER session's uncommitted work
  (a dns-cache/resolve-cmd rename) because P2 pulls `plugin/all` into the changed-package
  scope. Attribute the red before treating it as yours: the failing methods were owned by
  a component P2 never touched.

## Files

- `internal/component/plugin/registry/registry.go` -- typed 3 hooks, 3 globals, 6 accessors, snapshot field
- `internal/component/plugin/registry/interfaces.go` -- `BGPBootstrap` struct; `CoordinatorAccessor.GetExtra`->`Bootstrap()`
- `internal/component/plugin/coordinator.go` -- `extra map` -> typed `bootstrap` field; `SetBootstrap`/`Bootstrap`
- `internal/component/plugin/registry/registry_test.go`, `coordinator_test.go` -- typed-hooks + bootstrap round-trip tests
- `cmd/ze/hub/main.go` -- 9 `SetExtra` -> one `SetBootstrap`
- `internal/component/bgp/config/register.go`, `loader.go` -- read via `coord.Bootstrap()`
- ~47 plugin `register.go` files -- dropped the Configure* `.(T)` assertions
- `cmd/ze/hub/service_gnmi.go`, `internal/component/{bfd/bfd.go,l2tp/subsystem.go,pki/health.go,trafficstat/register.go,trafficfeature/register.go}` -- redundant Get-side assertion + unused-import cleanup
