# 913 -- firewall-irr

## Context

Firewall rules needed to match traffic by ASN or AS-SET using IRR-resolved prefix lists. The BGP component already had an IRR filter plugin; the firewall component needed the same capability but with different semantics: operator-controlled fetch (no auto-refresh by default), fail-closed commit verification, and nftables interval sets instead of BGP prefix-list matching.

## Decisions

- Used shared PrefixStore (`resolve/irr/store`) over a separate cache implementation: both BGP and firewall consumers share zefs keys (`meta/irr/{name}`), avoiding duplicate resolution and storage
- Separate plugin "firewall-irr" over extending the firewall engine: plugin self-containment means removing the directory removes the feature
- `mergeSameNameTables` in registry.go over separate tables: nftables sets are table-local, so IRR sets must live in the same nftables table as the chains referencing them. The merge happens at ApplyAll time.
- Config parser emits MatchInSet with deterministic names over new Match types: reuses existing MatchInSet lowering path, no backend changes needed beyond IntervalEnd
- Added `SetElement.IntervalEnd` to the firewall model over expanding prefixes to individual IPs: CIDR-to-interval encoding is the standard nftables pattern and scales to large prefix lists
- IPv4-only MatchInSet per term over dual-stack single-term: nftables cannot match both v4 and v6 addresses in a single rule expression in an inet table. IPv6 sets are created but require separate terms.
- refresh-interval default 0 (disabled) over always-on: the risk of invalid IRR data causing a firewall outage makes auto-refresh opt-in

## Consequences

- Any future firewall plugin that needs to register sets on the same table as the firewall engine can use RegisterTables with the same table name; mergeSameNameTables handles the merge
- The `internal/component/firewall/plugins/` directory is now a plugin discovery path in codegen (pluginDirs); future firewall plugins go there
- IPv6 IRR filtering requires the operator to write separate terms; a future spec could add term-splitting to handle both families transparently

## Gotchas

- nftables sets are table-local: a set registered on table A is invisible to chains in table B. This required the merge mechanism in ApplyAll, which was not anticipated in the original spec design.
- YANG augment path must match the actual config tree structure (`fw:table/fw:chain/fw:term/fw:from`, not `fw:filter/fw:term/fw:from`). The YANG formatter caught this.
- The shared PrefixStore already existed with per-entry zefs keys (`meta/irr/{name}`). The spec's plan to add separate `meta/firewall/irr/{name}` keys was unnecessary.

## Files

- `internal/component/firewall/plugins/irr/` (new: register.go, irr.go, config.go, cache.go, command.go, sets.go, yang/)
- `internal/component/firewall/config.go` (modified: irrSetMatch, source-asn/as-set cases)
- `internal/component/firewall/model.go` (modified: SetElement.IntervalEnd)
- `internal/component/firewall/registry.go` (modified: mergeSameNameTables)
- `internal/plugins/firewall/nft/backend_linux.go` (modified: IntervalEnd wire-through)
- `scripts/codegen/plugin_imports.go` (modified: firewall/plugins in pluginDirs)
- `internal/component/plugin/all/all.go` (regenerated)
