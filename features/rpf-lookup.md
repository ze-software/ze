# RPF Lookup

## Meta

| Field | Value |
|-------|-------|
| Name | RPF Lookup |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/plugins/rib, internal/core/rib/locrib |
| Real-path tests | test/plugin/rpf-multicast.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; the rpf command is registered in internal/component/bgp/plugins/rib |
| Defect review | 2026-10-07: journal command-takes-an-untyped-positional-value (rpf arguments untyped) taken as open |

## Description

Reverse Path Forwarding query: longest-prefix-match against Loc-RIB for any CIDR family (IPv4/IPv6 unicast/multicast). Exposes `show bgp rib rpf <family> <source-addr>` command returning matched prefix, next-hop, admin distance, and metric as JSON. Generic LPM on the sharded Loc-RIB (queries all shards, picks most specific). <!-- source: internal/core/rib/locrib/manager.go -- LPM method --> <!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- rpfLookup -->
