# Firewall IRR: Matching by ASN or AS-SET

Firewall rules match traffic by ASN or AS-SET using IRR-resolved prefix lists.
The BGP component already had an IRR filter plugin. The firewall needs the same
capability with different semantics: operator-controlled fetch with no auto
refresh by default, fail-closed commit verification, and nftables interval sets
instead of BGP prefix-list matching.

## Decisions

### The PrefixStore is shared with BGP

<!-- source: internal/component/firewall/plugins/irr/cache.go -- shared PrefixStore access -->

Both consumers read the same zefs keys, `meta/irr/{name}`, so a resolution and
its storage are not duplicated. The store already existed with those per-entry
keys, and the plan to add separate `meta/firewall/irr/{name}` keys was
unnecessary.

### A separate plugin, not an engine extension

<!-- source: internal/component/firewall/plugins/irr/irr.go -- plugin entry point -->

Plugin self-containment: removing the directory removes the feature.
`internal/component/firewall/plugins/` is now a plugin discovery path in codegen,
so a future firewall plugin goes there.

### Sets merge into the firewall's own table

<!-- source: internal/component/firewall/registry.go -- mergeSameNameTables -->

**nftables sets are TABLE-LOCAL.** A set registered on table A is invisible to a
chain in table B. IRR sets must therefore live in the same nftables table as the
chains that reference them, and the merge happens at `ApplyAll` time in
`mergeSameNameTables`.

The original spec design did not anticipate this. Any future firewall plugin that
needs to register sets on the engine's table uses `RegisterTables` with the same
table name and gets the merge.

### CIDR is encoded as an interval

<!-- source: internal/component/firewall/model.go -- SetElement.IntervalEnd -->
<!-- source: internal/component/firewall/plugins/irr/sets.go -- interval set generation -->

`SetElement.IntervalEnd` carries the CIDR-to-interval encoding, which is the
standard nftables pattern and scales to a large prefix list. Expanding a prefix
to individual addresses does not.

The config parser emits `MatchInSet` with deterministic names, reusing the
existing lowering path, so the backend needed no change beyond `IntervalEnd`.

### One address family per term

nftables cannot match both IPv4 and IPv6 addresses in a single rule expression
inside an `inet` table. `MatchInSet` is IPv4-only per term. IPv6 sets are created
and need separate terms. A future change could split terms and hide this.

### Auto-refresh is off by default

`refresh-interval` defaults to 0. Invalid IRR data causing a firewall outage is
the risk that makes auto-refresh opt-in.

## Trap

A YANG augment path must match the actual config tree:
`fw:table/fw:chain/fw:term/fw:from`, not `fw:filter/fw:term/fw:from`. The YANG
formatter catches the mistake.
