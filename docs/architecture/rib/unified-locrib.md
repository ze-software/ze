# The Unified Loc-RIB

Ze had two RIBs: a BGP-shaped one (Adj-RIB-In per peer, plus best-among-BGP) and
a kernel-facing one in sysrib. Cross-protocol arbitration happened in the
kernel. One store now holds candidates from every source, runs cross-source best
path, and feeds the kernel FIB through the existing event plumbing.

<!-- source: internal/core/rib/locrib/candidate.go -- Path, AdminDistance, pathKey -->
<!-- source: internal/core/rib/locrib/entry.go -- PathGroup, selectBest -->
<!-- source: internal/core/rib/locrib/manager.go -- RIB, NewRIB, Insert, InsertForward -->
<!-- source: internal/core/rib/locrib/change.go -- the Change stream -->
<!-- source: internal/core/rib/locrib/default.go -- the process-wide instance -->
<!-- source: internal/core/rib/locrib/shard.go -- sharded Loc-RIB -->

## Owner requirement: internal and external system RIB

On 2026-11-12, Thomas rejected the proposal to make the system RIB internal-only:

> Please change it so we support it both from internal and external correctly.

The registered runtime obtains an authoritative local reader or a selected feed
over the SDK. Both use the same forwarding evaluator; BGP observation events
are not an alternate system-RIB input. Equivalent daemon and kernel behavior
still requires the paired execution evidence recorded by the repair.
<!-- source: internal/component/sysrib/register.go -- runRouteViewPlugin -->
<!-- source: internal/component/sysrib/route_view_worker.go -- consume -->

The consumer transport is recorded in
`spec-route-policy-1-gate-proof-and-forked-sysrib`. It keeps admission and
cross-source selection in the daemon's Loc-RIB. Both execution modes must
preserve distances, next-hop resolution, ECMP, candidate inspection,
configuration changes, FIB output, and recovery. An external-run refusal or
an internal replacement for the configured subprocess does not meet the
requirement.

## The decisions

**The external selected projection is a reader, not a second Loc-RIB.**
`selectedProjection` stores the authority's selected paths in the generic prefix
store. It validates and owns a whole page before applying it. Snapshot pages,
including reconciliation tombstones, stay private until the completion marker;
then the completed table becomes readable. Deltas apply as a whole before the
caller resolves dependent routes. A reset makes the reader unavailable, which
is distinct from a completed empty table. The projection does not admit paths,
rank distances, evaluate policy or retain hidden candidates.
<!-- source: internal/component/sysrib/projection.go -- selectedProjection, Apply, Synchronized, LPM -->

The projection preserves resolved administrative distance, metric and recursive
flags, AIGP and tag presence, primary and backup forwarding metadata, and the
complete equal-cost set. Per-member source ownership, labels and Service SIDs are retained beside
the local `NextHop` value, which carries gateway, device, on-link state and
weight. Both readers copy that metadata into the forwarding cache. Promotion
uses the promoted member's labels and SID for the primary FIB fields. Protocol
and family names are looked up in the sealed registrations.
Wire input never creates a registration.
<!-- source: internal/component/sysrib/projection_path.go -- projectionPath, prepare -->
<!-- source: internal/component/sysrib/projection.go -- newSelectedProjection -->
<!-- source: internal/component/sysrib/route_view_forwarding.go -- prepareRouteViewSelectedMetadata -->
<!-- source: internal/component/sysrib/route_view_worker_local.go -- prepareRouteViewLocal -->
<!-- source: internal/component/sysrib/ecmp.go -- siblingPath -->

The authority supplies the forwarding group through `PathGroup.ForwardingMembers`.
Another source may contribute the primary's address and device with its own
labels and permission. Only a member owned by the primary's source is excluded
for naming the primary target.
It combines the primary producer's explicit or intra-source group with admitted
paths from other protocols at the same distance and metric. Each other path
contributes its primary target and its complete explicit `Path.ECMP` group,
with that path's source, labels and Service SID on every target. A path whose
usable targets exist only in `Path.ECMP` contributes those targets without
inventing a primary; a path with neither contributes nothing. Primary selection
keeps its distance, metric and first-seen ordering. Each member retains its
numeric source until export. On the wire, an optional member `protocol` names a
different registered owner; omission means the primary's protocol owns that
member. Both readers resolve ownership before applying FIB permissions.
Identical targets from different sources remain distinct until that permission
filter runs, so withholding one source cannot hide another source's permitted
path. The FIB then deduplicates and bounds the rendered target group as before.
Withholding a member also releases its gateway and Service SID dependencies;
permitting it again restores them from the retained selected value. Untracking
always releases prior dependencies, including when their owner is now withheld.
`Change.ECMP` and `ECMPMembers` retain their intra-source contract for protocol
observers. The selected forwarding view carries the complete group.
<!-- source: internal/core/rib/locrib/forwarding_members.go -- ForwardingMembers, equalCostPath, equalCostMember -->
<!-- source: pkg/plugin/rpc/route_view.go -- RouteViewNextHop -->
<!-- source: internal/component/sysrib/ecmp.go -- siblingProtocol, ecmpGroup, finishECMP -->

