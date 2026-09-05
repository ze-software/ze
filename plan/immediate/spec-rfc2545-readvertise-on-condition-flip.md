# Spec: an RFC 2545 condition flip leaves the peer with a next hop it cannot resolve

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

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
deliberately re-advertises nothing. The false-to-true direction is harmless.

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

**Cost note recorded with the ruling:** Ze has no Adj-RIB-Out refeed primitive
today, and building one is the bulk of the work.

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
- the equal-table guard that stops an interface burst rebuilding a peer's scope once per address
- the false-to-true direction, which is harmless

**Behavior to change:** (only what the user asked for)
- a true-to-false flip re-advertises the prefixes the condition governs

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
| reactor ↔ Adj-RIB-Out | a refeed primitive that does not exist today | No |

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
| R-1 | a refeed primitive fires an update storm on an interface burst | update counts jump on a flap | the equal-table guard already collapses a burst |
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
| AC-1 | the shared-subnet condition goes from true to false | the prefixes it governs are re-advertised with a next hop the peer can resolve |
| AC-2 | the condition goes from false to true | no re-advertisement is sent |
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
