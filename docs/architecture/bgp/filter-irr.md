# The IRR Filter Plugin

Ze had IRR whois and PeeringDB clients and no way to reach the BGP filter chain
with them. An operator ran bgpq4 outside ze, generated config and pasted it in.
`bgp-filter-irr` queries IRR for AS-SET prefixes, applies them as live import
filters, and refreshes on its own.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- plugin entry point, refreshASN -->
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- config parsing from OnConfigure JSON -->
<!-- source: internal/component/bgp/plugins/filter_irr/match.go -- prefix matching -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- re-evaluating a route rejected for a list reason -->

## The decisions

**A new plugin, not an extension of `filter_prefix`.** Static config-driven
lists and dynamic external data are separate concerns, which is the same split
RPKI and RIB already make.

**`resolve/irr` is imported directly, not reached over RPC.** BGP plugins run in
process, so RPC adds latency and isolates nothing. There is no import cycle:
`irr` imports only cache, textbuf and the standard library.

**About 30 lines of prefix matching are duplicated from `filter_prefix`.** Every
`filter_prefix` type is unexported, so sharing would mean exporting them or
extracting a common package, and that couples two independent plugins for a
small saving. Shared matching later means either exporting the types or an
extraction to `filter_common/`.

**Fail closed on an empty result or an error.** Accepting unvalidated routes is
worse than rejecting valid ones for a while. RPKI behaves the same way.

**An UPDATE is judged block by block, and only its announcements are judged.**
The reactor renders the legacy NLRI and Withdrawn Routes sections and each
MP_REACH_NLRI and MP_UNREACH_NLRI attribute as a block of its own, `nlri
add|del <prefix>...` for the legacy sections and `mp <family> add|del
<prefix>...` for the attributes (`reactor/filter_format.go`), so one UPDATE can
carry several. `partitionUpdate` splits the field at each `nlri` and `mp`
keyword and classifies the prefixes of every `add` block; a `del` block is
parsed but never judged, because withdrawing a route the list never authorized removes nothing
it admitted. All announced prefixes accepted is accept, and a mix is a modify
whose delta rebuilds every block in order: an announce block keeps its
accepted prefixes and is left out when the list admits none of them, and a
withdraw block is kept whole. A block with no prefix at all, the `mp <family>
add` marker the reactor renders for a non-CIDR family such as `l2vpn/evpn`, is
echoed unchanged: the engine reads a block left out as a changed one, refuses a
changed block of a non-CIDR family, and drops the UPDATE whole, legitimate
legacy prefixes included.

None accepted is a reject only when the UPDATE withdraws nothing. RFC 4271
Section 9 says the withdrawn routes "SHALL be removed from the Adj-RIB-In", and
the engine drops a rejected UPDATE whole, withdrawals included. So an UPDATE
that only withdraws is accepted, even for an ASN with no list yet, and one
that also announces is a modify whose delta holds its withdraw blocks alone.

A prefix that does not parse, in any block, rejects the whole UPDATE. So does a
rejected prefix in an announce block the engine cannot rewrite
(`partitionResult.rewritable`). The engine rewrites the legacy `nlri add` block
and the `mp <family> add` block of a CIDR family, which `filtertext.CIDRFamily`
declares once for the engine and the filters: IPv4 and IPv6 unicast, multicast
and mpls-label, whose label stacks it keeps (`reactor/filter_nlri_delta.go`,
`sectionNLRIDelta`). Any other family, a FlowSpec destination for one, or a
family name this process does not know, has no modify that drops the prefix,
so the plugin fails closed. The withdrawals are lost with the rejected UPDATE.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- judgeListed, judgeUnlisted, keepWithdrawals -->
<!-- source: internal/component/bgp/plugins/filter_irr/match.go -- partitionUpdate, rewritable, buildModifyDelta -->
<!-- source: internal/component/bgp/filtertext/cidr.go -- CIDRFamily -->

