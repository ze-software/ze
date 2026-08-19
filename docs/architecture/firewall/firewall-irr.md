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

### A term states the type of the set another owner provides

<!-- source: internal/component/firewall/config.go -- irrSetMatch -->
<!-- source: internal/component/firewall/validate.go -- validateMatch -->

Verify-time validation runs over the parsed configuration alone, long before the
merge, so the IRR set is not there to be found. `MatchInSet.ProvidedType` carries
the element type the supplying owner will register, and the config parser is the
only thing that sets it, for the four IRR leaves alone. `validateMatch` accepts a
match that carries it and checks the field against that type; a match without it
still has to name a set the table itself declares, so `source-address
"@irr_v4_typo"` typed by hand is refused as it always was.

### One table waits for a set nobody has registered yet

<!-- source: internal/component/firewall/registry.go -- dropTablesMissingAProvidedSet -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- refreshName -->

The owners register in no fixed order. At startup the firewall engine configures
before the plugin that supplies the set, because the plugin depends on it, and in
a reload transaction the participants apply in whatever order the orchestrator
emits. The backend refuses a rule whose set is not on the table, and that refusal
fails the reconcile for every owner.

`ApplyAll` therefore leaves out each merged table whose term names a provided set
nobody declared, programs the rest, and logs `table held back` at WARN with the
table and the set. The supplying owner calls `ApplyAll` when it registers, and
the table is programmed then.

The unit is one table, and it is the smallest unit that can wait: a set is
table-local, so no other table's terms can depend on the missing one. Holding
back the whole reconcile instead made one absent supplier the whole firewall's
problem. On a cold prefix cache, which is a fresh install, a wiped
`database.zefs`, or a cache file that cannot be read, the plugin registers no set
at all and no supplier is on the way, so the operator's tables, copp, the DDoS
tables and the policy routes all stayed out of the kernel behind one WARN.

A table left out is not programmed, so the traffic it filters is not filtered.
That is the choice `buildIfaceTables` already makes for a binding with no
prefixes: a rule set with no data behind it is not a filter, and an unfiltered
port beats a blackholed one.

`update firewall irr asn|as-set` ends the wait, because `refreshName` applies the
tables it built from what it fetched. Nothing else does on a cold cache:
`refresh-interval` defaults to 0.

### A reload reconfigures the plugin

<!-- source: internal/component/firewall/plugins/irr/irr.go -- configure -->

`OnConfigApply` receives a diff, not the configuration, so the plugin carries the
config `OnConfigVerify` approved into it and calls the same `configure` that
`OnConfigure` calls. Anything else leaves the plugin serving what the daemon
started with: a term added by a commit would never reach the registry, and the
firewall would wait for a set the plugin was never told to build.

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

### An empty answer is not data

<!-- source: internal/component/resolve/irr/client.go -- parseReply, the RPSL status line classifier -->
<!-- source: internal/component/resolve/irr/store/store.go -- Refresh, markStale, Purge -->

An RPSL reply ends with one status line. `C` means the query succeeded, `D`
means the server does not hold the key, `E` means the database holds several
copies of it, and `F <message>` means the server failed the query. `parseReply`
reads that line. Before it existed the parser skipped every word it could not
read as a prefix, so all four replies reduced to an empty prefix list with no
error, and a server error was indistinguishable from an answer of nothing.

`E`, `F` and a reply with no status line are errors. A reply cut short by the
4 MB read cap has no status line, so a partial record set is an error rather
than a shorter prefix list.

`Refresh` never replaces cached prefixes with none. A lookup that errors and a
lookup that succeeds with no prefixes both keep what is cached and set
`StaleSince` on the entry, and the second returns `store.ErrNoPrefixes`. The
guard lives in the store, so the firewall table terms, the firewall interface
bindings and the BGP IRR filter all get it from one place.

It holds per family. IPv4 and IPv6 are two queries, and an AS-SET with no IPv6
route objects answers the IPv6 one with `D`, exactly as a server having a bad
minute does. `commit` keeps the cached prefixes of a family that answered
nothing, and marks the entry stale. Replacing the entry wholesale cost the
interface binding its IPv6 accept term while the drop term that closes the
whitelist stayed, which drops every IPv6 packet arriving on the port.

`LookupPrefixes` refuses to cache an empty answer, so an empty one is never
served for an hour. An answer that carried prefixes IS cached for an hour, and
an operator forcing a refresh inside that hour is served from that cache rather
than from the server.

Removing prefixes is an operator action: `clear firewall irr asn|as-set` calls
`PrefixStore.Purge`. Without it, an AS-SET deregistered upstream would be
enforced forever.

### An interface binding with no prefixes produces no table

<!-- source: internal/component/firewall/plugins/irr/sets.go -- buildIfaceTables -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- verifyRefs -->

The interface ingress chain is a whitelist: accept terms for the AS-SET's
prefixes, then one drop term closing it. Emitted on its own that drop term
drops every packet arriving on the interface, and the apply succeeds, so a
customer-facing port goes dark with no error anywhere. `buildIfaceTables`
therefore emits nothing for a binding with no prefixes, and logs it. An
unfiltered port beats a blackholed one, and `verifyRefs` refuses the commit
before it gets that far.

## Trap

A YANG augment path must match the actual config tree:
`fw:table/fw:chain/fw:term/fw:from`, not `fw:filter/fw:term/fw:from`. The YANG
formatter catches the mistake.