The FIB event carries labels on each equal-cost member and one Service SID for
the entry. A promoted member supplies that entry-level SID. The selected-view
metadata does not add another SID field to the FIB event.
<!-- source: internal/component/sysrib/events/events.go -- BestChangeEntry, ECMPPath -->
<!-- source: internal/component/sysrib/fibimport.go -- fibChange -->

Member Service SIDs participate in next-hop dependency tracking. A resolved
equal-cost group excludes a member whose own SID does not resolve. Losing or
restoring its covering route re-evaluates the prefix without a producer update.
The winner's existing SID reachability check still applies before promotion.
<!-- source: internal/component/sysrib/fibimport.go -- trackNextHops, untrackNextHops -->
<!-- source: internal/component/sysrib/sysrib.go -- resolveECMPMember, fibEntry -->

**The Loc-RIB lives in `internal/core/rib/locrib/`, not inside a component.**
BGP is a candidate source and a reader. Sysrib is a candidate source and the FIB
consumer. Neither owns the store.

**The generic store is `internal/core/rib/store/` and imports nothing from
BGP.** `Store[T]` is BART-backed. `nlrikey.go` holds `NLRIToPrefix` and
`PrefixToNLRI`, keyed on `family.Family`. The map fallback stays behind
`-tags maprib` for benchmarking parity.

**ADD-PATH collapsed into the value layer.** The store no longer bifurcates
between a trie and a `map[NLRIKey]T`. BART is the only prefix index.
Per-path-id semantics live in BGP storage's `pathSet`, and locrib's
`PathGroup.Paths` is keyed by `(Source, Instance)`. BGP elects one best path
per prefix across every peer and ADD-PATH path before it mirrors anything, so
it writes one path per prefix at Instance 0 (`bgpLocRIBInstance`). ADD-PATH sessions gain
longest-prefix match and iteration. A caller that needs per-path-id lookup puts
a path-id map in the value layer.
<!-- source: internal/component/bgp/plugins/rib/storage/pathset.go -- value-layer ADD-PATH wrapper -->

**Cross-source best path runs off a distance table.** `Path.AdminDistance
uint8` orders before metric inside `selectBest`. The defaults follow Cisco and
Juniper, and `rib { distance { } }` overrides each of them. The Loc-RIB owns the
number: producers hand a path over without one, and every insert writes
`AdminDistance` from `resolvedDistance`, which reads the declaration for the
path's protocol from `internal/core/rib/distance` (BGP picks `ebgp` or `ibgp`
by `IsEBGP`). A path's `DistanceOverride`, set by a static route's own
`distance` or an accepting import chain, wins over the declared value for
that path alone. A protocol
neither declared nor in the bootstrap table ranks at `UndeclaredDistance` (255),
never at zero. When sysrib publishes a changed declaration it calls
`(*RIB).Reselect`, which re-resolves every stored path, re-runs selection, and
dispatches a `ChangeUpdate` for each prefix whose best or equal-cost set moved,
so a reload re-ranks the routes already installed and the FIB follows.
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance, DistanceProtocol, Reselect -->
<!-- source: internal/component/sysrib/route_view_distances.go -- applyDistances -->
<!-- source: internal/core/rib/locrib/policy.go -- admit -->

**An import gate judges every path before it is stored, and a rejected path
is kept hidden.** Every write ends in `(*RIB).insert`, so the gate sits there
and no source can skip it. `internal/core/rib/policy` carries one immutable
`Snapshot` through an atomic pointer: a compiled chain per source and one
central chain (`rib filter import`). `Snapshot.Evaluate` runs the source's chain,
then the central chain, which sees the tag the first one set; an answer that
is not an accept rejects, so a chain that cannot answer fails closed. Insert
reads the snapshot before it takes the shard lock and judges the path in
`admit`. When no chain is configured the snapshot is nil, so the gate is the
pointer load and the nil check, with no allocation, on the BGP full-table path.
Under the lock insert reloads the pointer and judges the path again if a newer
snapshot was published in between, because that snapshot's re-selection pass
may already have run over the shard.

