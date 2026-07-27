# Spec: fixit-forward-rail-initial-sync-ordering

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/learned/1252-masked-verdict-and-rfc-exemption.md` (the session that found this)
3. Source files under Current Behavior

## Freshness check (2026-07-27)

Re-verified against the tree at `5b6d64c35`; the defect is **still live** and the
premise holds. Line numbers below have drifted since 2026-07-22, so use these:

| Spec said | Now | What it is |
|-----------|-----|------------|
| `peer.go:899-906` | `peer.go:1148` | `ShouldQueue` definition |
| `reactor_api_batch.go:106` | `reactor_api_batch.go:111` | batch announce gate |
| `reactor_api_batch.go:235` | `reactor_api_batch.go:241` | batch withdraw gate |
| `reactor_api_forward.go:58` | `reactor_api_forward.go:103` | `AnnounceEOR` gate |

Still exactly three non-test callers, all on the route-INJECTION rail. A full
`grep -rn ShouldQueue internal/component/bgp/reactor/` shows no call in
`forward_rs.go` and none in `forwardUpdateCore`, so the forwarding rail remains
ungated.

Not started: this is a hot-path ordering change in the reactor, so it needs
`make ze-race-reactor` (`ai/rules/testing.md` makes that mandatory for reactor
concurrency edits) and an independent review pass before it can close.

## Design analysis (2026-07-27)

Not implemented. This records the shape the fix must take and, more usefully,
which two cheap-looking fixes are WRONG.

**D-1. Dropping the forwarded update is NOT safe: the fix must defer, not skip.**
The tempting one-liner is "if `ShouldQueue()`, skip this destination", mirroring
the existing drop for a not-established peer. It loses data. Initial sync reads
from the RIB, so for a withdraw of prefix P arriving mid-sync:

| Sync position | Effect of dropping the forwarded withdraw | Correct? |
|---------------|-------------------------------------------|----------|
| has not yet reached P | RIB already has P withdrawn, so sync never announces it | yes, by luck |
| already sent P | nothing else will ever withdraw P | **no -- peer holds P forever** |

The second row is the reported bug reproduced by its own proposed fix, so
`ShouldQueue` cannot gate a discard here. The not-established drop is safe only
because that peer gets a full RIB replay on establishment; a mid-sync peer does
not get a second replay.

**D-2. It cannot go on `opQueue` as-is.** `PeerOp` (`peer.go:111-118`, verified)
carries `Route *rib.Route`, `NLRI nlri.NLRI`, `Subcode`, `Message` -- structured
operations only, no wire-body member. A forwarded UPDATE here is wire bytes
deliberately: not re-deriving structure per destination is the entire purpose of
the forward rail, so converting it to a `PeerOp` would undo the change this spec
descends from.

**D-3. Strict order forbids a second queue.** Two queues drained independently
lose the relative order of a queued announce and a forwarded withdraw, which is
the exact invariant being restored. One FIFO must carry both, so `PeerOp` grows
a wire variant (`PeerOpForward` holding a pooled buffer handle) rather than the
forward rail growing a parallel path.

**D-4. The open question is buffer lifetime, and it is a memory-architecture
question, not a reactor one.** A queued wire body must outlive the source peer's
read buffer, so deferring means pinning a pooled buffer for the whole
initial-sync window. `opQueueMax` defaults to `DefaultOpQueueSize` = 10000
(`peer.go`) and scales with prefix-maximum, so a COUNT-bounded queue of wire
bodies is a BYTE-unbounded pin on the forward pools. Per
`ai/rules/memory-architecture.md` the queue has to be byte-budgeted like the
global shared pool, and the overflow behaviour chosen deliberately: pool
exhaustion is the backpressure signal, and the honest options are tearing the
destination session down (it re-syncs) or ending the sync early -- never a silent
drop, which is D-1 again by another route.

Settle D-4 first. It decides whether this is a small `PeerOp` extension or a
forward-pool change, and it is the part most likely to be got quietly wrong.

## Task

The BGP forwarding rail never consults `Peer.ShouldQueue()`, so a forwarded UPDATE
can overtake a route already queued for the same peer and leave that peer holding
a stale route.

`ShouldQueue()` (`internal/component/bgp/reactor/peer.go:899-906`) returns true
while a peer is running initial route sync or still has a non-empty `opQueue`. It
exists to preserve strict insertion order of route operations. It is called from
exactly three non-test sites, all on the route-INJECTION rail:
`reactor_api_batch.go:106` (batch announce), `:235` (batch withdraw), and
`reactor_api_forward.go:58` (`AnnounceEOR`).

The FORWARDING rail consults it nowhere. Neither `reactorForwardRS`
(`internal/component/bgp/reactor/forward_rs.go`) nor `forwardUpdateCore`
(`internal/component/bgp/reactor/reactor_api_forward.go`) gates on peer
readiness: their per-destination loops filter on `forwardFacts() != nil`, export
filters, community policy and RR rules only.

`forwardFacts()` is not a readiness gate. It is a plain atomic load
(`peer_forward_facts.go:78-80`) whose snapshot is stored by `setEncodingContexts`
(`peer.go:563`) BEFORE `sendingInitialRoutes` is set and before the sync
goroutine starts, so facts are non-nil for the whole initial-sync window.

Consequence: an announce for prefix P sits in a peer's `opQueue` while a withdraw
for P arrives from another peer and is forwarded directly. The withdraw reaches
the wire first, the queued announce drains after it, and the peer ends up
believing P is live when it has been withdrawn.

**This is NOT about End-of-RIB ordering.** RFC 4724 orders the EOR only against
the speaker's own initial dump, never against routes learned later
(`rfc/short/rfc4724.md:126`). Tests asserting EOR-vs-forwarded-route order were
asserting something Ze never owed and were corrected separately; see
`plan/learned/1252-masked-verdict-and-rfc-exemption.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor and forwarding overview
  → Constraint: (fill during design)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - UPDATE semantics and route replacement
  → Constraint: (fill during design) -- a withdraw and a later announce of the
    same prefix are order-sensitive; the receiver applies them as delivered.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go:891-918` - `ShouldQueue`/`PendingSync`
  → Constraint: `ShouldQueue` is true while `sendingInitialRoutes != 0` or the
    `opQueue` is non-empty; it also gates on state being Established.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - RS fast path
  → Constraint: `tryDirectWriteNoFlush` gates on session non-nil, FSM
    Established, and `writeMu.TryLock()`. No readiness check.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore`
  → Constraint: per-destination gates are facts-nil, community policy, RR rules,
    export filters and wire-rewrite failure. No readiness check.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go:78-80` - facts load
  → Constraint: non-nil from `setEncodingContexts` until teardown, therefore
    non-nil throughout initial sync.
- [ ] `internal/component/bgp/reactor/peer.go:111-118` - `PeerOp`
  → Constraint: `opQueue` holds STRUCTURED ops (`Route`, `NLRI`), not wire
    bodies, so a forwarded wire UPDATE cannot simply be queued there.

**Behavior to preserve:**
- The fast path must not block: it runs on the SOURCE peer's read goroutine, so
  waiting there stalls forwarding to every other destination of that source.
- `HoldWrites` must stay short: `writeMu` also gates KEEPALIVE
  (`session_write.go` `writeMessage`), so a long hold risks the hold timer.
- Forwarded updates to a genuinely not-established peer are DROPPED today, not
  deferred; that is deliberate (the peer gets a RIB replay on establishment).

**Behavior to change:**
- Only the ordering hazard above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A peer is Established but still draining its initial sync when an UPDATE arrives
from a different peer for a prefix already queued to it.

### Transformation Path
See "Design analysis" under the Freshness check above: D-1 to D-4 establish that
the forwarded update must be deferred on the SAME FIFO as `opQueue`, and that
buffer lifetime is the open question.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| source read goroutine ↔ destination session | `tryDirectWriteNoFlush` writes directly under `writeMu.TryLock()` | [ ] |
| forward pool worker ↔ destination session | `fwdBatchHandler` takes a blocking `writeMu.Lock()` | [ ] |
| initial-sync goroutine ↔ destination session | `opQueue` drain under `p.mu` | [ ] |

### Integration Points
- `internal/component/bgp/reactor/forward_rs.go`
- `internal/component/bgp/reactor/reactor_api_forward.go`

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| announce queued for a peer in initial sync, then a withdraw for the same prefix forwarded | -> | forwarding rail honours insertion order | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer P is Established and mid-initial-sync with an announce for prefix X queued; a withdraw for X is forwarded from another peer | P receives announce then withdraw, in that order; P does not end holding X |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `forward-order-during-initial-sync` | `test/plugin/forward-order-during-initial-sync.ci` (to create) | a route withdrawn while a peer is still draining its initial sync does not survive on that peer: the peer's wire shows announce then withdraw, never the reverse | |

## Files to Modify
- (fill during design)

## Implementation Steps

### Implementation Phases
1. Reproduce AC-1 as a failing test.
2. Design the readiness gate (note: `opQueue` cannot hold wire bodies, and
   blocking the fast path is not an option -- see Behavior to preserve).
3. Implement, then `make ze-race-reactor` (reactor concurrency change).

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` passes
