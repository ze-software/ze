# Spec: blackhole-authorization-from-irr

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Let a peer's blackhole authorization come from an IRR AS-SET, not only from a
hand-written prefix list.**

Owner request, 2026-08-13, on reading `authorized-covering-prefix 192.0.2.0/24`: make this
more flexible, an as-set or multiple prefixes. That leaf-list is now named
`prefixes`, and the container's `community` leaf-list is now `communities`
(owner, 2026-08-13, before any release carried either spelling).

### What already works

**Multiple prefixes already work.** `prefixes`
(`internal/component/bgp/plugins/rib/yang/ze-rib.yang`) is a `leaf-list` carrying
`ze:cumulative`. An operator states as many as they need. They accumulate down the bgp,
group and peer levels.

**Ze already resolves AS-SETs.** `filter_irr` takes `session.irr.as-set` per peer
(`internal/component/bgp/plugins/filter_irr/config.go`). It holds a cache and refreshes on
a configured interval. `update bgp irr as-set` drives it by hand.

**The containment test also exists.** `evaluatePrefix` (`.../filter_irr/match.go`) answers
whether an entry covers a route. `prefixListFromIRR` sets `le: 32` and `le: 128`, so a /32
blackhole inside an authorized block passes it.

**Neither half is missing. The join is.**

## The syntax stays stable

Owner constraint, 2026-08-13: the configuration that lands today MUST keep working when
this spec lands. **It does, and the shape needs no change now.**

`as-sets` arrives as a SIBLING leaf-list beside `prefixes`. Every configuration
written today keeps parsing. The name matches `session.irr.as-set`, so an
operator meets one word for one kind of object, and the two answer DIFFERENT
questions: `session.irr.as-set` states what the peer may advertise at all, and
the blackhole `as-sets` states what it may blackhole within. Whether a deployment
puts the same set in both is the operator's business.

**Do NOT wrap the two in an `authorized { }` container.** A container is the edit that
breaks an operator's configuration, and a sibling leaf buys the same result for nothing.

**A grouped `list` form has no natural key here, either.** The `blackhole` block
is per peer, so every entry authorizes the same neighbour and they all OR
together: there is nothing for a list key to distinguish. A grouped form earns
its place only when a rule carries a SECOND field that binds to specific
prefixes, such as a per-rule community or a per-rule action. Without that field,
a later reader gets a name nobody reads.

## Why this is not a defect

`plan/future/README.md` refuses defects. RFC 7999 Section 3.3's first condition is met
today. A peer with a community listed and no covering prefix blackholes nothing, and
`prefixes` is what the operator states. Nothing Ze does is wrong.

This work removes a maintenance cost. An operator peering with a customer who holds fifty
prefixes states fifty lines, and edits them by hand when the customer's IRR record changes.

## The design question

**`filter_irr` publishes no per-prefix authorization fact.** `handleFilterUpdate`
(`.../filter_irr/filter_irr.go`) returns one accept, reject or modify verdict for the whole
update, over the plugin text protocol. The blackhole decision asks a different question: is
THIS prefix covered by an equal or shorter prefix in THIS peer's resolved set?

Three shapes. The spec picks one and states why.

| Shape | Note |
|-------|------|
| A neutral package holds the resolved set. Both plugins read it | The precedent is fresh: `internal/component/bgp/filtertext` was extracted in `d9c724e38` so two plugins deciding on a community cannot drift. `ai/rules/plugins.md` bans a plugin's spelling in a shared package, so the package holds the SET |
| A per-prefix query joins the plugin protocol | This crosses a process boundary for each route. Measure it first. The blackhole decision runs in `blackholeRouteTypeForBest` (`.../rib/rib_bestchange.go`), called from `checkBestPathChange` before the shard lock |
| The rib plugin resolves the AS-SET itself | A second resolver, a second cache, a second timer. Almost certainly wrong. It is named here so the next reader does not re-derive why |

## What must not be lost

**Fail closed** (`ai/rules/evidence.md`). An AS-SET that never resolved, or whose refresh
failed, MUST authorize nothing. A hand-edited list cannot silently become empty. A resolver
can. **State what the operator sees when a resolution is stale or absent.** A blackhole
that stops working is an outage the operator asked for and did not get.

**Both sources compose.** An operator will want a static exception beside an AS-SET for the
bulk. State whether they union.

**The evidence tiers hold.** `RFC7999-3.3-1` is enrolled with both polarities at
unit/verify AND interop/nightly. `check_evidence_ratchet` is keyed by kind and tier, so
this work MUST reduce neither. The interop pair in
`test/interop/scenarios/bgp-rfc7999-blackhole-frr/` keeps proving the condition, whatever
supplies the prefixes.