**The operator writes `import bgp-filter-irr:$remote_as`. Nothing is
auto-injected.** The reactor has no dynamic filter injection API, and
`$remote_as` already resolves through the existing filter variable mechanism.

**`refresh-interval` is `uint32`.** The YANG range is 60 to 86400, which exceeds
the `uint16` maximum of 65535.

**A commit reaches the filter through the reload pair, not through
`OnConfigure`.** `OnConfigure` fires once, at boot. A live commit is delivered
to `OnConfigVerify`, which holds the candidate, then to `OnConfigApply`, which
makes it the filter's config through the same `handleConfigure` the boot path
uses; `OnConfigRollback` puts the replaced config back. An apply with no
verified candidate is an error. With `OnConfigure` alone the SDK answered every
apply OK and called nothing, so a changed as-set was accepted and ignored.
`test/reload/bgp-filter-irr-added-live.ci` proves the edit is enforced. The
transaction is the SDK's `ConfigTx`, which keeps the verified candidate and
the replaced config; the filter supplies only the parse of the bgp subtree and
`handleConfigure` as the install.
<!-- source: internal/component/bgp/plugins/filter_irr/config_tx.go -- newIRRConfigTx -->
<!-- source: pkg/plugin/sdk/config_tx.go -- ConfigTx -->

