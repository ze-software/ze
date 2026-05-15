# 710 -- Static Route Table Selection and Interface-Only Next-Hops

## Context

Ze's static route plugin only installed routes in the main kernel table. VyOS uses `protocols static table 100 route ...` for policy-based routing and `route 0.0.0.0/0 { interface pppoe0 { distance 230 } }` for interface-only default routes. Both patterns are needed for LNS deployment (Surfprotect PBR table + PPPoE default route).

## Decisions

- Created a separate `routingtable` plugin rather than embedding the registry in the static plugin, because table-name-to-ID mapping is cross-cutting (policy routing also needs it, VRF spec will absorb it later).
- Used a package-level `atomic.Pointer[Registry]` rather than injection via plugin SDK, because `init()` self-registration prevents constructor injection and `bfdapi.GetService()` established the pattern.
- Used `map[routeKey]*routeState` (flat composite key) instead of the spec's nested `map[uint32]map[Prefix]*routeState`. Same semantics, fewer map operations, simpler code.
- Reused `nextHop` struct with zero `Address` for interface-only rather than a separate `InterfaceNextHop` type. The kernel model is unified (`Gw` and `LinkIndex` are independent fields), so a separate Go type would split what the kernel treats as one thing.
- YANG `list interface-next-hop` placed as sibling inside `forward` case (not a separate choice case), so mixed ECMP (gateway + interface-only) is valid at the schema level.

## Consequences

- Static config changed from `static { route ... }` to `static { table default { route ... } }`. All existing `.ci` tests and documentation updated.
- Config tree format is map-keyed (`{"table":{"default":{"route":{"10.0.0.0/8":{...}}}}}`) not array-of-objects. This matches `Tree.ToMap()` output. Every config parser in every plugin must use this format.
- Non-default table routes are not redistributed into BGP. The `emitRouteChange` guard returns early for `Table != 0`.
- `RouteChangeEntry` gained a `Table uint32` field (value type, pool-safe, zero = main).

## Gotchas

- `yangToList` did not flatten choice/case nodes (only `yangToContainer` did). This meant YANG `choice` inside a `list` silently dropped all case children from the config schema. Fixed by adding `flattenChoiceCases` handling in the `yangToList` loop. This was a pre-existing bug masked by cached test results.
- `Tree.ToMap()` serializes YANG lists as `map[key]value` (list key becomes map key), NOT as `[]object` with key as a named field. The old `parseStaticConfig` used array format (`.([]any)`) which never matched the daemon's runtime format. Unit tests passed because they used array-format JSON. Fixed both the static and routingtable parsers.
- `netlink.Route.Table = 0` (RT_TABLE_UNSPEC) and `254` (RT_TABLE_MAIN) both resolve to the main table in the kernel. Routes installed with Table=0 are read back with Table=254 by `RouteList`. The `listRoutes` function normalizes 254 back to 0 for consistency.
- Two plugin-list tests (`TestAvailablePlugins`, `TestAllPluginsRegistered`) need manual updates when adding a new plugin.

## Files

- `internal/plugins/routingtable/` (new): routing-table registry plugin with YANG, config, Resolve API
- `internal/plugins/static/config.go` (rewritten): map-keyed tree traversal, table resolution, interface-next-hop parsing
- `internal/plugins/static/inject.go` (modified): `routeKey` composite map key, redistribution skip for non-zero table
- `internal/plugins/static/schema/ze-static-conf.yang` (modified): `list table` wrapping, `list interface-next-hop`
- `internal/component/config/yang_schema.go` (modified): choice/case flattening in `yangToList`
- `internal/core/redistevents/events.go` (modified): `Table uint32` field on `RouteChangeEntry`
