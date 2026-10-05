# Spec: bgp-graceful-restart-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The strict RFC re-read of `spec-rfc-requirement-quote-hand-backfill`
found that Ze's Graceful Restart (RFC 4724) and Long-Lived Graceful Restart
(RFC 9494) helper keeps routes it must drop and forwards stale routes it must
depreference or hold back. An operator meets both as routing: stale routes
survive a NOTIFICATION teardown, and a route a neighbour marked LLGR_STALE is
preferred and advertised as if fresh.

Each row was confirmed open at its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 4724 Section 4 (RFC4724-4-2) | "It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message." | `internal/component/bgp/plugins/gr/gr.go::handleStructuredState` and `handleStateEvent` | Both test `reason == "notification"`, but the only producer (`internal/component/bgp/reactor/peer_run.go`, through `notifyPeerClosed`) sends "session closed" or "connection lost", so GR retains and marks stale after a NOTIFICATION teardown | weak; no journal row |
| D2 | RFC 9494 Section 4.2 (RFC9494-4.2-8) | "the F bit for a specific address family is not set in the newly received LLGR Capability, or" / "a specific address family is not included in the newly received LLGR Capability, or" | `internal/component/bgp/plugins/gr/gr_state.go::onSessionReestablished` | Keeps a family when EITHER the new GR or the new LLGR capability sets its F bit, so a family with F clear in LLGR, or absent from LLGR, is not purged when GR carries it with F set | unimplemented finding; row has no gap marker |
| D3 | RFC 9494 Section 4.3 (RFC9494-4.3-1) | "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:" and "Treat the route as the least preferred in route selection" | `internal/component/bgp/plugins/rib/bestpath.go::comparePair`, `internal/component/bgp/plugins/rib/storage/familyrib.go` insert | Only a route the helper marks itself is least preferred; a route received carrying LLGR_STALE (0xFFFF0006) is inserted fresh and competes normally. No non-test code reads the community on receipt | unimplemented finding |
| D4 | RFC 9494 Section 4.3 (RFC9494-4.3-3) | "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received." | `internal/component/bgp/plugins/gr/gr_egress.go::LLGREgressFilter` | Decides on the local stale level only, so a received LLGR_STALE route is advertised unchanged to a neighbour that never sent the LLGR capability | unimplemented finding |
| D5 | RFC 9494 Section 4.2 (RFC9494-4.2-6) with Section 4.3 (RFC9494-4.3-2) | "The LLGR_STALE community MUST NOT be removed when the route is further advertised." | `internal/component/bgp/reactor/peer_forward_facts.go::applyFactsSendCommunity`, `internal/component/bgp/plugins/filter_community` `removeValues` | The recorded 4.3-2 gap: the community can be stripped on egress, so Section 4.3 is not performed in full and 4.2-6 is claimed on one bullet | unimplemented finding; 4.3-2 carries `{gap}` |

Related: `plan/spec-gr-advanced.md` designs N-bit (RFC 8538) retention on top of
the `wasNotification` branch and assumes it works; D1 makes it work first.

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One unit test per row, red against HEAD |
| Both polarities | D1: NOTIFICATION teardown flushes, TCP loss retains. D2: each of the three bullets purges, a family with both F bits set is kept. D3/D4: received LLGR_STALE is least preferred and withheld from a non-LLGR neighbour, a fresh route is not |
| Discrimination | Every RFC-tagged unit carries a `./le rfc discriminate-record` record |
| Real entry point | `.ci` tests driving a peer through NOTIFICATION and reconnect |
| Interop | The existing GR interop scenario against FRR (`test/interop/scenarios/bgp-graceful-restart-frr`) extended with a NOTIFICATION teardown, and an LLGR scenario with a peer sending LLGR_STALE |

## Owner decisions

| Row | Question |
|-----|----------|
| - | None open: every row follows its RFC sentence and no reading is in doubt |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`, `rfc/full/rfc9494.txt` Sections 4.2 to 4.4

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go` - session-down handlers
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown`, `onSessionReestablished`
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter`
- [ ] `internal/component/bgp/reactor/peer_run.go` - the close reason strings
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - `comparePair`

**Behavior to change:** the rows above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a session teardown (NOTIFICATION or TCP loss), a reconnect OPEN, an UPDATE carrying LLGR_STALE

### Transformation Path
1. reactor close reason to the GR plugin as a structured state event
2. GR state machine retains or purges
3. RIB best path and egress filters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ GR plugin | structured state event with a close reason | No |

### Integration Points
- `notifyPeerClosed`, `apiStateObserver.OnPeerClosed`

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer sends NOTIFICATION | → | GR session-down handler | [to fill in design: `.ci`] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR peer torn down by a NOTIFICATION sent or received | routes flushed, no stale retention |
| AC-2 | reconnect during LLGR with each Section 4.2 purge condition | that family's stale routes removed |
| AC-3 | route received with LLGR_STALE from an LLGR neighbour | least preferred; not advertised to a non-LLGR neighbour; community kept on further advertisement |
| AC-4 | the `weak` verdicts of RFC4724-4-2, RFC9494-4.2-6, RFC9494-4.2-8, RFC9494-4.3-1 and RFC9494-4.3-3, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28; RFC9494-4.3-3 transfer completed 2026-10-05) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1 to D5 | [to fill in design] |

### Functional Tests
- [to fill in design]

## Files to Modify

- the producers in the defect table and their tests; `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination records and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
