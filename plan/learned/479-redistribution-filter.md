# 479 -- Redistribution Filter

## Context

Ze had no policy filter infrastructure for external plugins. In-process filters existed (OTC via role plugin), but there was no way for external plugins to act as route filters on ingress or egress. Users needed the ability to reference plugin-provided filters in config (`redistribution { import [ rpki:validate ] }`) with cumulative chain resolution across bgp/group/peer levels, piped transform semantics, and reject short-circuit.

## Decisions

- Plugins declare named filters at stage 1 (`declare-registration`) over requiring compile-time registration. This keeps filter capability as runtime IPC, not a new plugin type. One plugin can declare multiple named filters (e.g., `rpki:validate` and `rpki:log`).
- Config validation is format-only at parse time (`<plugin>:<filter>` format), over validating against plugin registry. Plugins start after config parsing, so plugin/filter existence is checked at runtime (filter-update call time). This is a deliberate design trade-off, not a missing feature.
- Three filter categories (mandatory/default/user) with override mechanism, over a flat chain. Override infrastructure (`applyOverrides`, `DefaultImportFilters`/`DefaultExportFilters`) is built but default/mandatory filters are not yet populated -- deferred to `spec-named-default-filters`.
- Text protocol for filter IPC (`filter-update` RPC with text-format attributes) over binary/wire encoding. Simpler for external plugins; raw mode field exists but is not yet wired.
- Text-level dirty tracking (delta merge in `applyFilterDelta`) over wire-level re-encoding. Wire-level dirty tracking (rebuilding only modified attributes via ModAccumulator) deferred to `spec-redistribution-filter-phase2`.
- `redistribution.go` placed in `internal/component/bgp/config/` (not `internal/component/config/` as originally spec'd) because it depends on bgpconfig types.

## Consequences

- External plugins can now declare route filters and receive filter-update callbacks with accept/reject/modify responses. Six functional tests prove the wiring end-to-end.
- The override infrastructure is ready but inert until default named filters are populated. When `spec-named-default-filters` converts loop/OTC to named defaults, `applyOverrides` activates automatically.
- Wire-level re-encoding is the main performance gap: modify responses work at the text level but wire bytes are not rebuilt from modified text. For import filters this means the cached wire bytes may not reflect text-level modifications. `spec-redistribution-filter-phase2` addresses this.
- `PolicyFilterChain` is a pure function (no reactor dependency) -- easy to test and reuse. The reactor bridges to it via `policyFilterFunc()` which handles IPC timeout (5s) and plugin lookup.

## Gotchas

- Config validation cannot check plugin/filter existence at parse time because plugins declare at stage 1 (after config). Runtime errors surface at filter-update call time, not startup.
- `redistribution_test.go.broken` exists alongside the working test file -- leftover from a failed approach.
- The spec listed `internal/component/config/redistribution.go` as the file path, but the actual location is `internal/component/bgp/config/redistribution.go`. Future specs should verify package paths before listing them.
- `SendFilterUpdate` lives in `internal/component/plugin/ipc/rpc.go` (engine-to-plugin IPC), not in `pkg/plugin/sdk/` (plugin-side SDK). The SDK side is `OnFilterUpdate`/`handleFilterUpdate`.

## Files

- `internal/component/bgp/reactor/filter_chain.go` -- PolicyFilterChain, PolicyFilterFunc, applyFilterDelta, policyFilterFunc
- `internal/component/bgp/reactor/filter_chain_test.go` -- 8 unit tests (chain + delta)
- `internal/component/bgp/config/redistribution.go` -- extractRedistributionFilters, validateFilterRefs, applyOverrides, DefaultImportFilters
- `internal/component/bgp/config/redistribution_test.go` -- 5 config tests
- `internal/component/plugin/registration.go` -- FilterRegistration struct
- `internal/component/plugin/registration_test.go` -- +2 filter declaration tests
- `internal/component/plugin/ipc/rpc.go` -- SendFilterUpdate
- `internal/component/plugin/server/server.go` -- CallFilterUpdate
- `pkg/plugin/sdk/sdk_callbacks.go` -- OnFilterUpdate
- `pkg/plugin/sdk/sdk_dispatch.go` -- handleFilterUpdate
- `internal/core/ipc/schema/ze-plugin-engine.yang` -- filters list in declare-registration
- `internal/core/ipc/schema/ze-plugin-callback.yang` -- filter-update RPC
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- redistribution container
- `internal/component/bgp/reactor/reactor_notify.go` -- ingress policy chain wiring
- `internal/component/bgp/reactor/reactor_api_forward.go` -- egress policy chain wiring
- `docs/guide/redistribution.md` -- user guide
- `test/plugin/redistribution-{import-accept,import-reject,import-modify,export-reject,declare,chain-order}.ci` -- 6 functional tests
