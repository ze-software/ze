# The IRR Filter Plugin

Ze had IRR whois and PeeringDB clients and no way to reach the BGP filter chain
with them. An operator ran bgpq4 outside ze, generated config and pasted it in.
`bgp-filter-irr` applies the AS-SET prefix lists the `irr` plugin fetched and
stored as live import filters. It fetches nothing itself: the `irr` plugin
(`docs/architecture/irr-owner.md`) queries the IRR server and PeeringDB on the
schedule `system irr` sets, stores each list, and tells the filter when one
changed.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- plugin entry point, handleConfigure, apply -->
<!-- source: internal/component/bgp/plugins/filter_irr/stored.go -- storeReader, the stored-list reader -->
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- config parsing from OnConfigure JSON -->
<!-- source: internal/component/bgp/plugins/filter_irr/match.go -- prefix matching -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- re-evaluating a route rejected for a list reason -->

## The decisions

**A new plugin, not an extension of `filter_prefix`.** Static config-driven
lists and dynamic external data are separate concerns, which is the same split
RPKI and RIB already make.

**The filter reads the `irr` plugin's store; it holds no IRR or PeeringDB
client.** Until 2026-11-15 the filter imported `resolve/irr` and fetched on its
own schedule, from `OnStarted`, every reload and a ticker. Thomas ruled that
data fetched from the internet must survive a restart unchanged and refresh only
at the frequency the configuration sets, with one fetcher for every consumer
(`plan/release/fixes/spec-fetched-data-survives-restart.md`). The filter now
names what it enforces in its interest key and reads the stored copies through
the shared reader in `internal/component/resolve/irr/store` (`consumer.go`).
It reaches the owner only through stored keys and notices, so it works the same
run internal or forked.

**About 30 lines of prefix matching are duplicated from `filter_prefix`.** Every
`filter_prefix` type is unexported, so sharing would mean exporting them or
extracting a common package, and that couples two independent plugins for a
small saving. Shared matching later means either exporting the types or an
extraction to `filter_common/`.

**Fail closed on an empty result or an error.** Accepting unvalidated routes is
worse than rejecting valid ones for a while. RPKI behaves the same way.

**An UPDATE is judged from its route fields, and only its announcements are
judged.** The filter declares the `prefix` and `withdrawn` reads, so each call
carries the NLRI the UPDATE announces and the NLRI it withdraws, each an
object from family to NLRI list in the UPDATE event's shapes
(`docs/plugin-development/protocol.md`, "The Route Fields of filter-update").
`judgeNLRI` judges every announced NLRI of a family `family.Family.PlainCIDR`
declares: IPv4 and IPv6 unicast, multicast and mpls-label. The NLRI of every
other family, a FlowSpec rule or an EVPN route, is not a prefix the list can
authorize, and passes unjudged. A withdrawal is never judged, because
withdrawing a route the list never authorized removes nothing it admitted. All
judged prefixes accepted, or none to judge, is accept. A mix is a modify whose
`sets` hold one prefix remove per rejected NLRI, naming it in the shape the
`prefix` field carried it; the engine drops exactly those entries from the
body NLRI and MP_REACH_NLRI, a labeled entry with its label field
(`policychain/external_intents.go`, `applyPrefixIntent`), and keeps the rest of
the UPDATE.

None accepted is a reject only when the UPDATE withdraws nothing. RFC 4271
Section 9 says the withdrawn routes "SHALL be removed from the Adj-RIB-In", and
the engine drops a rejected UPDATE whole, withdrawals included. So an UPDATE
that only withdraws is accepted, even for an ASN with no list yet, and one
that also announces is a modify removing every rejected prefix, which leaves
the withdrawals.

A `prefix` or `withdrawn` element that names no prefix rejects the whole
UPDATE and holds nothing. A labeled-unicast route under ADD-PATH cannot be
named by a remove yet: the event's labeled shape carries no `path-id`, so the
engine refuses the remove and the filter's `on-error` rejects the UPDATE.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- judgeListed, judgeUnlisted, keepWithdrawals -->
<!-- source: internal/component/bgp/plugins/filter_irr/match.go -- judgeNLRI, judgedFamily, nlriPrefix, rejectedRemoves -->
<!-- source: internal/core/family/cidr.go -- PlainCIDR -->

**The operator writes
`import { filters [ bgp-filter-irr:$remote_as ]; final accept; }`. Nothing is
auto-injected.** The reactor has no dynamic filter injection API, and
`$remote_as` already resolves through the existing filter variable mechanism,
which serves a dynamic group's members only: a static peer naming a variable is
refused at load (`docs/guide/bgp-peering.md`, dynamic groups).

