# Structural Forwarding: what left the critical path

The route-server forwarding gap against BIRD on grouped input was closed by
changing structure, not by tuning constants. `docs/architecture/core-design.md`
Section 9 describes the mechanisms that resulted. This page records what they
replaced and the ordering constraint that keeps them correct.

<!-- source: internal/component/bgp/reactor/forward_body.go -- shared body building -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactor-native RS forwarding -->

## Four costs, and what replaced them

| Old cost | Replacement |
|----------|-------------|
| Forward context stored in a `sync.Map` that every structured and text dispatch path loaded from, one map hop per UPDATE | a value-carrying `workItem` passed through dispatch, holding the source peer, the message and the text payload |
| Peer-down withdrawal inventory built inline in the forward path, allocating strings and writing maps before any byte left the box | NLRI records extracted as `netip.Prefix` before forwarding, applied to the withdrawal map after forwarding |
| One `Retain(id)` per destination peer, so N destinations meant N entry points | one `RetainN(id, peerCount)` per update id, fed by a pending dispatch buffer |
| Identical path attributes written as separate TCP writes | `fwdBucketMerge` at the batch-handler level merges NLRIs into fewer outbound bodies, inside the negotiated message size limit |

The critical path now touches no `sync.Map`, allocates no string for an NLRI
key, and issues one cache retain per UPDATE.

## The ordering constraint

**`extractWireNLRIRecords` MUST run before forwarding.** Cache eviction can free
the pool buffer backing `msg.WireUpdate` once `ForwardCached` has run, so
extraction after forwarding reads freed memory. Application to the withdrawal
map happens after forwarding, when the keys are already materialized. The pooled
`nlriRecord` slice is returned once the map update completes.

Correctness of the withdrawal map depends on this split: extract while the cache
buffer is alive, apply once the bytes are gone.

## Receive validation and cache changes

`forwardUpdateCore` queries the Adj-RIB-In validation gate for each prefix and
received Path Identifier. A pending or ineligible path becomes a withdrawal;
eligible sibling NLRIs retain their attributes. A generation that has already
been replaced cannot advertise over its replacement. The cached received bytes
remain unchanged, and derived forwarding buffers belong to the same cache entry.
The forwarding API succeeds when either the eligible section or its synthesized
withdrawal is dispatched. An ineligible announcement that becomes an accepted
withdrawal is not reported as suppressed by every destination.
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateValidated -->

When receive validation is enabled, or an UPDATE carries FlowSpec or AIGP,
`reactorForwardRS` defers to cached forwarding. That choice remains set on the
source peer across reconnects and configuration changes. Later attribute-free
withdrawals and replacements therefore use the same source FIFO as the earlier
announcement. Peers that never require this deferral keep the direct-write path.
The choice is recorded at receipt even when the fast path is disabled, so
enabling it cannot overtake cached work already queued for that source.

Live cache entries also retain their source peer and session generation. Source
removal or re-establishment invalidates queued entries, and destination workers
check that generation before writing. A new peer at the same address cannot
forward a cached UPDATE from the removed peer.

The common path serializes eligibility lookup and destination enqueue. Overflow
superseding removes an older equal body and appends the new item at the tail.
Recovery therefore stays after an intervening validation withdrawal, including
when the destination is congested. The removed item releases its cache reference
and pool handles once.

`validationChanged` enqueues route keys on the route server's existing source
workers. Each worker flushes its buffered live UPDATEs before asking Adj-RIB-In
for `replay-path`, which returns the current retained path or an explicit
withdrawal for a removed path. This callback performs no RPC on the emitting
Adj-RIB-In command handler. Recovery therefore uses the stored bytes without
requiring another UPDATE, and an event overtaken by another change still queries
the current route.
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- validationChanged, processValidation -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateCore, forwardValidationWire -->
<!-- source: internal/component/bgp/reactor/reactor_api_relay.go -- buildRelayUpdate, buildRelayWithdrawal -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- dispatchOverflow, forwardSourceCurrent -->
<!-- source: internal/component/bgp/reactor/received_update.go -- receivedPeer, receivedGeneration -->
<!-- source: internal/component/bgp/reactor/peer.go -- forwardCached, forwardGeneration, setState, Stop -->

FlowSpec uses the same gate with a full native NLRI key rather than a CIDR
prefix. Its selecting RIB publishes per-path generation and authorization after
checking the associated unicast table; a missing provider is ineligible. The
partitioner splits SAFI 133/134 NLRIs, preserves permitted siblings and turns
infeasible paths into MP_UNREACH. When optional receive validation is disabled,
non-FlowSpec sections pass through unchanged, including explicit unicast
withdrawals in a mixed UPDATE. Mandatory FlowSpec authorization supplies no
unicast eligibility verdict.

The key retains native framing with the shortest length encoding, so a rule
advertised with either RFC 8955 length form has one eligibility and withdrawal
identity. ADD-PATH remains a separate field and a VPN Route Distinguisher remains
part of the key.

Because the authorization lookup is synchronous, FlowSpec configuration requires
an internal `bgp-rib` process. An explicitly external RIB is rejected even when
it receives all UPDATE and state events; event delivery alone cannot provide
that lookup.

