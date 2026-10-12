# Spec: an RFC 2545 condition flip leaves the peer with a next hop it cannot resolve

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-link-local-only-next-hop.md (its AC-17 designs the per-peer re-advertise this spec reuses) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An interface address event that flips RFC 2545 Section 3's shared-subnet
condition must re-advertise the prefixes that condition governs. Today it does
not, so when the condition goes FALSE a peer keeps a link-local next hop it can
no longer resolve on its own link, until the next update for that prefix
arrives.

`(*Reactor).refreshPeerLinkScopes`
(`internal/component/bgp/reactor/reactor_iface.go`) is the producer. On an
address event it snapshots the peer list, reads `network.ConnectedPrefixes()`,
and for each peer whose stored `llScope.connected` differs from that table calls
`peer.refreshForwardFactsIfLiveFrom(connected)`. It re-settles the snapshot and
deliberately re-advertises nothing. The false-to-true direction was first
recorded as harmless; since 2026-10-12 it re-advertises too (AC-2), to agree
with `spec-bgp-link-local-only-next-hop` AC-17.

This is Thomas's ruling on B-16, given on 2026-08-08 after the closing review of
the RFC 2545 work in `ec3ad9c76`: "re-advertise the affected prefixes". He gave
it after BIRD and FRR were read at source, so it is a deliberate choice to
exceed both references.

**CORRECTED 2026-08-30: this is NOT an RFC violation.** RFC 2545 Section 3 binds
the act of advertising, not re-advertising, and `rfc/short/rfc2545.md` puts
usability on the receiver. The reading Ze took is correct and is not overturned.
What the ruling adds is that a flip is itself an event owing the peer a new
advertisement, taking the RFC 4271 Section 9.2 Update-Send reading. It stays
live because Thomas ruled for it, not because the wire is wrong.

**Both references are exceeded, knowingly.** BIRD never re-advertises on an
interface event and does not evaluate the condition dynamically at all. FRR
re-announces on an address change but guards both call sites with
`!IN6_IS_ADDR_LINKLOCAL` and freezes `shared_network` at establishment, likely
because it is an update-group key. The shape to copy is FRR's per-peer forced
re-announce scoped to that peer's channels, triggered the way BIRD triggers
`channel_request_feeding` from `channel_roa_out_changed`, which is its one
refeed trigger that is neither a config nor a protocol event.

**Cost note, corrected 2026-10-12:** the note recorded with the ruling ("Ze
has no Adj-RIB-Out refeed primitive today, and building one is the bulk of
the work") is stale, but the RIB's `outboundResend` alone is not the
primitive: it replays the next hop `ribOut` recorded as an explicit address
(`rib.go`, `sentFamilyNextHop`; `rib_replay.go`), so it never re-runs the
egress next-hop decision this spec's condition feeds (round-9 B1 on
`spec-bgp-link-local-only-next-hop`). The re-send that does re-run it has
three parts, one per route source, each scoped to one peer and one family
with no End-of-RIB: forwarded routes, sent or withheld, through the forward
rail over adj-rib-in's stored copy (`request bgp adj-rib-in replay` with a
new family argument and no `wait` argument, relayed by `RelayStoredRoute`,
which applies `applyFactsNextHop` and the egress pair trimming per
destination). Because adj-rib-in trails the live forward rail and every
relayed route passes the forwarding fence as part of an initial update,
that relay runs under one per-peer re-send hold on live forwards for the
whole pass, with a cut taken after the hold is up from the sources'
received-UPDATE IDs (`receiveCut`), never the shared message counter, and
the relay starts only after each source's publication reaches it and
adj-rib-in's process has applied every prior event (`drainRecoverySources`,
`DrainEventsApplied`, the calls relay recovery already makes), both waits
sharing one 2 s budget and holding no lock, so a route replaced or
withdrawn during the pass ends as the live rail left it; the routes go in
ascending message-ID order, and the pass promises the per-source current
stored route, not the live rail's choice among sources for UPDATEs
received concurrently (a Known Limitation); a forked adj-rib-in refuses
the family replay, so it gets no forwarded re-send and the worker logs a
Warn naming the fix (that spec's R-9) (round-10 B1, I1 and round-11 B1,
I1, I2, N1 on `spec-bgp-link-local-only-next-hop`). API routes go through
`outboundResend` restricted to entries with `ribOutEntry.LocalOrigin` set
and `ribOutEntry.ConfigStatic` clear (round-10 I2),
replaying a `next-hop self` entry as `self` from a new `ribOutEntry` marker
and an explicit next hop as itself; and config and default-originate
routes through a `staticMu` sibling of `deliverStaticRouteDelta`
(`reactor/peer_static_wire.go`).
`plan/immediate/spec-bgp-link-local-only-next-hop.md` (Key Design Decisions,
"Re-advertise", and AC-17) writes that re-send for an interface address
event, run on a long-lived reactor worker rather than on the EventBus
goroutine. This spec reuses that path, and adds
only its own trigger: an RFC 2545 Section 3 condition flip.

