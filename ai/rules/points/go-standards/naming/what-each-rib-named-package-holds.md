---
kind: table
level:
stage:
---
| Package | Is |
|---------|----|
| `core/rib/` | Namespace, no root package: `locrib` (unified sharded Loc-RIB arbitrating best paths across protocols), `routeinstall` (RouteSink used by forked route-installing plugins in place of a direct Loc-RIB write), `store` (generic prefix-keyed route store on a BART trie) |
| `component/bgp/rib` | BGP's own RIB with per-attribute-type deduplication (the BGP Adj-RIB layer, distinct from the protocol-neutral Loc-RIB) |
| `core/routingtable` | Maps routing-table NAMES to kernel table IDs (the mapping types) |
| `plugins/routingtable` | The named-routing-table registry engine; wraps and re-exports `core/routingtable` so consumers keep a single import path |
