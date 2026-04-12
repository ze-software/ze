# 568 -- listener-dynamic-walk

## Context

Port conflict detection used a hardcoded `knownListenerServices` slice of 8 TCP
services plus a bespoke `collectWireguardListeners` helper. Every time a new service
or interface kind added `ze:listener` in YANG, a Go developer had to add a new entry
(or helper) to `listener.go`. The spec replaced both with a single YANG-schema walker
that discovers all `ze:listener`-marked list nodes dynamically at startup.

## Decisions

- Added `Listener bool` to `ListNode` in schema.go, populated by `hasListenerExtension()`
  in yang_schema.go during the YANG walk. Follows the `Sensitive`/`Hidden` precedent.
- `DiscoverListenerServices(schema)` walks the schema tree, finds `ListNode` with
  `Listener = true`, and builds a `[]listenerService` with path, protocol, and name.
- Protocol: TCP if list has `ip` + `port` children (zt:listener grouping), UDP if
  `listen-port` child. Avoids adding an argument to the `ze:listener` extension.
- Naming: drop well-known top-level grouping prefixes (`environment`, `telemetry`,
  `interface`) at position 0; keep others (e.g. `plugin`). Drop trailing `server`
  list name. Join with "-". Produces identical names to the old hardcoded list.
- `hasEnabledLeaf`: checked from the schema parent container, not the config tree.
  Services whose YANG parent has an `enabled` leaf require `enabled=true` in config
  (YANG default false). Services without one (plugin-hub) are always collected.
- Changed `CollectListeners` signature from `(tree *Tree)` to `(tree *Tree, schema *Schema)`.
  Both callers already had the schema available.
- Deleted `knownListenerServices` and `collectWireguardListeners` entirely.

## Consequences

- Adding a new `ze:listener` to any YANG module is now self-describing: no Go code
  change needed for port conflict detection to discover it.
- The `CollectListeners` signature change is internal (not in the plugin SDK), so
  no backwards-compatibility concern.
- "telemetry-prometheus" would have been the name under a simpler algorithm; adding
  `telemetry` to the dropped-prefix list preserved the original "prometheus" name.

## Gotchas

- The `hasEnabledLeaf` check must inspect the schema parent container, not the config
  tree. A missing `enabled` in the config tree is ambiguous: it could mean "YANG has
  an enabled leaf defaulting to false" (skip) or "YANG has no enabled leaf" (always
  collect). Only the schema resolves this.
- `walkListenerNodes` passes the parent node to `buildListenerService` specifically
  to support this check.
- Tests use `YANGSchema()` (which is not cached) to get the real schema. This makes
  the tests integration-level rather than pure unit, but avoids building a mock schema
  that could drift from the real YANG.

## Files

- `internal/component/config/schema.go` -- `ListNode.Listener` field
- `internal/component/config/yang_schema.go` -- `hasListenerExtension()`
- `internal/component/config/listener.go` -- `DiscoverListenerServices`, `CollectListeners`
- `internal/component/config/listener_test.go` -- 3 new tests, 8 updated
- `cmd/ze/config/cmd_validate.go` -- caller passes schema
- `internal/component/bgp/config/loader_create.go` -- caller passes schema
