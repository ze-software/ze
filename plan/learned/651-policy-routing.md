# 651 -- Policy-Based Routing with Next-Hop Action

## Context

Ze needed policy-based routing to replace the VyOS Surfprotect config on the Exa LNS: nftables rules classify traffic on L2TP interfaces, set fwmarks, and ip rules steer marked packets to alternate routing tables (GRE tunnel to content filter). Three vendor approaches were studied: JunOS filter-based forwarding (firewall filter + routing-instance + rib-group), Nokia SR OS ip-filter with inline forward action, and VyOS policy route (iptables mark + ip rule + kernel table). Ze follows the VyOS/Linux-native model (nftables fwmark + ip rule) with a Nokia-inspired next-hop action that auto-manages routing tables.

## Decisions

- VyOS-style fwmark + ip rule over direct nftables routing expressions. Standard Linux mechanism, works on all kernels, well-understood. The nftables chain type is `route` at hook `prerouting` with priority -150.
- Next-hop action (`then { next-hop 10.0.0.1; }`) auto-allocates a kernel routing table from range 2000-2999, adds a default route via the specified next-hop, and creates the fwmark + ip rule. Users don't need to pre-create a static routing table. Same next-hop address shares a single auto-allocated table across rules.
- Reserved ranges to prevent collisions: kernel tables 1000-1999 for VRF auto-allocation, 2000-2999 for policy-routing auto tables, fwmarks 0x50000-0x5FFFF for policy-routing. User-specified table IDs in 1000-2999 and 253-255 (kernel system tables) are rejected at config parse time.
- All policies merge into a single `ze_pr` nftables table with one prerouting chain. Per-policy tables were rejected because multiple base chains at the same hook/priority produce non-deterministic evaluation order in nftables, and `accept` in one chain does not prevent marking by another. A single chain gives true first-match-wins semantics.
- Rule ordering via explicit `order` leaf (`rule bypass-dst { order 10; ... }`). Rules sorted ascending by order, then by name as tiebreaker. Chosen over numeric-only rule names (Nokia/VyOS style) to keep readable names while providing explicit ordering. JunOS-style positional ordering is impossible with JSON maps.
- Conflicting terminal actions rejected at config parse time. If a `then` block contains more than one of `table`, `next-hop`, `accept`, `drop`, the config is rejected with a clear error listing the conflicts. This prevents silent precedence surprises from map iteration order.
- Firewall table registry (`RegisterTables`/`ApplyAll`) so multiple components share one `backend.Apply`. The nft backend's Apply is full-desired-state reconciliation (deletes orphan `ze_*` tables). Without the registry, policyroute's Apply would delete firewall tables and vice versa. Both components register under owner keys; ApplyAll merges all owners' tables before calling Apply.
- Reuses firewall match/action types rather than duplicating. MatchTCPFlags and ParseTCPFlags added to the firewall model/config; policyroute imports them. SetTable is NOT a firewall action type; the policyroute component translates `table N` into `SetMark` + ip rule internally.
- IPv6 next-hop rejected at parse time with clear error. The netlink rules_linux.go uses `As4()` which would panic on IPv6. Guard at the config boundary rather than the netlink boundary.
- Policy and rule names validated via `firewall.ValidateName()` because they flow into nftables term names (`policy-rule` format).

## Consequences

- Policy routing is a standalone plugin (`internal/plugins/policyroute/`) that depends on the firewall package for types, backend, and registry, but does not depend on the firewall component/engine.
- The firewall engine was changed from `b.Apply(tables)` to `RegisterTables("firewall", tables); ApplyAll()`. This is a behavioral change: Apply now includes tables from all registered owners. If no other owners are registered, behavior is identical.
- The `ze_pr` table name is fixed (not per-policy). All policy rules live in one chain. Term names are `policyname-rulename` to avoid collisions.
- Config reload resets the mark/table allocator and re-translates from scratch. Marks are sequential (not deterministic hash), so they may change across reloads. This is fine because ip rules are also recreated.
- The plugin needs a `make generate` run to appear in `all.go` blank imports for production builds. Tests work without it because the test binary imports the package directly.

## Mistakes

- First implementation used `backend.Apply(result.Tables)` directly, which would delete all firewall `ze_*` tables as orphans. Discovered during logic review. Fixed with the table registry.
- First implementation created per-policy nftables tables (`ze_pr_surfprotect`, `ze_pr_redirect`). Review identified that multiple base chains at the same hook/priority have non-deterministic evaluation order, and `accept` only terminates within one chain. Fixed by merging all rules into a single `ze_pr` table.
- `parsePolicyAction` used early returns for each terminal action, making the first-checked action win silently when multiple were present. Fixed by scanning all terminal keys upfront and rejecting conflicts.
- `ParsePortSpec` and `ParseTCPFlags` were unexported, requiring duplication. Exported them for cross-package reuse.
- `PolicyRule.Matches` was a `[]PolicyMatch` that always contained exactly one element. Simplified to a single `PolicyMatch` field.
- Rules iterated from `map[string]any` with non-deterministic order. Added explicit `order` leaf with sort, name as tiebreaker.
- IPv6 next-hop address would panic at `As4()` in netlink code. Added IPv6 rejection at parse time.
- Policy and rule names were not validated, allowing special characters that would produce invalid nftables term names.
- Kernel system tables 253-255 were not rejected, allowing `then { table 254; }` to install rules into the main table.
