# 401 -- Role OTC (RFC 9234 Phase 2)

## Objective

Implement RFC 9234 OTC (Only to Customer) attribute processing: ingress stamping/rejection and egress filtering via a generic peer filter chain in the reactor.

## Decisions

- `import` keyword replaces Phase 1 `name` keyword (declares role + enables non-overridable ingress rules)
- `export` keyword controls egress role filtering with `default` expansion per RFC 9234 Section 5
- Attribute types decentralized: plugins register their attribute codes via `attribute.RegisterName()` in `init()`, not hardcoded centrally
- New `bgp-aigp` stub plugin created to own AIGP attribute (code 26)
- Peer filter chain uses registry-based function registration (same pattern as InProcessNLRIDecoder)
- Filter closures capture package-level state; reactor passes only PeerFilterInfo{Address, PeerAS}
- Config keys by IP (via `extractRemoteIP`), with `filterNameToIP` mapping for named peers
- Panic recovery wrappers (fail-closed: reject/suppress) protect reactor goroutines from filter bugs

## Patterns

- Per-plugin attribute registration: `attribute.RegisterName(code, name)` from `init()`, documented as init-only
- Generic filter chain: `IngressFilterFunc` returns `(accept, modifiedPayload)`, `EgressFilterFunc` returns `bool` + writes to `*ModAccumulator` for per-peer modifications
- Name-to-IP resolution: config uses peer names as keys, filters use IP addresses; `extractRemoteIP` + `filterNameToIP` bridge the gap
- OTC egress suppression runs before `srcCfg == nil` guard to apply to IBGP-sourced routes with OTC
- `resolveExport` expands `default` token to RFC 9234 Section 5 role sets; `unknown` pseudo-role handles untagged peers

## Gotchas

- Config keys by peer NAME (from config resolution), but filters look up by IP (from reactor's netip.Addr). Critical key mismatch found by deep review -- required `extractRemoteIP` to bridge. Same mismatch affected `OnValidateOpen` (`peerConfigs[input.Peer]`) and had to be fixed separately.
- `setFilterState` clearing `filterRemoteRoles` broke tests that modified and restored filter state in subtests. Restore must also re-set remote roles.
- `insertOTCInPayload` must return nil (not original payload) on uint16 overflow, otherwise caller treats unchanged payload as "modified" and rebuilds WireUpdate unnecessarily.
- `isUnicastFamily` defined but not yet called from filters. OTC scope (IPv4/IPv6 unicast only per RFC) needs family extraction from payload -- deferred to follow-up spec.
- Egress OTC stamping (adding OTC to outgoing routes without it) uses `ModAccumulator` (already in the `EgressFilterFunc` signature). The filter writes `mods.Set("set:attr:otc", localASN)`; the reactor's `applyMods` framework applies it. Only the `applyMods` handler registration is needed -- no signature change required.
- `resolveExport` allocates per call in the hot path (per-UPDATE per-peer). Should pre-compute at config time.

## Files

- `internal/component/bgp/plugins/role/otc.go` -- OTC wire parsing, ingress/egress filters
- `internal/component/bgp/plugins/role/config.go` -- import/export config parsing, extractRemoteIP, resolveExport
- `internal/component/bgp/plugins/role/role.go` -- filter state management, name-to-IP mapping
- `internal/component/bgp/plugins/role/register.go` -- OTC attribute + filter registration
- `internal/component/bgp/plugins/role/schema/ze-role.yang` -- import/export YANG schema
- `internal/component/bgp/reactor/reactor_notify.go` -- ingress filter call site + safeIngressFilter
- `internal/component/bgp/reactor/reactor_api_forward.go` -- egress filter call site + safeEgressFilter
- `internal/component/plugin/registry/registry.go` -- PeerFilterInfo, filter types
- `internal/component/bgp/attribute/attribute.go` -- RegisterName()
- `internal/component/bgp/plugins/aigp/` -- new stub plugin
- `test/interop/scenarios/20-role-frr/`, `21-role-gobgp/` -- interop tests