A rejected path is upserted with `Path.Hidden` set. The RIB owns that flag, as
it owns `AdminDistance`, so a producer's value is discarded. `selectBest` and
`siblingNextHops` skip hidden paths, so `Best`, `LPM` and every `Change` leave
them out. A group whose paths are all hidden has no best, the same as an empty
group, so its subscribers see `ChangeRemove` and sysrib withdraws the FIB entry.
A hidden path stays stored, so `Lookup`, `Inspect` and `Iterate` still list it.

`LPM` skips groups whose paths are all hidden and returns the most specific
cover with a selectable path, including a default route. Each shard first
looks up its longest covering prefix; only a hidden-only match triggers exact
ancestor lookups, bounded by 32 for IPv4 or 128 for IPv6. The most specific
selectable result across the shards wins.
<!-- source: internal/core/rib/locrib/manager.go -- LPM -->

`Path.HiddenBy` names the chain that hid it (`ChainRoleSource` or
`ChainRoleCentral`), copied from the `RejectedBy` field of the verdict
`Snapshot.Evaluate` returned. It is one byte, in `Equal` and out of `key()`, so
`show rib candidates` reports the verdict the RIB acted on rather than a fresh
evaluation. A path no chain rejected carries `ChainRoleUnspecified`.

The policy metrics are kept in the same shard lock as the upsert, with no
allocation: `ze_rib_policy_hidden_paths{source}` moves when a stored path
changes between hidden and selectable or a hidden path is removed, and
`ze_rib_policy_rejected_total{source,chain}` counts every producer offer a
chain rejects. A `Reselect` re-judgement counts a rejection only when the
verdict changes, so recommitting an unchanged chain adds nothing. The series
are pre-bound per source and chain when the policy plugin hands the registry
to `locrib.SetPolicyMetricsRegistry`; the hidden counts are kept whether or not
a registry is bound.
<!-- source: internal/core/rib/locrib/policy_metrics.go -- accountStored, accountRemoved, SetPolicyMetricsRegistry -->
<!-- source: internal/core/rib/policy/policy.go -- ChainRole, Verdict.RejectedBy -->
An accepting chain may set `distance`, which writes the `DistanceOverride`
pair, and `tag`. Those set actions are applied to a copy of the producer's
own values. The path keeps those values unexported beside the values in
effect, so each evaluation starts from what the producer offered and a reload
never applies a set twice.

`locrib.PublishPolicy` publishes a snapshot and calls `Reselect` on the
process-wide RIB. `Reselect` judges every stored path again from the producer's
values, hidden paths included. It then re-ranks and dispatches `ChangeAdd` for a
prefix it re-admits, `ChangeRemove` for one it hides, and `ChangeUpdate` where
the best or the equal-cost set moved. Relaxing a filter therefore installs the
path with no producer event, and tightening one withdraws it.
<!-- source: internal/core/rib/policy/policy.go -- Snapshot, NewSnapshot, Evaluate, Set, Current -->
<!-- source: internal/core/rib/locrib/policy.go -- admit, producerPath, PublishPolicy -->
<!-- source: internal/core/rib/locrib/manager.go -- insert, siblingNextHops -->
<!-- source: internal/core/rib/locrib/entry.go -- selectBest -->
<!-- source: internal/core/rib/locrib/distance.go -- reselectShard, dispatchReselected -->

**The policy plugin is the only publisher of the snapshot.** A plugin whose
config holds an attachment declares it from its own `init()` with
`policy.MustRegisterRIBImport`. `isis`, `ospf`, `static` and `connected` fill
a source chain. `sysrib` fills the central chain from `rib`. The registration
adds the attachment's root to the config reads of the policy plugin, which
runs in the engine.

Its config transaction parses every chain at verify (`parseRIBImports`), so a
wrong chain fails the commit before anything changes. At apply,
`installRIBImports` compiles the chains into one snapshot and calls
`locrib.PublishPolicy`. A config with no chain publishes nil. An apply that
finds nil already published publishes nothing.
<!-- source: internal/component/policy/rib.go -- MustRegisterRIBImport, RIBImport -->
<!-- source: internal/component/policy/rib_compile.go -- parseRIBImports, installRIBImports, compileRIBSnapshot -->
<!-- source: internal/component/policy/register.go -- addPolicyConfigRead -->

**A filter type runs at the gate through an in-process route evaluator.** The
gate runs on every insert, from several shards at once, so it never calls a
plugin process. A filter type that a Loc-RIB attachment can use registers a
compiler with `policy.MustRegisterRouteEvaluator` from its own package. The
compiler turns the config of one instance into a function that judges a route
by value. `prefix-list` and `family-filter` register one.

