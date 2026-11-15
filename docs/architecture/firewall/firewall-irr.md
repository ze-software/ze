# Firewall IRR: Matching by ASN or AS-SET

Firewall rules match traffic by ASN or AS-SET using IRR-resolved prefix lists.
The BGP component already had an IRR filter plugin. The firewall needs the same
lists with different semantics: fail-closed commit verification and nftables
interval sets instead of BGP prefix-list matching.

## Decisions

### The irr plugin fetches and stores, this plugin reads

<!-- source: internal/component/firewall/plugins/irr/stored.go -- storeReader, pass, passNow, readTarget -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- runFirewallIRR, handshakeConfigure, configure -->

This plugin fetches nothing and holds no IRR or PeeringDB client. The `irr`
plugin fetches, schedules and stores every list, and the BGP IRR filter
enforces the same stored copy (`docs/architecture/irr-owner.md`). The plugin
writes its interest key, `meta/irr-interest/firewall-irr` (every AS-SET and AS
number its terms and interface bindings name), and reads through the shared
reader: for an ASN reference the binding, then the set's pointer, then the
dataset the pointer names ("What a consumer reads").

The reads run on one worker, `storeReader`. The startup handshake delivers the
config before any state RPC works, so `handshakeConfigure` records it and does
nothing else: no table is registered and no reference is reported unstored
there. Doing both in the handshake programmed every set from an empty cache and
warned that every reference held no data, at every start, a moment before the
`OnStarted` read programmed them again. The first pass after `OnStarted` reads
every reference, programs the tables once, and writes the interest key. A
reference with nothing stored is logged at INFO: on a cold start the `irr`
plugin fetches it as soon as the interest key names it, and its notice programs
it.

A notice `irr/dataset-stored` queues the set it names; the pass reads the
references to that set, and every ASN reference, because a binding that moved
is announced only by the notice of the set it moved to. A notice whose
generation skips one the plugin has not applied means a notice was lost, so the
pass reads every reference again. A pointer equal to the one the enforced copy
was read under needs no dataset read. `ErrNotStored` drops the list (the set was
never fetched, or `clear irr` removed it), and any other read failure keeps the
list enforced and logs a WARN under `doctor-firewall-irr-stored-copy-unreadable`.

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
<!-- source: internal/component/firewall/plugins/irr/stored.go -- pass -->

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
store, or an unreadable cache entry, the plugin registers no set
at all and no supplier is on the way, so the operator's tables, copp, the DDoS
tables and the policy routes all stayed out of the kernel behind one WARN.

A table left out is not programmed, so the traffic it filters is not filtered.
That is the choice `buildIfaceTables` already makes for a binding with no
prefixes: a rule set with no data behind it is not a filter, and an unfiltered
port beats a blackholed one.

The `irr` plugin's notice ends the wait: the pass that reads the stored set
applies the tables it built. On a cold start the `irr` plugin fetches every set
the interest key names at once; `update irr asn|as-set` fetches one by hand.

### A table term declares both families or neither

<!-- source: internal/component/firewall/plugins/irr/sets.go -- buildTermSets -->
<!-- source: internal/component/firewall/config.go -- expandProvidedTermV6 -->

The parser cannot see the prefix data. It emits an IPv6 twin of every IRR term,
whatever the entry announces. An ASN or AS-SET announcing IPv4 and no IPv6 is
ordinary. So the plugin declares BOTH set names for any entry it has data for.
The family that announced nothing gets a set with no elements. That set matches
no address, which is what the twin must read.

Declaring one family instead cost the operator the whole table. The twin named a
set no owner declared. `dropTablesMissingAProvidedSet` held the table back, and
the commit reported success with nothing in the kernel.

An entry announcing nothing at all still yields no set. A cold cache therefore
keeps holding the table back until prefixes arrive. `buildIfaceTables` keeps
`buildSets`, which answers per family. It emits an accept term per family under
the same condition, so it never names a set it did not declare.

### A reload reconfigures the plugin

<!-- source: internal/component/firewall/plugins/irr/irr.go -- configure, verify, verifyStored -->
<!-- source: internal/component/firewall/plugins/irr/stored.go -- passNow -->

`OnConfigVerify` reads the `irr` plugin's store and nothing else, so a commit
sends no IRR query. It refuses a reference with nothing stored, a stored value
it cannot read, a stored list with no prefixes, and a list too long for a set,
each naming the reference, and the first two naming `update irr as-set <name>`
or `update irr asn <n>` (Thomas, design gate: the refusal stays, verify does no
network I/O).

`OnConfigApply` receives a diff, not the configuration, so the plugin carries the
config `OnConfigVerify` approved into it, installs it with `configure`, and runs
one pass on the commit's own goroutine (`passNow`): it reads every reference,
applies the tables even when no copy changed (a commit can move a reference to
another table or add an interface binding), and answers the apply's error, so
the commit reports whether its tables reached the firewall.

