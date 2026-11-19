# Policy-Based Routing

`internal/plugins/policyroute` steers traffic with nftables packet marking and
`ip rule` table selection.

Config shape: `policy { route <name> { interface; rule { from; then; } } }`. The
`from` block carries source and destination address, ports, protocol, TCP flags
and `@set` references. The `then` block carries at most one terminal action:
accept, drop, `table N` or a next-hop IP. `tcp-mss N` may accompany a terminal
action or stand alone; alone it adjusts MSS without adding a terminal action.

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

### A `@set` reference is a copy of a firewall table's set

<!-- source: internal/plugins/policyroute/sets.go -- parsePolicySections, verifyPolicySections, resolveSetReferences, firewallSetCatalog, checkSetType, undeclaredSetError -->
<!-- source: internal/plugins/policyroute/translate.go -- referencedSets, parseAddressMatch, parsePortMatch -->
<!-- source: internal/plugins/policyroute/register.go -- ConfigReads -->
<!-- source: internal/component/firewall/provided_sets.go -- ProvidedSetOwner -->

An nftables set resolves only inside the table that holds the rule, so the
`ze_pr` chain cannot read a set another table declares. Policy routing copies
each set its rules name into `ze_pr`, with the set's type, flags and elements.
An address leaf names an `ipv4` or `ipv6` set, and a port leaf an
`inet-service` set.

The firewall section is a read root of the plugin (`ConfigReads`). The engine
delivers it whole beside `policy route` whenever either root changes, and
`ze config validate` delivers it to the verifier too. The verify resolves each
`@name` on `source-address`, `destination-address`, `source-port` and
`destination-port` against the sets the firewall tables declare, and refuses at
commit a name no table declares, a name two tables declare, and a set the leaf
cannot read. The type is checked by leaf: an address leaf takes only an `ipv4`
or `ipv6` set, a port leaf only an `inet-service` set. A name built from the
prefix of a set another plugin fills at run time (the IRR sets of
`firewall-irr`, the domain-group sets of `firewall-domain`) is not declared in
any table, so its refusal names that plugin and the firewall leaf that matches
such a set (`firewall.ProvidedSetOwner`) instead of asking the operator to
declare it.

The resolved set rides on the rule in one of four fields
(`PolicyMatch.SourceSet`, `DestinationSet`, `SourcePortSet`,
`DestinationPortSet`), and translate refuses an `@name` that carries none, so
no apply can name a set `ze_pr` lacks. The verify (`verifyPolicySections`)
also runs the translation the apply will run, on a fresh allocator that touches
neither the plugin's state nor the kernel, so a refusal only translate gives (a
port or `tcp-flags` value it cannot read, the mark or table ranges running out)
surfaces at commit rather than at apply.

The cost is two kernel copies of one set, kept equal because both are written
from the same commit's firewall section. That forecloses a set whose elements
come from anywhere but the configuration: a set another owner registers (IRR,
domain groups) is not in the firewall section and is refused, and an element
written into the kernel set by hand reaches one copy only.

### A reinstall changes only what changed

<!-- source: internal/plugins/policyroute/rules_diff.go -- policiesUnchanged, replaceRules -->
<!-- source: internal/plugins/policyroute/register.go -- applyPolicies -->

Because the firewall root is a read root, every firewall commit reaches the
apply, and most of them change no set a policy names. The policies carry their
resolved sets, so the apply compares them with the installed ones
(`policiesUnchanged`) and touches nothing when they are equal: a commit that
changes a set no policy names reinstalls nothing, while one that changes a
referenced set's elements is a change. Before a first install succeeded there
is nothing to keep, so the comparison never skips it.

When something changed, the `ze_pr` table is registered and applied first, then
the ip rules and auto routes move by difference (`replaceRules`). What the new
result holds and the old one does not goes in first, auto routes before the
rules that steer into their tables. What only the old result held comes out
after, rules before routes. A rule both hold is never touched, so a packet a
rule steered before and after the change is steered throughout. When an install
fails, what that call added is removed again, the previous tables are
registered and applied again, and the previous rules stay in place, so the
failure is reported over the state the transaction recorded.

### Reserved ranges by construction

<!-- source: internal/plugins/policyroute/model.go -- mark and table ranges -->

The fwmark range is `0x50000` to `0x5FFFF`. The auto table range is 2000 to 2999.
Allocation is sequential with dedup: the same next hop reuses a table.

Ze reserves three table ranges: VRF tables (1000 to 1999, held for VRFs, no
producer yet), policy-routing auto tables (2000 to 2999) and the IKE engine's
IPv6 tunnel-source guard tables (3000 to 3999). Each is declared once in
`internal/core/iprule` (`VRFTables`, `PolicyRouteAutoTables`,
`IPsecSourceGuardTables`), `TestTableRangesAreDisjoint` checks them against each
other, and `iprule.Reserved` answers the range that holds a table ID. An
operator-supplied table ID inside any of them is refused at PARSE time, not at
apply time: a policy route `then table` (`validateActionTable`) and a
`routing-table` id (`routingtable.ValidateTableID`) both ask `iprule.Reserved`.
Table IDs 253 to 255 (kernel system) are refused too. The
`ip rule` encoder lives there too (`iprule.Mark.Rule`): the IKE engine builds
source-prefix rules with the same encoder (`iprule.Source.Rule`), and a gated
component cannot import this gated plugin.

<!-- source: internal/core/iprule/iprule.go -- TableRange, PolicyRouteAutoTables, IPsecSourceGuardTables, Reserved, Mark -->
<!-- source: internal/core/routingtable/registry.go -- validateTableID -->
<!-- source: internal/core/iprule/iprule_linux.go -- newRule, Mark.Rule, Source.Rule -->

### One terminal action per rule

Only one of accept, drop, table or next-hop is allowed. The conflict is detected
at parse time with an error listing the conflicting actions. A TCP-MSS-only rule
keeps its unspecified terminal action through translation, so it adds neither a
verdict nor a routing-table selection.

## The registry pattern this reinforces

`firewall.RegisterTables("policy-routes", tables)` followed by
`firewall.ApplyAll()` is how a non-firewall plugin contributes nftables tables.
The registry merges every owner's tables, so no `Apply` deletes another owner's
work.

## Related

Table IDs and route metrics reaching netlink go through the bound described in
[netlink int field truncation](netlink-int-field-truncation.md).