Verify refuses a chain that names a type with no evaluator, and a
`<plugin>:<filter>` reference. A compiled chain applies Ze chain semantics. A
reject stops the chain. A filter that does not reject passes the route on with
its sets. `final` decides after the last filter.
<!-- source: internal/component/policy/rib.go -- RouteEvaluator, RouteEvaluatorCompiler, MustRegisterRouteEvaluator -->
<!-- source: internal/component/policy/rib_compile.go -- readRIBChain, compileRIBChain, ribChain.Evaluate -->
<!-- source: internal/component/bgp/plugins/filter_prefix/rib_evaluator.go -- compileRouteEvaluator -->
<!-- source: internal/component/bgp/plugins/filter_family/rib_evaluator.go -- compileRouteEvaluator -->

**A Path carries its whole next-hop set, not one address.** `NextHop` is the
route's gateway, `Interface` names its outgoing device, and `Weight` gives
that next-hop's share of the group. `OnLink` identifies a protocol-established
link-layer adjacency, including one outside the interface's IP subnet.
Sysrib preserves that adjacency instead of recursively replacing its gateway.
Recursive resolution carries the terminal device and adjacency marker into the
FIB. `ECMP` members carry the same forwarding metadata; a device-only member is
a usable path without a gateway. Dependencies include recursive members, so a
covering-route change can replace or remove one member while keeping the others.
Live changes and replay use the same resolved group.

`Interface`, `OnLink`, `Weight` and `SRv6SID` follow the `Labels` contract. They
are excluded from `key()`: changing a source's forwarding target updates the
same path. `Equal` compares them so the change reaches the FIB. `ECMP` stays out
of both. The authoritative forwarding comparison includes source ownership,
targets, labels and Service SIDs. A sibling ownership or metadata change advances the revision and emits an
update even when the winner and target set remain unchanged. Hidden and
unequal-cost paths do not enter that comparison. The Service SID survives
local changes, forked route-install RPC and replay.

Insertion resolves the offered path's distance once before comparison and
storage. An unchanged effective path, including its explicit ECMP set, needs
no temporary member copies. The upsert still stores the latest producer values
and accounts for every rejected offer. A notification borrows an explicit
intra-source next-hop slice directly from the path.
<!-- source: internal/core/rib/locrib/candidate.go -- Path.Equal -->
<!-- source: internal/core/rib/locrib/manager.go -- siblingMembers, siblingNextHops, insert, Remove -->
<!-- source: internal/core/rib/locrib/entry.go -- upsert -->
<!-- source: internal/core/rib/locrib/forwarding_members.go -- equalForwardingMemberSets -->
<!-- source: internal/core/rib/locrib/distance.go -- dispatchReselected -->

The fields an import chain reads follow the same contract. `Tag` is meaningful
only with `HasTag`, because 0 is a real tag. `Level` is the IS-IS level of the
route SPF already chose. `OSPFRouteType` is the OSPF path type, which is not
`RouteType`, the forwarding action. `Hidden` follows them too. All four are
out of `key()` and in `Equal`, so a re-advertisement with a new tag updates the
same path and reaches consumers.
<!-- source: internal/core/rib/nexthop/nexthop.go -- the NextHop value type -->
<!-- source: internal/core/rib/distance/distance.go -- the declaration seam the Loc-RIB reads -->

**The sources.** BGP, OSPF, IS-IS, the static plugin and the connected plugin
insert paths, and sysrib reads them. A connected path names NO next-hop, which is
what the recursive resolver treats as the end of a chain, so a protocol next-hop
covered only by an interface prefix resolves rather than reporting unreachable. A static route enters only when it is in the MAIN table: the store is
keyed by (family, prefix) and carries no table, so a named-table route would
collide with the main-table route for the same prefix
(`docs/architecture/static-routes.md`).

**Metric recursion is separate from forwarding recursion.** A terminal IGP path
contributes its SPF metric once. Recursive static paths follow the next hop;
recursive BGP paths contribute received AIGP, never MED. Missing AIGP and
unreachable next hops are distinct results. Accumulated cost saturates at the
64-bit limit. Selected-path and ECMP changes advance the Loc-RIB revision, which
lets a forked BGP RIB invalidate its metric cache without another BGP UPDATE.

