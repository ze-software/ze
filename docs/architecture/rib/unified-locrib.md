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

## The decisions

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
by `IsEBGP`). A path's own `DistanceOverride`, set only by a static route that
names `distance`, wins over the declared value for that path alone. A protocol
neither declared nor in the bootstrap table ranks at `UndeclaredDistance` (255),
never at zero. When sysrib publishes a changed declaration it calls
`(*RIB).Reselect`, which re-resolves every stored path, re-runs selection, and
dispatches a `ChangeUpdate` for each prefix whose best or equal-cost set moved,
so a reload re-ranks the routes already installed and the FIB follows.
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance, DistanceProtocol, Reselect -->
<!-- source: internal/component/sysrib/register.go -- reselectLocRIB -->

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
of both; a group change is detected through `Change.ECMP`. The Service SID
survives local changes, forked route-install RPC and replay.
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

1. The route server in ze is forward-all, not a per-peer best-path computer.
   There is no per-peer Change to subscribe to.
2. Per-peer egress work (egress filters, RFC 4456 route-reflector injection,
   next hop, AS override, eBGP prepend) is keyed off the per-received-UPDATE
   trigger, not off a best change.
3. The receive-path trigger is also where the inbound filter pipeline runs, both
   the in-process ingress filters with copy-on-modify and the external-plugin
   import policy chain. Retiring it would re-home the filter pipeline for no
   gain.

They could collapse only if locrib stored every received path per peer, which is
not a Loc-RIB.

**Phrases that signal the wrong model is returning:** "retire the receive-path
trigger", "single trigger", "per-peer best-path Change events", "drive
forwarding from locrib".

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
