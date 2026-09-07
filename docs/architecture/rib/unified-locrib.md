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
`PathGroup.Paths` is keyed by `(Source, Instance)`. ADD-PATH sessions gain
longest-prefix match and iteration. A caller that needs per-path-id lookup puts
a path-id map in the value layer.
<!-- source: internal/component/bgp/plugins/rib/storage/pathset.go -- value-layer ADD-PATH wrapper -->

**Cross-source best path runs off a distance table.** `Path.AdminDistance
uint8` orders before metric inside `selectBest`. The defaults follow Cisco and
Juniper, and `rib { distance { } }` overrides each of them. The producer stamps
the value it reads from `internal/core/rib/distance`, because `selectBest` runs
before sysrib sees the route, so a number that reaches sysrib alone cannot change
cross-source selection.

**A Path carries its whole next-hop set, not one address.** `NextHop` is the
route's own gateway, `Interface` is the device to send out of, and `Weight` is
that next-hop's share of the group. `ECMP` holds the rest of the group, one
`nexthop.NextHop` per member with the same three facts. A protocol-learned path
names a gateway and leaves the device and the weights zero; a configured route
may name a device instead of a gateway, and may weight the members.

`Interface` and `Weight` follow the `Labels` contract. They are excluded from
`key()`, because a source moving a prefix to another device is the same path
updated rather than a second path, and they are compared by `Equal`, because the
FIB has to observe the move. `ECMP` stays out of both: a group change is detected
through `Change.ECMP`.
<!-- source: internal/core/rib/nexthop/nexthop.go -- the NextHop value type -->
<!-- source: internal/core/rib/distance/distance.go -- the declaration seam producers read -->

**The sources.** BGP, OSPF, IS-IS and the static plugin insert paths, and sysrib
reads them. A static route enters only when it is in the MAIN table: the store is
keyed by (family, prefix) and carries no table, so a named-table route would
collide with the main-table route for the same prefix
(`docs/architecture/static-routes.md`).

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

**Code that walked the old `multi` map now walks `pathSet` inside the trie
value.** Single-path families are unaffected.

## What consumes it

BGP publishes a BGP-sourced `Path` into locrib instead of emitting the
final cross-protocol best. Sysrib registers as a candidate source for
kernel-learned routes and subscribes to best-path changes for FIB programming. A
future protocol source registers the same way: a `Path` with an
`AdminDistance` and nothing else. The type is `Path` and not `Candidate`,
because one entry is one path from one source.

Per-family NLRI splitting consumes the moved keys.
<!-- source: internal/core/bgp/nlri/nlrisplit/nlrisplit.go -- per-family NLRI split -->