**A refused kernel install grants no ownership.** An Update after a refused Add
still uses an exclusive add. Before replacing an existing IP route, the Linux
backend checks the destination, table and priority against the current kernel
protocol. A foreign route is retained. Changing a Ze route's priority installs
the new entry before removing the superseded Ze entry.
At startup, retained Ze routes are tracked separately from refreshed routes.
Replay may replace a retained entry after checking its current kernel owner;
only entries that receive no successful refresh are swept.
The route monitor retains an owned copy of the last successful forwarding
description. The shared watcher delivers Ze-owned deletions, and the monitor
matches their prefix, table and priority against that description before recovery.
An external deletion restores the complete entry, including device, adjacency,
weights, labels, SID and ECMP members. Route writes and cache updates share the
monitor's lock, so delayed notifications cannot restore withdrawn routes or
superseded priorities. Ze add notifications are ignored to prevent recovery loops.
A conflicting foreign replacement is reported and retained. The external-route
importer filters all Ze protocols so FIB output cannot return as external candidates.

**Two Insert methods, not variadic options.** `Insert` stays for non-BGP
callers. `InsertForward` threads the optional `ForwardHandle`. See
`docs/architecture/rib/forward-handle.md`.

**`FamilyIndex` was not introduced.** Every supported family is prefix-shaped
today, so BART keyed on `netip.Prefix` covers all of them. The abstraction waits
for the first non-prefix family that actually lands.

**Sharding is a behavior change to the manager, not a file move.** It landed
after the reorganization compiled and passed.

## The two triggers do not collapse

**This is the model. A proposal to consolidate the triggers has been made and
refuted once, at the cost of hours.**

| Trigger | Fires | Serves |
|---------|-------|--------|
| receive path (`StructuredEvent`) | per received UPDATE, duplicates included | forwarders: route server, route reflector |
| `locrib.OnChange` | per best-path change | state trackers: sysrib byte-mirror, route archive, RR cluster-list extractor |

Three facts refute driving forwarding from locrib:

1. The route server in ze forwards each received UPDATE; it is not a per-peer
   best-path computer. Where sources contest a prefix at a client without
   ADD-PATH, the reactor's writer owes that key and asks `bgp-rib`'s election
   for that one destination on demand (a recovery lookup). There is no
   per-peer Change to subscribe to.
2. Per-peer egress work (egress filters, RFC 4456 route-reflector injection,
   next hop, AS override, eBGP prepend) is keyed off the per-received-UPDATE
   trigger, not off a best change.
3. The receive-path trigger is also where the inbound filter pipeline runs, both
   the engine's loop-detection step and the per-peer import policy chain, whose
   members modify with copy-on-modify. Retiring it would re-home the filter pipeline for no
   gain.
<!-- source: internal/component/bgp/reactor/session_ownership.go -- oweAnnouncement -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- recoveryAdmission.admit -->

They could collapse only if locrib stored every received path per peer, which is
not a Loc-RIB.

**Phrases that signal the wrong model is returning:** "retire the receive-path
trigger", "single trigger", "per-peer best-path Change events", "drive
forwarding from locrib".

## Authoritative selected views

The daemon owns the selected-view service for the system RIB. Its Open, Read
and Close operations register from sysrib alongside an exact-generation
cleanup hook; shared plugin dispatch discovers them. Open is permitted during
configure, while Read and Close wait for runtime dispatch. The caller must be
the process configured to run `rib`, including a renamed block using `rib`.
JSON input cannot supply that identity. A session token belongs to that exact
process generation, and replacement waits for its predecessor's scoped cleanup.
<!-- source: internal/component/sysrib/register_route_view.go -- init, routeViewOpenHandler, routeViewReadHandler, routeViewCloseHandler -->
<!-- source: internal/component/sysrib/route_view.go -- open, withSession, cleanupLocked -->

The default `selected-feed` reader subscribes before scanning. Its callback puts only a prefix key in
a bounded channel and never waits for transport, retains a forward handle or
encodes a value. Snapshot and catchup requests copy selected primary, multipath
and repair metadata under the Loc-RIB read lock, including tag and AIGP presence.
No borrowed labels, next-hop slices or PathGroup storage survive that read.
The complete JSON reply is bounded off-lock, including its framing overhead.
<!-- source: internal/component/sysrib/route_view_feed.go -- offer, scanSnapshot, routeViewPageEntries -->
<!-- source: internal/component/sysrib/route_view_path.go -- routeViewCopyEntry -->

Snapshot pages remain provisional until Complete. Changes captured during the
scan or paging are reread from the authority; corrections remain snapshot
upserts or tombstones before completion. An older buffered add therefore cannot
restore a withdrawn prefix. Each record advances the independent cursor once,
and snapshot completion advances it once more, including an empty snapshot.
Idle delta reads retain their cursor. A distance application takes its barrier
after authoritative reselection; the metric revision is never a feed cursor.
<!-- source: internal/component/sysrib/route_view_feed.go -- snapshotPage, synchronize, deltaPage -->