A reference whose name the previous config also carried keeps its enforced copy
across the reload, and its pointer unchanged costs no dataset read. A
configure that rebuilt the lists empty handed `applyTables` nothing: the sets
were registered with no prefixes, `ApplyAll` held back the operator's table for
naming a set no owner supplied, and the commit that caused it reported success.

### A crash starts the plugin again

<!-- source: internal/component/firewall/plugins/irr/irr.go -- runFirewallIRR -->
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput.FailurePolicy -->
<!-- source: internal/component/plugin/server/failure_policy.go -- (*Server).applyFailurePolicy -->

The plugin declares `failure-policy: restart`, so a process that ends is started
again. Without that declaration a crash was permanent for the life of the
daemon, and the registry then held back every table naming an IRR set.

The restart costs nothing the plugin has to rebuild by hand: the lists live in
the `irr` plugin's stored copies, so the replacement reads them at `OnStarted`
and programs its sets from what was already fetched. It is bounded at 5 restarts in
60 seconds and 20 over the life of the daemon, which is what stops an answer
that panics every time from cycling: past the bound the plugin is disabled and
`show health` reports doctor-plugin-stopped.

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

### The refresh period is the irr plugin's

A firewall IRR set refreshes every 3600 seconds by default, the same dataset
BGP refreshes, at `system irr refresh-interval`; `system irr refresh-interval 0`
makes every consumer's refresh manual (Thomas, Q-6). The `firewall irr server`,
`peeringdb-url` and `refresh-interval` leaves are no longer read. A refresh
cannot empty a firewall set: an answer with nothing for a family keeps that
family's stored prefixes (below), and a list too long for a set is refused.

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

These rules run in the `irr` plugin, which holds the only IRR and PeeringDB
clients; `clear irr cache` and `clear peeringdb cache` reach those clients and
leave every stored list in place (`docs/architecture/irr-owner.md`, "Commands").
`show firewall irr` reads no server: it answers the copy this plugin enforces.

Removing prefixes is an operator action: `clear irr asn|as-set` removes the
stored copy, and this plugin drops the list on the notice. Without it, an
AS-SET deregistered upstream would be enforced forever.

### A list too long for a set is refused, and each family is bounded on its own

<!-- source: internal/component/firewall/plugins/irr/sets.go -- maxPrefixesPerFamily, oversizedEntryError -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- applyTables, verifyStored -->

One firewall set holds 500000 prefixes. The bound used to TRUNCATE: the builder
stopped there, logged one WARN and returned the entries that fit, and
the refresh read that short list as a refresh that worked
and programmed it. Nothing downstream could see it. `Set.Validate`
(`internal/component/firewall/model.go`) checks the name and the type, and the
nftables backend loops over `Elements` whatever it holds.

A firewall set is read in two directions and the builder can see neither, so a
partial set has no safe reading:

| The term says | A truncated set | An empty set |
|---------------|-----------------|--------------|
| `accept` from this set | drops the traffic the missing prefixes carry | drops every packet of that family the term guards |
| `drop` on this set | forwards the traffic the missing prefixes carry | blocks nothing |

So the list that does not fit is an error. `oversizedEntryError` answers it,
`verifyStored` refuses the commit with it, and `applyTables` refuses the apply,
which leaves the sets already registered exactly as they are. This is the answer
the IRR client already gives its own read cap, where a reply cut short by the
4 MB limit is an error rather than a shorter prefix list.

One oversized reference stops the whole apply rather than its own table. Leaving
that entry's sets out instead makes `dropTablesMissingAProvidedSet` hold back
its whole table, which takes a filter that was working out of the kernel.
Registering nothing keeps every table as it is, and the operator is told:
the apply logs it at WARN, `show firewall irr` reports the entry
as `oversized`, and `ze doctor` reports
`doctor-firewall-irr-oversized-data`.

The bound is PER FAMILY, because one family is one set in the kernel. The two
families shared one budget until 2026-09-20: IPv4 took the whole 500000 and IPv6
got what was left. A large IPv4 list therefore left the IPv6 set EMPTY rather
than short, which matched no address, and the interface whitelist's drop term
then took every IPv6 packet arriving on the port. That is the same outage the
per-family guard in the irr plugin's `PrefixStore.Refresh` exists to prevent, reached by another
route. A budget one list can spend on behalf of the other is not a budget.

### An interface binding with no prefixes produces no table

<!-- source: internal/component/firewall/plugins/irr/sets.go -- buildIfaceTables -->
<!-- source: internal/component/firewall/plugins/irr/irr.go -- verifyStored -->

The interface ingress chain is a whitelist: accept terms for the AS-SET's
prefixes, then one drop term closing it. Emitted on its own that drop term
drops every packet arriving on the interface, and the apply succeeds, so a
customer-facing port goes dark with no error anywhere. `buildIfaceTables`
therefore emits nothing for a binding with no prefixes, and logs it. An
unfiltered port beats a blackholed one, and `verifyStored` refuses the commit
before it gets that far.

## Trap

A YANG augment path must match the actual config tree:
`fw:table/fw:chain/fw:term/fw:from`, not `fw:filter/fw:term/fw:from`. The YANG
formatter catches the mistake.
