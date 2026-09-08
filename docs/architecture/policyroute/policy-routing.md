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
<!-- source: internal/plugins/policyroute/translate.go -- ruleTerms, termName -->

All policies merge into one nftables table (type filter, hook prerouting,
priority -150) with one chain. Rule ordering stays simple, and several tables
competing for hook priority does not arise.

### One term for each named interface

A rule installs one term for each interface its policy names. A policy that
names no interface gets one term with no interface match. The interface match is
the first match of its term, and the rule's own matches follow it.

The interface leaf-list is an OR, because a packet arrives on exactly one
interface. nftables ANDs the matches inside a rule and has no branch inside one.
The alternatives therefore cannot share a term: separate rules are the only OR
nftables has.

A term is named `<policy>-<rule>` when the policy names one interface or none.
It is named `<policy>-<rule>-<N>` when the policy names several, where N is the
interface's position in the leaf-list, counted from 1.

The position rather than the interface name, because `mergeRuleCounters`
(`internal/plugins/firewall/nft/backend_linux.go`) sums every kernel rule that
shares a term name into one counter row. A shared name would report the group's
total once for each interface.

The terms of one rule stay together, in leaf-list order, where the single term
sat. At most one of them can match a packet. Their relative order therefore
decides nothing, and the `order` leaf keeps deciding which rule runs first.

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
