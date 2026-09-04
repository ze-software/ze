# Policy-Based Routing

`internal/plugins/policyroute` steers traffic with nftables packet marking and
`ip rule` table selection.

Config shape: `policy { route <name> { interface; rule { from; then; } } }`. The
`from` block carries source and destination address, ports, protocol, TCP flags
and `@set` references. The `then` block carries one terminal action: accept,
drop, `table N`, a next-hop IP, or `tcp-mss N`.

## Decisions

### A plugin, not a component

<!-- source: internal/plugins/policyroute/register.go -- plugin registration and lifecycle -->

Policy routing registers as a plugin, like static routes. It depends on the
firewall component for nftables backend access.

### One `ze_pr` table for every policy

<!-- source: internal/plugins/policyroute/translate.go -- table and term construction -->

All policies merge into one nftables table (type filter, hook prerouting,
priority -150) with one chain, and terms named `<policy>-<rule>`. Rule ordering
stays simple, and several tables competing for hook priority does not arise. The
interface wildcard is prepended to every rule.

### Reserved ranges by construction

<!-- source: internal/plugins/policyroute/model.go -- mark and table ranges -->

The fwmark range is `0x50000` to `0x5FFFF`. The auto table range is 2000 to 2999.
Allocation is sequential with dedup: the same next hop reuses a table.

VRF tables (1000 to 1999) and policy-routing auto tables (2000 to 2999) do not
overlap by construction. An operator-supplied table ID in 1000 to 2999
(ze-reserved) or 253 to 255 (kernel system) is rejected at PARSE time, not at
apply time.

### One terminal action per rule

Only one of accept, drop, table or next-hop is allowed. The conflict is detected
at parse time with an error listing the conflicting actions.

## The registry pattern this reinforces

`firewall.RegisterTables("policy-routes", tables)` followed by
`firewall.ApplyAll()` is how a non-firewall plugin contributes nftables tables.
The registry merges every owner's tables, so no `Apply` deletes another owner's
work.

## Related

Table IDs and route metrics reaching netlink go through the bound described in
[netlink int field truncation](netlink-int-field-truncation.md).