Only one current selected-table snapshot is pinned, proportional to the
authority's selected prefixes and metadata, with a two-minute lifetime and no
history of prior snapshots. Consumed entries release their owned slices. The
capture channel holds at most 4096 keys; the pending delta journal holds at most
4096 records and charges at most 16 MiB of owned values, with thirty-second
retention. Overflow, expiry or an interrupted catchup invalidates the view and
fences its output before returning Reset. A failed withdrawal remains an error;
it cannot become a successful reset on a later read. A replacement retries a
stopped predecessor's failed cleanup before becoming the new owner.
<!-- source: internal/component/sysrib/route_view_feed.go -- readPage, invalidate, readFailure -->
<!-- source: internal/component/sysrib/route_view.go -- open, cleanupLocked -->

The session lifetime is independent of the borrowed configure request. One
joined generation watcher observes process exit and transport failure, polling
transport state every 250 ms outside shard and request locks. The same worker
also checks snapshot, journal and candidate retention while the consumer is
idle. Maintenance attempts the request lock without waiting, so a scan cannot
block transport-failure detection. Reads wait at most 1000 ms and also select
cancellation. Cleanup cancels before joining requests and the watcher, then
releases selected and candidate snapshots. A caller must rebuild after Reset
and consume a completed view before publishing forwarding output from its
cursor. A successful distance application that requires reset receives a new
reset marker; even a consumer at the previous frontier must read the explicit
Reset before considering the acknowledgment synchronized.
<!-- source: internal/component/sysrib/route_view.go -- watchGeneration, cleanupLocked -->
<!-- source: internal/component/sysrib/route_view_feed.go -- wait, validateConsumed -->

An internal system RIB can request the `local-authority` reader when opening its
session. The authenticated process must run internally and have a readable
daemon Loc-RIB. This lease uses direct reads and the existing local feed.
It retains no selected snapshot, capture channel or journal, and remains ready
at logical cursor zero. Distance acknowledgment follows authoritative reselection
and returns zero because the direct reader already sees that authority.
Selected Read requires reopening with `selected-feed`.
<!-- source: internal/component/sysrib/route_view.go -- open, read, maintain -->
<!-- source: internal/component/sysrib/route_view_feed.go -- synchronize, validateConsumed -->

Generation checks, candidate maintenance and output cleanup remain active for
the local lease. Ordinary external operation uses the default reader.
<!-- source: internal/component/sysrib/route_view.go -- open, maintain, cleanupLocked -->

The consumer starts one joined runtime worker after configuration. External
pages update a read-locked projection before any forwarding evaluation or
recursive cascade. An incomplete snapshot emits no forwarding or replay.
Only the Broadcast identity is admitted on the system replay hop. Zero and
targeted identities are refused before queue mutation or wakeup, in either
placement. Requests received during synchronization coalesce until the complete
view can answer them.
<!-- source: internal/component/sysrib/route_view_worker.go -- Start, readSelected, HandleEvent, replay -->

The runtime `rib-routing-view` doctor check reads a monitor owned by that
consumer. Initialization, reset, reconstruction, failure and shutdown clear
readiness. A completed authoritative view becomes healthy after its output is
acknowledged, in either placement. The monitor declares scalar readiness,
pending-prefix and reconstruction metrics through the existing SDK publisher
before startup. It starts no additional worker. Routing-view failure logs name
the check's declared `doctor-rib-routing-view-unavailable` code, the configured
plugin instance and its recovery command.
<!-- source: internal/component/sysrib/doctor_route_view.go -- findings, newRouteViewHealth -->
<!-- source: internal/component/sysrib/register_route_view.go -- reportRouteViewFailure -->

The end of the consumer's engine session is a normal stop, never a failure,
and the doctor check then reports the stopped state. Forked, a daemon stop
closes the connection without `bye`, so the worker's read or publication can
meet the closed connection before its owner context ends. The daemon's stop
also ends the daemon-side wait of a pending long-poll read before the
connection closes, and so does a stop of only the system RIB's process
generation. The engine leaves a request that its stop cancelled unanswered,
so the read ends with the closed connection and never with a refusal.
Internally, the stop fences native dispatch, and a native publication or read
the stop cancelled after admission returns the same closed-bridge answer, so
it is a normal stop too. Every other error under a
live owner context is a failure, including an owner's refusal whose text
names EOF, a closed connection or a stopped request: a refusal arrives as an
RPC error and never as a transport end.
<!-- source: internal/component/sysrib/route_view_worker.go -- run -->
<!-- source: internal/component/sysrib/route_view_client.go -- routeViewTransportStopped -->
<!-- source: internal/component/sysrib/route_view_feed.go -- wait -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpJSON, engineSessionEnded -->
<!-- source: internal/component/plugin/server/dispatch_native.go -- directCallError -->