**The filter has no IRR settings of its own.** Where and when a list is
fetched is the `irr` plugin's `system irr { server; source-address;
refresh-interval }` and `system peeringdb url`. `ze-filter-irr.yang` no longer
augments the policy root, and `parseIRRConfig` reads the bgp root alone: a peer
names only its `irr { as-set; enable }` and its `bgp-filter-irr:<asn>` import
filter.

**A commit reaches the filter through the reload pair, not through
`OnConfigure`.** `OnConfigure` fires once, at boot. A live commit is delivered
to `OnConfigVerify`, which holds the candidate, then to `OnConfigApply`, which
makes it the filter's config through the same `handleConfigure` the boot path
uses; `OnConfigRollback` puts the replaced config back. An apply with no
verified candidate is an error. With `OnConfigure` alone the SDK answered every
apply OK and called nothing, so a changed as-set was accepted and ignored.
`test/reload/bgp-filter-irr-added-live.ci` proves the edit is enforced. The
transaction is the SDK's `ConfigTx`, which keeps the verified candidate and
the replaced config; the filter parses both the `bgp` and `policy` roots and
uses `handleConfigure` as the install. Its Stage 1 `WantsConfig` requests both:
`bgp` supplies peers and AS-SETs, while `policy` still carries the unread IRR
leaves until the config conversion removes them. An external program has no compiled-in registry row, so it
cannot rely on that row's `ConfigReads` to receive `policy`. The same settings
therefore reach the internal and forked evaluator.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- runFilterIRR -->
<!-- source: internal/component/bgp/plugins/filter_irr/config_tx.go -- newIRRConfigTx -->
<!-- source: pkg/plugin/sdk/config_tx.go -- ConfigTx -->

**A changed as-set starts from no list.** `handleConfigure` carries a list over
a reconfigure only while the ASN's configured as-set is the same, including
both empty (`carriedOverBy`). The reload publishes a changed ASN with no list,
so an UPDATE after the edit is rejected and held until the reader has read the
new set's stored copy, never judged against the old one. A configured as-set
is kept in the one spelling every store key uses (`store.ASSetName`, upper
case). With no configured set, the set is the one the `irr` plugin's binding
for the AS number names (`meta/irr-asn/{asn}`), which PeeringDB chose.

**The prefix-lists survive a restart.** This reverses the owner decision of
2026-10-17 that kept the lists in memory only. Its recorded reason was a state
write that took the SDK's full 5 s under disk load and expired the
first-resolution wait the filter then had
(`plan/journal/startup-wait-expires-under-load.md`); that wait was replaced by
reject, hold and re-admit, and the filter writes no list at all now. Before and
after, as an operator sees it:

| Situation | Before (until 2026-11-15) | After |
|-----------|---------------------------|-------|
| Router reboots, IRR server reachable | `show bgp irr` shows every ASN pending; announcements are rejected and held until each whois answers; the lists can differ from the ones before the reboot | `show bgp irr` shows the same AS-SETs, counts, generation and `refreshed-at` as before; no query is sent at start; announcements are judged against the stored lists at once |
| Router reboots, IRR server unreachable | Every IRR-filtered peer's announcements stay rejected until the server answers | The stored lists are enforced |
| Reload that changes nothing IRR | Every ASN is resolved again | No query; the pointers are read, and a dataset only when its pointer moved |

**How the filter reads.** One worker, `storeReader.run`, started by
`OnStarted`, does every store read and write, never the configure path and
never under `plug.mu`:

1. `OnStarted` asks it for a full pass. The pass reads the generation counter
   (`meta/irr-generation`), then for each enrolled ASN the binding (only when
   no as-set is configured), the pointer (`meta/irr-set/{as-set}`) and, when
   the pointer names a copy the ASN does not already enforce, the dataset it
   names. What it found is installed and published in one swap under
   `plug.mu` (`apply`, then `publishLocked`), and the readmitter is woken.
2. The pass then writes the interest key `meta/irr-interest/bgp-filter-irr`
   (the configured sets, and the AS numbers whose set PeeringDB chooses) and
   emits `irr/interest-changed`, when the interest changed. A refused write is
   retried every minute and reported (`doctor-bgp-irr-interest-unsaved`). The
   write comes after the publish, so a slow store never delays a list.
3. Each `irr/dataset-stored` notice, subscribed at stage 5 so none is lost
   before the first read, queues its AS-SET and generation. The next pass reads
   the ASNs on that set and every ASN whose set PeeringDB chooses, because a
   binding that moved is announced only by the new set's notice. A notice whose
   generation skips one the reader has not seen means a notice was lost: the
   pass reads every pointer again.
4. A reload after the start asks for a full pass, so a newly named set is read
   at once if stored, and the interest notice makes the `irr` plugin fetch it
   if not.

A read answers one of three ways, and `apply` treats each differently: nothing
stored (`store.ErrNotStored`: not fetched yet, or `clear irr` removed it) drops
the list, so the peer's announcements are rejected and held; a stored value
that cannot be used (a failed digest or decode, a store error, a list over the
500,000-prefix cap) keeps the list enforced before, logs a WARN and reports
`doctor-bgp-irr-stored-copy-unreadable`; a copy read is installed.

An UPDATE that arrives before an ASN has a list is judged at once, with no
wait: its announcements take the filter's on-error verdict, reject, its
withdrawals still apply (`keepWithdrawals`), and the announced prefixes are
held. The re-admission path below asks the peer for them by ROUTE-REFRESH once
the list is published. A configure that runs before `OnStarted`, inside the
startup handshake, where no state RPC is served, asks the reader for nothing.

**What the filter reports.** A runtime doctor check, `bgp-filter-irr-stored-copies`,
answered by `answerDoctorCheck` in the filter's own process, reports
`doctor-bgp-irr-stored-copy-unreadable` per ASN whose stored copy could not be
used, `doctor-bgp-irr-interest-unsaved` while the interest key is unsaved, and
`doctor-bgp-irr-stored-copy-behind` when the generation the `irr` plugin last
raised is above the one the reader applied. The refresh metrics
(`ze_irr_refresh_outcomes_total`, `ze_irr_data_age_seconds`) are the `irr`
plugin's; the filter registers none.
<!-- source: internal/component/bgp/plugins/filter_irr/doctor.go -- storedCopyCheckDecl, answerDoctorCheck -->

**The evaluator reads a snapshot.** The plugin registers one evaluator with
`sdk.OnFilter` (`evaluate`), called through the typed bridge slot when the
plugin runs internal and once per route of a `filter-update` batch when it
runs forked, concurrently on the engine's goroutines. It takes no lock and
never waits: it reads `irrSnapshot`, one immutable value holding every
enrolled ASN's list (nil while nothing is stored) and whether its last read
could not use a stored value, swapped whole through an atomic pointer. `publishLocked` builds it
under `plug.mu` wherever `byASN` or a state's list changes: the end of
`handleConfigure` and the reader's `apply`. A
route is therefore judged wholly by one config and one set of lists, and
`show bgp irr` and the re-admission worker keep reading `byASN` under the
lock. The evaluator reads only the `prefix` and `withdrawn` fields it
declares, through the library chain every BGP chain runs as.

**Its filter names are declared.** At Stage 1 the plugin declares one filter
name pattern, `asnFilterNamePattern`, covering a decimal AS number from 1 to
4294967295, on import, failing closed. The config load judges every
`bgp-filter-irr:<asn>` reference against it, so a reference whose suffix is
not an AS number fails the load instead of rejecting every route at run time;
`extractASNFromFilter` still reads the number from the name of each call.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- irrSnapshot, publishLocked, evaluate, handleFilterUpdate, filterDecls, asnFilterNamePattern, handleConfigure, apply -->

A resolved prefix list over the entry cap is refused whole, never cut: the
refresh fails with an error naming the IPv4 and IPv6 counts and the cap, and
the previous list stays in force. A cut filled the budget with IPv4 first, so
every IPv6 route was rejected while the refresh reported success.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- prefixListFromIRR -->
<!-- source: pkg/plugin/rpc/types.go -- FilterUpdateTimeout -->

**A route rejected for a list reason is asked for again, by ROUTE-REFRESH.** The
owner asked for it on 2026-10-17. No RFC obliges it: RFC 2918 Section 1 says
only that "When the inbound routing policy for a peer changes, all prefixes from
that peer must be somehow made available and then re-examined against the new
policy", in an introduction. `handleFilterUpdate` hands every verdict to the
`readmitter`, block by block: a rejected prefix, whether the ASN had no list or
the list did not hold it, is held under (peer, family, prefix); an accepted or
withdrawn one is dropped. An UPDATE rejected whole holds every prefix it
announced, the ones the list admits too, because the engine dropped them all.
Only an ASN the config enrolls holds anything, since no list of any other
ASN is ever read. The held count is an atomic beside the map, so an UPDATE with
nothing to hold and nothing held takes no lock. The reader's `apply` wakes the
worker whenever it publishes a changed list, and the worker sends `request peer <peer>
refresh <family>` for each peer and family with a held prefix the new list
admits, then drops those prefixes: the peer sends them again and the filter
holds again any it still rejects.

A whole reject is the one verdict that can make that a loop. The admitted
prefixes it holds are due on every pass, the peer resends the same UPDATE, and
the filter rejects it whole again, once a minute (`readmitInterval`) for
as long as the peer keeps announcing it. A refused prefix in an MP_REACH_NLRI
of a CIDR family no longer causes one: the UPDATE is a modify, so only the refused prefix is held, and a
pass asks for nothing until the list admits it. The case that remains is a
refused prefix in a block the engine cannot rewrite, a FlowSpec destination for
one, beside a prefix the list admits.

Replaying from Adj-RIB-In was the alternative that puts nothing on the wire, and
it does not exist: the import chain runs before `bgp-adj-rib-in`, which stores
only the routes the chain accepted, so the rejected route is held by nobody.
ROUTE-REFRESH is the only path. The worker first reads `show bgp peer <peer>
detail`: a peer that is not established, or that the answer no longer names,
has its held prefixes dropped, because a session that comes back sends
everything again. Route Refresh must be negotiated with the peer: ze
advertises it only with `session { capability { route-refresh enable } }`
(`reactor/config_capabilities.go`), and the peer must advertise it too. A peer
without it, or whose refresh the engine refuses with `errSendNotPermitted`
because its binding grants neither `send [ refresh ]` nor `send [ * ]`, gets
one warning naming the fix, and its admitted prefixes are dropped rather than
asked for forever. Any other refresh failure, a timeout for one, holds the
prefixes again for the next pass, except one an UPDATE judged while the
request was in flight. The reactor's refresh command skips a peer without the
capability in silence, which is why the worker asks first, and it answers
success for a peer whose session went down meanwhile, which is why the worker
reads the session again: down or absent drops what the peer holds, and a read
that fails drops nothing. The admitted prefixes are dropped before the request
rather than after, so a prefix the peer's reply is rejected for again stays
held.

Configure warns once for each enrolled peer whose binding lacks the grant, and
once for each enrolled peer to which ze does not advertise Route Refresh. The
peer's binding decides the grant when it states a `send` list, and its group's
otherwise, because `send` is not cumulative across levels
(`bgp/config/resolve.go`); the peer's `session > capability > route-refresh`
decides the capability the same way (`configjson.GetCapability`).

The worker is one goroutine for the plugin's life, started in `OnStarted` and
stopped when the plugin returns. Its wake channel has depth one, so a burst of
publishes is one pass. Its timer, `readmitInterval` (one minute), drops what a
peer that went down holds and asks again for what the published lists admit.
It fetches nothing: a failed fetch is retried by the `irr` plugin's schedule,
and the list it then stores reaches the worker through the reader's publish. The held
set is capped at `heldPrefixMax`; a prefix refused by the cap is evaluated again
only when its peer sends it again. A reconfigure drops what a peer holds once
the config no longer filters that peer under that ASN, keyed on the peer's
address (`connection > remote > ip`), even when the ASN stays enrolled through
another peer. `test/plugin/filter-irr-readmit.ci` proves the path end to end.
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- grantsRefresh, advertisesRouteRefresh -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- readmitter, run, pass, readmit -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit_engine.go -- dispatchEngine -->

Removing the `filter_irr/` directory removes every IRR filter feature. The
plugin self-containment test proves it.

## Constraints

**The per-peer `irr { as-set }` YANG augment must cover three paths**: a
standalone peer, a grouped peer, and the group-level session.

**`handleConfigure` and `apply` write every mutable plugin field under
`plug.mu`**: `byASN`, `config` and each state's list, and `started` is read and
written under the same lock. The reader captures what it needs of each ASN
under a read lock (`readTargets`), reads with no lock held, and `apply`
re-reads `byASN[asn]` under the write lock: a state a reload replaced with
another configured set is left alone, because the reload asked for its own
read.

**The filter writes one key, its own.** It holds a read-only grant on the
datasets, pointers, bindings and generation, and a write grant on
`meta/irr-interest/bgp-filter-irr` alone (`register.go`,
`statestore.RegisterPluginKeyInstance`). See `docs/architecture/irr-owner.md`
for the keys and `docs/architecture/resolve.md` for the store the owner runs.

**A test feeds the route fields, never the text form.** The evaluator reads
only `prefix` and `withdrawn`, so an input carrying the old `update` text and no
fields judges an UPDATE that announces nothing, and its assertion passes
whatever the list holds. The helpers in `route_fields_test.go` write the fields
in the shape the engine sends.
