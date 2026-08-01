# 751: FlowSpec-to-Firewall Bridge

## Context
Ze had full FlowSpec NLRI codec support (RFC 8955/8956) and a multi-owner firewall registry but no connection between them. FlowSpec routes were received, parsed, and stored in the RIB but never translated into actual nftables rules.

## Decision
Created a new `flowspec-firewall` plugin (`internal/plugins/flowspec-firewall/`) that subscribes to BGP UPDATE events directly (bypassing the RIB) and translates FlowSpec NLRI + extended community actions into firewall terms via the existing `firewall.RegisterTables`/`ApplyAll` multi-owner registry.

Key design choices:
- **RIB bypass**: FlowSpec routes are ephemeral filtering instructions, not routing state. The bridge receives UPDATE events directly via `SetStartupSubscriptions` + `OnEvent` and maintains its own per-peer rule map.
- **Hook selection**: Two chains in one `ze_flowspec` inet table: `flowspec-fwd` (forward hook) for transit traffic, `flowspec-in` (input hook) for locally-destined traffic. Selection based on whether the FlowSpec destination prefix contains any local interface address (tracked via `(interface, addr-added/addr-removed)` EventBus events).
- **Unsupported components rejected**: Rules containing packet-length, fragment, flow-label, or ICMP code components are not installed (silent ignore per RFC 8955 Section 6). Conservative matching that drops more traffic than intended is dangerous.
- **No-action rules skipped**: FlowSpec routes without traffic actions (discard, rate-limit, mark) are not installed since accept-only rules are nftables clutter.

## Consequences
- FlowSpec routes from BGP peers produce real nftables rules.
- Peer session teardown removes all rules from that peer.
- Max-rules cap (default 1000) prevents DoS via rule flooding.
- VPN FlowSpec (SAFI 134), redirect action, and traffic-action (sample/terminal) are deferred to v2.

## Gotchas
- The JSON event format uses `"ipv4/flow"` as the family key (from `family.Family.String()`), not `"ipv4/flowspec"`.
- FlowSpec NLRI JSON has nested array-of-arrays for OR groups: `"destination":[["10.0.0.0/24"]]`.
- Numeric component values in JSON have operator prefixes: `"=6"`, `">80"`, `">=1024"`. Only equality operators are supported in v1; range operators (`>=`, `<=`, `>`, `<`, `!=`) cause the rule to be rejected. Silently treating them as equality would narrow the match dangerously.
- The firewall multi-owner registry merges all owners on `ApplyAll`, so the flowspec owner's tables coexist with config-driven and policy-route tables.
- FlowSpec Type 4 (Port = src OR dst) requires two nftables terms per rule, since nftables AND-s matches within a term. The bridge splits them automatically.
- Term ordering in chains must be deterministic (sorted by name hash) since nftables evaluates rules sequentially and overlapping rules from different peers would otherwise have unpredictable priority across restarts.
- `buildTable` iterates Go maps (non-deterministic) so explicit sorting is mandatory before building the chain term list.

## Files

None recorded.