Candidate commands and configuration changes enter the same worker through a
sixteen-request queue. Each request's two-minute forward budget starts before
enqueueing, so queue residence cannot renew its deadline. Candidate scans pin
the daemon epoch, consumer session,
snapshot and exact row cursor; they return owned raw JSON, preserving integers
without a floating-point conversion. Configuration reserves an ordered distance
identity before sending, consumes the acknowledged selected frontier, and
collects forwarding changes until the final permission sweep can replace each
prefix's pending result. Only those final values are published and acknowledged.
<!-- source: internal/component/sysrib/route_view_worker_operations.go -- Candidates, configure, candidates -->

Unknown system-RIB commands return a complete operator refusal listing
`show rib`, `show rib candidates`, `show nexthop-table` and `show ecmp-groups`,
with `ze show rib` as the recovery command.
<!-- source: internal/component/sysrib/register.go -- routeViewCommandHandler -->

The local worker subscribes before its initial scan and queues at most 4096
prefix keys. It copies selected paths under the Loc-RIB lock, then resolves and
publishes after releasing that lock. Overflow requests a full scan, including
removal of cached prefixes absent from authority, rather than retaining borrowed
change values or an unbounded overflow set.
<!-- source: internal/component/sysrib/route_view_worker_local.go -- newRouteViewLocalCapture, offer, scan, consumeLocal -->

After recording overflow, the callback retries the same dropped key without
waiting. That key or another
queued key wakes recovery even when the worker already drained the queue.
One two-minute lifetime bounds all attempts of a full scan, including continuous
overflow, and worker shutdown cancels the scan.
<!-- source: internal/component/sysrib/route_view_worker_local.go -- recordOverflow, scan, consumeLocal -->

Both readers use the same selected forwarding and cascade functions. Publication
awaits the daemon acknowledgment for each ordered batch. Internal placement
requires the native value operation; external placement encodes the existing
batch once. Batches have at most 4096 records and oversized frames split in
bounded halves, preserving all metadata or refusing an oversized single route.
<!-- source: internal/component/sysrib/route_view_worker_output.go -- publish, publishChunk -->

A selected page can be admitted before the daemon fences its lease, then reach
the consumer before an obsolete publication is refused. The publisher probes
that exact lease and cursor without waiting. Recovery requires explicit Reset
with `rejected-publication` naming the exact refused operation, followed by
normal projection validation. The existing worker then closes the fenced
lease and reconstructs current authority. The daemon records this proof only
after complete output, operation and cursor validation, before emission, for
a typed retention or cursor reset. A terminal invalidation remains terminal
even if another reset follows. One owned refused batch bounds retained proof;
its exact retry is the only retry that can expose it again. Previous completed
operation identities remain consumed after the output acknowledgment is freed.
<!-- source: internal/component/sysrib/route_view_output.go -- publish, provePublicationReset -->
<!-- source: internal/component/sysrib/route_view_feed.go -- invalidate, restartRefusal, resetPage -->
<!-- source: internal/component/sysrib/route_view_worker_output.go -- publicationReset -->

The Broadcast identity stays in the consumer's request map until all its
output is acknowledged. Each arrival advances that identity's revision. The
worker retires only the revision it actually served, preserving a request that
arrives after delivery but before the prior acknowledgment. Duplicate arrivals
retain one identity slot; an exhausted revision refuses without mutation.
Before rendering a local replay, the worker consumes captured authority keys
through the same bounded inspection and overflow recovery as live changes. A
replay wakeup cannot retire a newer request using state older than those keys.
A proven publication reset during replay reconstructs
the same worker and answers the retained broadcast request from current
authority. It cannot replay the fenced projection's obsolete routes.
<!-- source: internal/component/sysrib/route_view_worker.go -- consume, replay -->
<!-- source: internal/component/sysrib/route_view_worker_local.go -- applyLocal, consumeLocal -->

Both publication adapters revoke the authenticated owner's visible proof before
decoding a new attempt. A malformed JSON envelope, malformed batch or invalid
native value therefore cannot borrow an earlier valid refusal's proof. Another
process generation cannot revoke the current owner's proof or output.
<!-- source: internal/component/sysrib/register_route_view_output.go -- routeViewPublishHandler, routeViewPublishNative -->
<!-- source: internal/component/sysrib/route_view_output.go -- revokePublicationReset -->

