# Spec: fixit-peer-pending-sync-settles-too-early

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`test/plugin/mup-ipv4-announce.ci` and `test/plugin/ipv6-announce-withdraw.ci` fail under CPU
oversubscription because a plugin's `quiesce()` returns before the peer's
initial sync is owed, so a withdraw joins the same initial dump and overtakes
the End-of-RIB marker.

**Measured, not inferred.** Under load the received frames were announce,
withdraw, and NO marker at all, failing as `Expected UPDATE (len=23), Received
UPDATE (len=27) WITHDRAWN`. An earlier reading of this failure had the marker
overtaking the announce; that reading is wrong and should not be carried
forward.

`Peer.PendingSync` (`internal/component/bgp/reactor/peer.go`) reports settled on
`sendingInitialRoutes` being zero and an empty operation queue, and
`DrainPeerSync` (`internal/component/bgp/reactor/reactor_api.go`) calls the peer
settled on that. A peer that is still CONNECTING, with a momentarily empty
queue, is neither down nor idle yet reports settled.

**A second, related defect is BLOCKED and must not be fixed here.**
`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
gates its hold on `apiSyncExpected`, which counts only process bindings
carrying `send [ update ]`. Nothing on the injection path reads that leaf-list,
so the bare `process X { }` idiom used by mup4, ipv4, ipv6, announce and
nexthop injects routes with no hold at all. That is a guard whose condition is
unrelated to its hazard. RFC 4724 Section 4 requires the marker to be sent once
the speaker completes its initial routing update, and a route handed to ze
before establishment belongs to that update, so a route-pushing plugin must sit
inside the barrier.

That fix was implemented, proven RED to GREEN at the owning layer, and then
REVERTED: any hold inside `sendInitialRoutes` keeps `sendingInitialRoutes`
non-zero, which widens the window where `ShouldQueue` is true, and the
forwarding rail did not consult `ShouldQueue` when this was written
(`spec-fixit-forward-rail-initial-sync-ordering`, closed 2026-08-11: both
forwarding rails now hold on `Peer.forwardOrderHold`, and the pool worker holds
its overflow with `overflowHeld`). Measured A/B on
identical builds differing only in that gate:
`test/plugin/role-otc-rs-withdraw-eor.ci` passes with the gate off and fails
with it on, delivering the same relayed route twice. Separating "initial sync
running" from "End-of-RIB not yet sent" is the shape that unblocks it.

**A ruling must be adjudicated before either half lands.**
`test/plugin/plugin-nexthop.ci` quotes Thomas, 2026-07-27: the marker is ordered only
against this speaker's own initial dump, never against routes learned
afterwards. That was applied to a PLUGIN-INJECTED route by analogy with
`role-otc-unicast-scope.ci`, where the route is FORWARDED from another peer.
The masked-verdict-and-RFC-exemption record says of that same shape
that the pattern alone is not the defect, the route's provenance is. Ze's own
design treats plugin-injected initial routes as part of the dump. Under
`ai/rules/rfc-compliance.md` that ruling is void by default and must be
re-raised rather than cited. It decides whether mup4, ipv4 and ipv6 keep their
seq-2 marker rule at all.

**Already fixed and NOT part of this spec:** the test matcher that hid this
failure. `ExpectedOrKeepalive` (`internal/test/peer/checker.go`) silently
consumed a marker that any later expectation still matched, so the failure was
reported against the wrong pair of frames. That is landed and proven.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - orientation for the owning layer
  -> Decision: [fill at design time]
  -> Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- [fill at design time]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - PendingSync, the settled predicate
- [ ] `internal/component/bgp/reactor/reactor_api.go` - DrainPeerSync, the quiescer behind quiesce()
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - sendInitialRoutes and the apiSyncExpected gate

**Behavior to preserve:**
- The landed matcher fix in `internal/test/peer/checker.go`. This spec does not revisit it.
- `test/plugin/role-otc-rs-withdraw-eor.ci` must keep passing; it is the regression that reverted the barrier fix.

**Behavior to change:**
- [fill at design time]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
[fill at design time]

### Transformation Path
[fill at design time]

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| [fill at design time] | [fill at design time] | [fill at design time] |

### Integration Points
[fill at design time]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| [fill at design time] | -> | [fill at design time] | [fill at design time] |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| [fill at design time] | [fill at design time] | a connecting peer is not reported settled |

### Functional Tests
- [ ] `test/plugin/mup-ipv4-announce.ci` and `test/plugin/ipv6-announce-withdraw.ci` under oversubscribed runs
- [ ] `test/plugin/role-otc-rs-withdraw-eor.ci` must not regress

## Files to Modify

Not yet determined. The owning file follows from the diagnosis, which is
the first job of whoever takes this spec.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Checklist

### Goal Gates (MUST pass)
- [ ] A peer that has not established is never reported settled by `quiesce()`
- [ ] The RFC 4724 ruling on plugin-injected routes is re-raised with Thomas and recorded
- [ ] No fixture was edited to match current behaviour

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-precommit-verify` green before commit
