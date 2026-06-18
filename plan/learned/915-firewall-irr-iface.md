# 915 -- firewall-irr-iface

## Context

ISP customer-facing interfaces need ingress source validation: traffic arriving on an interface with source addresses not in the customer's IRR-registered prefixes should be dropped (BCP 38). The existing firewall-irr plugin (913) handled term-level ASN/AS-SET matching in operator-defined chains. This spec adds per-interface AS-SET bindings that generate complete ingress filter chains automatically.

## Decisions

- Separate `ze_irr_iface` table over merging into the operator's firewall table: keeps plugin-generated chains isolated from operator chains, avoids ordering surprises
- Prerouting hook over input or forward: catches both local-destined and transit traffic in one chain
- Chain policy accept with per-interface drop terms over policy drop: only filters configured interfaces; unconfigured interfaces pass through unfiltered
- Extended `allRefs()` with deduplication over separate refresh path: minimal code change, single loop refreshes both term and interface AS-SETs
- Added `store.PrefixStore.Put()` for test seeding over mock interfaces: simple, matches existing store API style

## Consequences

- Any future per-interface firewall feature (e.g., destination validation, rate limiting by interface) can follow the same pattern: add bindings to config, generate terms in `buildIfaceTables`
- The `ze_irr_iface` table name is hardcoded; a second plugin using the same name would collide via `mergeSameNameTables`
- IPv6 filtering requires the AS-SET to have cached v6 prefixes (same limitation as term-level)

## Gotchas

- None. The predecessor spec (913-firewall-irr) had already solved the hard problems (nftables sets are table-local, YANG augment paths, shared PrefixStore keys). This spec was a clean extension.

## Files

- `internal/component/firewall/plugins/irr/config.go` (modified: ifaceBinding type, parseIfaceBindings, allRefs dedup, termRefs)
- `internal/component/firewall/plugins/irr/sets.go` (modified: buildIfaceTables, ifaceTermName, constants)
- `internal/component/firewall/plugins/irr/irr.go` (modified: applyTables uses termRefs+buildIfaceTables, extractIfaceRefs for verify)
- `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` (modified: list interface with source-as-set)
- `internal/component/resolve/irr/store/store.go` (modified: Put method for test seeding)
- `internal/component/firewall/plugins/irr/config_test.go` (modified: interface binding tests)
- `internal/component/firewall/plugins/irr/sets_test.go` (modified: buildIfaceTables tests)
- `internal/component/firewall/plugins/irr/verify_test.go` (modified: uncached iface binding test)
- `test/plugin/firewall-irr-iface-reject.ci` (new: functional test for commit rejection)
- `docs/guide/firewall.md` (modified: per-interface source validation section)
- `docs/features.md` (modified: firewall feature description)