For FlowSpec changes, route-server workers fetch the current path attributes
directly from the selecting RIB instead of using the CIDR-only `replay-path`
command. The reflector queues the same recovery work on its validation worker.
Both use `RelayStoredRoute`, including its normal reflection and egress policy
checks. The reconstruction retains FlowSpec's required empty next-hop field.
If the route server's optional receive-store replay is unavailable on peer-up,
it replays the authoritative RIB's feasible FlowSpec paths before sending EOR.
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- flowSpecEligible, flowSpecPath -->
<!-- source: internal/component/bgp/plugins/rr/validation.go -- startValidation, replayValidation -->

## How long a Path Identifier lives

Ze's own Path Identifier for a re-advertised path is held in one table, which
both rails read, so a replayed route and a live forward of one path carry the
same value (`forward_path_id.go`).

The key mirrors the key the SOURCE uses to name a path. A source that negotiated
no ADD-PATH names a path by its prefix and frames no identifier, so ze holds one
identifier for that source's whole session and gives every prefix of it the same
one. A source that negotiated ADD-PATH names a path by (prefix, identifier), so
ze holds one entry per pair.

Native families are walked with their registered NLRI splitters on both raw and
cross-context forwarding paths. The framing is preserved while the leading
ADD-PATH identifier is replaced. Identity comes from each family's registered
route-key function, so changes to non-key label fields or FlowSpec length
encoding keep the identifier. Long native keys are retained in full.

Only the second kind is freed before the peer is removed, and the free runs at
ONE point: the recent-update cache evicting the UPDATE that withdrew the path
(`recent_cache.go` `evictLocked`). It cannot run at the end of either rail,
because one UPDATE reaches both: `reactorForwardRS` serves the destinations it
can and hands the rest to the rs plugin as `FastPathSkipped`, which forwards them
through `forwardUpdateCore`. Freeing after the first rail would mint a fresh
identifier for the second rail's destinations, and each of those would then hold
a route ze can never withdraw.

The free has two exceptions. A path retained by the validation provider keeps
its identifier while ineligible, so recovery can replace its earlier
advertisement. An UPDATE that withdraws and announces the same pair also keeps
the identifier because the destination still holds the path. RFC 7606 Section 5.1
forbids a conforming sender to write both fields,
so the announced section is empty for every ordinary withdraw and the exception
costs nothing. When it is not empty, the section is keyed once
(`fwdAnnouncedPaths`) rather than walked once per withdrawn NLRI: both counts
come from one peer's message, and the product of the two is what the peer would
otherwise choose.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdReleaseSection, fwdAnnouncedPaths -->
<!-- source: internal/component/bgp/reactor/recent_cache.go -- evictLocked, Delete -->

The walk runs BEFORE the entry's buffer goes back to the pool, because it reads
slices into that buffer (`docs/architecture/memory/lifetime-contracts.md`).

With receive validation enabled, eviction also checks whether the path remains
pending or stored in Adj-RIB-In. A retained ineligible path keeps its identifier
for recovery, and an obsolete withdrawal cannot free its replacement's identity.
The presence check and identifier removal share the identifier-table lock.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- releasePath -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- validationRetainsPath -->

## Request-owned batch writes

`AnnounceNLRIBatch` and `WithdrawNLRIBatch` carry their caller's context through
grouped, partial-deduplication, stale-route and split-message sends. Waiting for
the session writer is cancellable; a cancelled waiter cannot emit its UPDATE
after another writer releases the session.

After acquiring the writer, the request owns the socket deadline through the
write and flush. Cancellation interrupts a blocked flush. If it interrupts a
write, the socket is closed because a partially emitted BGP frame cannot safely
be resumed by a later request. Cancelling only a waiter leaves the active
writer's socket untouched. Cleanup joins the cancellation callback before
releasing write ownership, so an old request cannot expire a later writer.
<!-- source: internal/component/bgp/reactor/session_write.go -- sendUpdateCounted, requestWriteDeadline -->
<!-- source: internal/component/bgp/reactor/session_write_mutex.go -- LockContext -->

## Configured native next-hop self

A native `PluginRoute` retains explicit `NextHopSelf` intent until initial sync.
Both grouped and ungrouped builders resolve it from the connected session's
actual local endpoint, independently of the peer's default forwarding next-hop
mode. An unavailable endpoint prevents that route from being sent; it does not
substitute a configured address or another next hop. Self intent is part of the
grouping key, so a self route cannot borrow an explicit route's attributes.
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendPluginRoutesVia, pluginRouteGroupKey -->
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams -->

## What bucketing excludes, and why

Bucket merge handles an item with exactly one `rawBodies` entry, no parsed
`updates`, and `peerBufIdx == 0`. Those three conditions exclude every
parsed-update path, and every copy-on-modify path whose copy differs per
destination: such bytes must not be merged.

One copy is not excluded, and must not be. The RFC 7911 Path Identifier rewrite
(`fwdRegenerateRawPathIDs`) writes into a pooled copy whose handle travels on
`fwdBodyResult.transcodeBuf` rather than on `peerBufIdx`, so the item still meets
all three conditions. It is correct to merge it, because ze's identifiers are
chosen per ingress path and not per destination: the copy carries the same bytes
for every destination that reads Path Identifiers.

Merging uses pooled scratch buffers and FNV-64a for attribute grouping. A hash
collision is caught by a `bytes.Equal` check against the actual attribute bytes
before any merge happens.