Forwarding publication uses one registered operation with JSON and native value
adapters. Both validate the entire batch before changing its output ledger.
The JSON adapter stops before row 4097 without decoding the remaining rows.
Both adapters refuse invalid UTF-8 in device and gateway-zone names.
The daemon emits the existing typed `BestChangeBatch`, including replay and
primary, ECMP, label, SID and repair metadata. Native publication counts its
complete request frame without serializing route values to JSON.
<!-- source: internal/component/sysrib/register_route_view_output.go -- routeViewPublishHandler, routeViewPublishNative -->
<!-- source: internal/component/sysrib/route_view_output.go -- publish, routeViewValidateOutput -->
<!-- source: internal/component/sysrib/route_view_output_size.go -- routeViewPublicationBytes -->

The active generation and completed reader cursor authorize publication.
Positive operation identities order batches. An identical retry returns its
original acknowledgment without another emission, even after a newer completed
read. New operations require the most recently consumed cursor.
<!-- source: internal/component/sysrib/route_view_output.go -- publish -->

The ledger records each possibly delivered prefix before emission.
Failed emission fences the view and
withdraws its outstanding output. Failed withdrawal remains in the ledger and
blocks replacement until retry succeeds.
<!-- source: internal/component/sysrib/route_view_output.go -- publish, withdraw -->

Cleanup emits withdrawals only for the session's ledger. Operating-system
protocols cannot acquire output ownership, and the caller's existing FIB
permission decision keeps withheld routes out of publication. Unknown withdrawals
acquire no ownership and emit no removal. Acknowledgment means accepted event
delivery, and does not claim that a kernel or another backend programmed it.
<!-- source: internal/component/sysrib/route_view_output.go -- routeViewValidateOutputEntry, publish, withdraw -->
<!-- source: internal/component/sysrib/fibimport.go -- fibPermitted -->

Cleanup scans the ledger once and emits family batches of at most 4096 entries.
Each emitted batch owns its storage, which later cleanup cannot reuse.
Fencing releases the retained acknowledgment. Failed batches retain their ledger
entries for retry, and successful cleanup releases the empty ledger.
<!-- source: internal/component/sysrib/route_view_output.go -- withdraw -->

## Kernel integration fixtures

The FIB package's recursive, withholding and on-link integration tests start
the real plugin server and manager with the system RIB's registered dependency
closure. The original delivered section becomes the coordinator's config tree;
the server handles nested configure requests, binds the actual process identity
and activates the current routing-view transport. Its accepted best-change
events reach the tests' existing namespace-bound FIB writers. These tests prove
the selected forwarding decisions against the kernel; the paired daemon tests
separately prove operator configuration and external process placement.
<!-- source: internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go -- startSysRIB -->
<!-- test: internal/plugins/fib/kernel/crossfamily_integration_linux_test.go -- TestFIBRecursiveIPv4ViaIPv6 -->
<!-- test: internal/plugins/fib/kernel/fibwithhold_integration_linux_test.go -- TestFIBWithholdLeavesNoKernelRoute -->
<!-- test: internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go -- TestFIBOnLinkAdjacencyAndUnsupportedPrefix -->

## Constraints

**`OnChange` subscribers run synchronously under the locrib write lock, so they
must be cheap.** `AddRef` and the handle's byte copy are lock-safe, using
`sync.Once` plus an atomic. Anything heavier belongs on the subscriber's own
goroutine. Sharding must keep this contract.

`Inspect` keeps the shard read lock while its callback reads a live
`PathGroup`; the callback must not retain the group or its `Paths` slice.
`Best` selects and copies the best `Path` inside that same read scope, so an
in-place upsert cannot race selection. `Lookup` returns a shallow group copy:
its slice is not a snapshot and must not be read alongside writers.

**Code that walked the old `multi` map now walks `pathSet` inside the trie
value.** Single-path families are unaffected.

## What consumes it

BGP publishes a BGP-sourced `Path` into locrib instead of emitting the
final cross-protocol best. Sysrib registers as a candidate source for
kernel-learned routes and subscribes to best-path changes for FIB programming. A
future protocol source registers the same way: a `Path` with an
`AdminDistance` and nothing else. The type is `Path` and not `Candidate`,
because one entry is one path from one source.

Per-family NLRI splitting consumes the moved keys. A registered `nlrisplit.Splitter`
walks native wire boundaries with a `func([]byte) bool` visitor. Returning `true`
continues; returning `false` stops after the current NLRI, includes it in the
returned count, and returns no error without inspecting the remaining bytes.
A nil visitor validates and counts the entire section without allocating.
Malformed entries reached by the walk return an error and the count of preceding
entries. Visitor slices alias the input and retain the ADD-PATH identifier where
the family uses it; callers MUST copy bytes they retain beyond the callback.
`Split` and `SplitWithdrawn` materialize all entries using the same framing.
<!-- source: internal/core/bgp/nlri/nlrisplit/nlrisplit.go -- per-family NLRI split -->