**A changed as-set starts from no list.** `handleConfigure` carries a resolved
list over a reconfigure only while the ASN's as-set is the same, and
`forgetUnconfigured` purges the store's entry for it, so the store holds only
the sets the config names (`store.commit` already refuses to merge another
AS-SET's family into an answer). The first UPDATE after the edit waits, bounded,
for the new set's resolution rather than being judged against the old one.
The same pass purges the entry of an ASN the new config no longer enrolls.

**Deleting the configured as-set is a change.** `asnState` keeps two fields:
`configured`, the as-set the operator wrote and the only query hint, and
`asSet`, the set the ASN last resolved under, which `show bgp irr` reports.
With no configured set every refresh passes an empty hint, so the store asks
PeeringDB each time and keeps its cached set during an outage: neither the
first answer nor the `AS<asn>` literal an outage produced is pinned. A reload
carries a state over only while `configured` is the same, including both
empty. It cancels every state it does not carry over, so a lookup still in
flight for it commits nothing (`store.commit`), and `forgetUnconfigured`
purges a deleted or changed set's store entry under `plug.mu`, before the new
states are published. Without both, `store.discoverASSet`, which keeps a
cached entry's AS-SET when PeeringDB fails, would hand the deleted set back.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- asnState, handleConfigure, forgetUnconfigured -->

**The prefix-lists live in memory only.** The owner decided it on 2026-10-17.
The plugin builds its `PrefixStore` with no persistence, never asks the SDK for
its state store, and writes nothing to disk. The disk copy it used to keep cost
more than it gave: the daemon state write took the SDK's full 5 s under disk
load, which made an UPDATE's first-resolution wait expire
(`plan/journal/startup-wait-expires-under-load.md`). The store is created by the
first configure and kept for the plugin's life; a later configure points it at
the configured servers with `UseClients`, so a reload loses no resolved entry
and the per-family last-known-good still has the entry to merge into.

A restart therefore starts from no list. `OnStarted` sets `started` and runs
`initialResolve`, which resolves every enrolled ASN from the IRR server. An
UPDATE that arrives before an ASN's first resolution waits for it, for at most
`firstResolveWait`. That bound sits one second below `rpc.FilterUpdateTimeout`,
the reactor's deadline for a filter-update call, so the verdict given when it
expires still reaches the reactor. An UPDATE still without a list then has its
announcements rejected and held, and the re-admission path below asks the peer
for them by ROUTE-REFRESH once the list arrives. An IRR server unreachable at
startup has no cached list to fall back to: the announcements stay rejected,
and the worker retries the lookup every `readmitRetryInterval`.

A configure that runs before `OnStarted`, inside the startup handshake, starts
no resolution, because startup must not depend on the IRR server; only a
reload configure, after `started` is set, starts it itself.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- firstResolveWait, handleConfigure, forgetUnconfigured, refreshASNCtx -->
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
Only an ASN the config enrolls holds anything, since no refresh of any other
ASN ever runs. The held count is an atomic beside the map, so an UPDATE with
nothing to hold and nothing held takes no lock. `refreshASNCtx` wakes the
worker whenever it publishes a list, and the worker sends `request peer <peer>
refresh <family>` for each peer and family with a held prefix the new list
admits, then drops those prefixes: the peer sends them again and the filter
holds again any it still rejects.

A whole reject is the one verdict that can make that a loop. The admitted
prefixes it holds are due on every pass, the peer resends the same UPDATE, and
the filter rejects it whole again, once a minute (`readmitRetryInterval`) for
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
advertises it only with `capability { route-refresh enable }`
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
(`bgp/config/resolve.go`); the peer's `capability > route-refresh` decides the
capability the same way.

The worker is one goroutine for the plugin's life, started in `OnStarted` and
stopped by `stopCh`. Its wake channel has depth one, so a burst of refreshes is
one pass. Its timer, `readmitRetryInterval` (one minute, the shortest
`refresh-interval`), retries the lookup of every ASN that holds a prefix and has
no list or a failed one, and drops what a peer that went down holds. The held
set is capped at `heldPrefixMax`; a prefix refused by the cap is evaluated again
only when its peer sends it again. A reconfigure drops what a peer holds once
the config no longer filters that peer under that ASN, keyed on the peer's
address (`connection > remote > ip`), even when the ASN stays enrolled through
another peer. `test/plugin/filter-irr-readmit.ci` proves the path end to end.
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- grantsRefresh, advertisesRouteRefresh -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- readmitter, run, pass, readmit -->
<!-- source: internal/component/bgp/plugins/filter_irr/readmit_engine.go -- dispatchEngine -->

**Each configure cycle gets its own `refreshStop` channel.** `handleConfigure`
runs at boot and on every applied or rolled-back reload, and without a stop
channel per cycle the refresh goroutines accumulate.

Removing the `filter_irr/` directory removes every IRR filter feature. The
plugin self-containment test proves it.

## Constraints

**The per-peer `irr { as-set }` YANG augment must cover three paths**: a
standalone peer, a grouped peer, and the group-level session.

**`handleConfigure` publishes every mutable plugin field under `plug.mu`
together**: `byASN`, `prefixStore`, `config` and `refreshStop`, and `started`
is read and written under the same lock. A worker that
captured `st := byASN[asn]` under an RLock must re-read `st = byASN[asn]` under
the later write lock before it mutates. A reconfigure can swap `byASN` in
between, which orphans the captured pointer and loses the refresh result in
silence. This is one bug class, not four separate races. `refreshASNCtx` also
drops a result when the live state is not the one it captured and does not
share its lookups context: a lookup a reload superseded is dropped. The reload
cancelled it, through a context derived from the state's so the cancel lands
before the purge, so it either committed nothing or committed before the
purge removed it. A reload that kept the as-set shares the context, and its
answer is applied.

**The prefix store is this plugin's own**, in memory, and holds only entries
for ASNs it enrolled: nothing else writes to it, and `forgetUnconfigured` drops
an ASN the config no longer enrolls. The firewall IRR plugin builds its own
store and persists it. See `docs/architecture/resolve.md` for the store itself.

**On a lookup error, `Refresh` returns a non-nil `CachedEntry` alongside the
error**, carrying the resolved AS-SET and no prefixes, so this plugin can still
record the fallback AS-SET. It must not update its in-memory map or the store on
that path.

**The IRR client caches lookups for one hour in memory**, so a `Refresh` inside
the TTL is a silent no-op. A test that must see a lookup failure needs a fresh
client pointed at an unreachable server.

**`strings.Cut(text, "nlri ")` matches a substring.** A test case that means to
lack the token must not contain it anywhere, and `"no nlri here"` does.