**Resolved, 2026-10-12 (round 12 on `spec-bgp-link-local-only-next-hop`,
c166bb9f72):** the re-send shape is B, the in-place re-send above, with no
session reset (that spec's Owner decisions, "Round 12: re-send shape"). It
follows from two standing owner decisions: "match bird and frr behaviour",
following FRR where they differ, and re-advertise on change, decided from
RFC 4271 Section 9.2 and FRR's per-peer re-announce (round-7 I3 there).
FRR re-announces in place (`bgp_announce_route` per AFI/SAFI from
`bgp_interface_address_add` and `bgp_interface_address_delete`, "we do not
want the peering to bounce") and BIRD does neither, so FRR decides. Option
A, a Cease with subcode 6, is not taken. The B-16 ruling's "re-advertise
the affected prefixes" holds unchanged here, and AC-1 and AC-2 re-advertise
in place on that path.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/` - the Adj-RIB-Out and the update-send path
  → Decision: <to be filled>
  → Constraint: <to be filled>

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2545.md` - Section 3's shared-subnet condition and where it puts usability
  → Constraint: Section 3 binds the act of advertising, not re-advertising
- [ ] `rfc/short/rfc4271.md` - Section 9.2 Update-Send, the reading the ruling takes
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `(*Reactor).refreshPeerLinkScopes` reads `network.ConnectedPrefixes()`, skips a peer whose stored `llScope.connected` equals it, and otherwise calls `peer.refreshForwardFactsIfLiveFrom`; it re-advertises nothing, and `handleAddrAddedPayload` is one of its callers
- [ ] `rfc/full/rfc2545.txt` - Section 3, the shared-subnet condition itself

**Behavior to preserve:** (unless the user explicitly said to change it)
- one refresh per interface burst, never one per address event. The
  equal-table guard that gave it is replaced by
  `spec-bgp-link-local-only-next-hop` AC-17: the guard compares masked
  prefixes, which cannot see a link-local move, so the refresh now compares
  each peer's inputs unmasked, and the burst collapses on the worker's
  capacity-one channel (amended 2026-10-12 to agree with that spec)

**Behavior to change:** (only what the user asked for)
- a true-to-false flip re-advertises the prefixes the condition governs
- a false-to-true flip re-advertises them too, with the link-local now
  appended: the next hop changes, so the route is a new one in the peer's
  Adj-RIB-Out and RFC 4271 Section 9.2 owes the UPDATE ("All newly installed
  routes and all newly unfeasible routes for which there is no replacement
  route SHALL be advertised to its peers by means of an UPDATE message").
  This replaces the earlier "harmless, nothing sent" reading so the two specs
  agree (`spec-bgp-link-local-only-next-hop` AC-17, amended 2026-10-12)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an interface address added or removed event, delivered to the reactor
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ BGP reactor | interface address event payload | No |
| reactor ↔ route sources | the re-advertise `spec-bgp-link-local-only-next-hop` designs: the forward rail over adj-rib-in's stored routes (`RelayStoredRoute`), under its one per-peer re-send hold, its cut from the sources' `receiveCut`, and its publication and applied-event waits (round-10 B1, round-11 B1), RIB `outboundResend` for `LocalOrigin` entries that are not `ConfigStatic` with `self` replayed as `self` (round-10 I2), and the config-route re-send under `staticMu`; each re-runs the egress next-hop decision | No |

### Integration Points
- `peer.refreshForwardFactsIfLiveFrom` (`peer_forward_facts.go`), also the settings-apply path
- the update-send path that would carry the re-advertisement

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a refeed fires an update storm on an interface burst | update counts jump on a flap | the link-scope worker's capacity-one channel collapses a burst to one pass, and only a peer whose answer changed is re-advertised (`spec-bgp-link-local-only-next-hop` AC-17). Within a pass nothing is suppressed as unchanged: a forwarded write never is and every `outboundResend` command carries `meta["replay"]`, so a pass for a changed peer re-sends every stored route of the family (round-10 I1; Known Limitations) |
| R-2 | a per-peer refeed disturbs update-group sharing | <to be filled> | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes are re-announced to peers that did not need them, or the next hop stays wrong |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-bgp-link-local-only-next-hop`, `spec-connected-static-reach-the-locrib` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an address event that makes the shared-subnet condition false | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the shared-subnet condition goes from true to false | the prefixes it governs are re-advertised in place, with no session reset, with a next hop the peer can resolve, through `spec-bgp-link-local-only-next-hop`'s re-advertise path (its Key Design Decisions, "Re-advertise", option B of its round-12 owner decision) |
| AC-2 | the condition goes from false to true | the prefixes it governs are re-advertised in place in one pass, with no session reset, with the link-local appended after the global, through the same path as AC-1 (RFC 4271 Section 9.2; amended 2026-10-12 to agree with `spec-bgp-link-local-only-next-hop` AC-17, which re-advertises on every change of Ze's own next hop). The link-local is the egress's answer on the re-send, never a recorded one: the forward rail's pair trimming decides it for a forwarded route, `linkLocalNextHopFor` for an API route replayed as `self` or as its explicit global, and the same call for a config route; a replay of a recorded address with `nhop set` is not the path (re-checked after round-9 B1, N4) |
| AC-3 | an interface burst delivers several address events with one resulting table | one re-advertisement, not one per event |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | removes the shared subnet from the link and the peer keeps forwarding | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | `internal/component/bgp/reactor/` | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| <to be filled> | `test/interop/scenarios/` | FRR or BIRD | the peer's next hop is resolvable after the flip | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_iface.go` - <to be filled>
- the Adj-RIB-Out refeed path - <to be filled>

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| BGP family surface (new SAFI / capability / attribute) | | N-A, no new family |
| Prometheus counters/metrics | | a refeed counter, if one is added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc2545.md`, `docs/features/rfc-status.md` |
| 12 | Internal architecture changed? | | `docs/architecture/bgp/` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The
re-advertisement is an addition Thomas ordered rather than a Section 3
obligation, so the comment must say which document each line answers to.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | only the prefixes the condition governs are re-advertised |
| Rule: rfc-compliance | the code comment does not claim Section 3 requires what Thomas ordered |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| <to be filled> | <to be filled> |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- A pass is not incremental on the wire, as in
  `spec-bgp-link-local-only-next-hop` Known Limitations: it re-sends every
  stored route of the family from every source and every locally
  originated entry for a peer whose condition flipped; without ADD-PATH a
  prefix several sources hold is written once per source in ascending
  message-ID order, which matches the live rail's last write only for
  UPDATEs received one after another, never for UPDATEs received
  concurrently; and live forwards to that peer wait behind the pass, up to
  the 2 s publication and applied-event budget plus the relay, for all its
  families under one hold (round-10 I1, B1; round-11 I1, N1).
- A forked `bgp-adj-rib-in` stores MsgID 0, so the family replay refuses
  there and forwarded routes are not re-sent; the worker logs a Warn naming
  in-process `bgp-adj-rib-in` as the fix (that spec's R-9, round-11 I2).
- Where `bgp-adj-rib-in` is not loaded, or the destination's `send update`
  attachment does not name it, forwarded routes are not re-sent
  (`filterPermittedPeers`; that spec's R-8, round-10 N1).
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
