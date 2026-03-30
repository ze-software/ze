# Spec: iface-3 — BGP Reactions to Interface Events

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-1 |
| Phase | - |
| Updated | 2026-03-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-0-umbrella.md` — shared topics, payloads
3. `internal/component/bgp/reactor/reactor_bus.go` — reactor Bus subscription (from spec-reactor-bus-subscribe)
4. `internal/component/bgp/reactor/listener.go` — BGP listener management
5. `internal/component/bgp/reactor/reactor.go` — reactor core

## Task

Make the BGP reactor react to interface address events on the Bus. When `interface/addr/added` fires for an address matching a peer's `LocalAddress`, start a listener and initiate connections. When `interface/addr/removed` fires, gracefully drain sessions and remove the listener. Also extend `local-address` to accept interface names (VyOS `update-source` pattern).

## Required Reading

### Architecture Docs
- [ ] `plan/spec-iface-0-umbrella.md` — Bus topics, payload format, BGP reaction design
  → Decision: BGP subscribes to `interface/` prefix, matches against peer `LocalAddress`
  → Decision: graceful drain sends NOTIFICATION cease subcode 6
- [ ] `plan/learned/423-reactor-bus-subscribe.md` — reactor Bus subscription
  → Decision: handlers registered before Start via `OnBusEvent()`, prefix-based
  → Constraint: handlers must never hold `reactor.mu` (deadlock with `publishBusNotification`)
- [ ] `internal/component/bgp/reactor/listener.go` — current listener management
  → Constraint: `startListenerForAddressPort(addr, port, peerKey)` creates per-address listeners

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` — BGP-4: TCP connection binding (Section 8)
  → Constraint: BGP binds to specific local addresses per peer
- [ ] `rfc/short/rfc4486.md` — BGP Cease NOTIFICATION subcodes
  → Constraint: subcode 6 = "Other Configuration Change"

**Key insights:**
- Reactor already has `OnBusEvent()` API for Bus subscription (from spec-reactor-bus-subscribe)
- Listener management exists but assumes addresses always exist
- Graceful drain: stop accepting → NOTIFICATION cease → wait hold timer → force close → remove listener

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_bus.go` — `OnBusEvent()`, `Deliver()`, handler dispatch
- [ ] `internal/component/bgp/reactor/reactor.go` — `startListenerForAddressPort()`, peer management
- [ ] `internal/component/bgp/reactor/listener.go` — `Listener` struct, `net.ListenConfig`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` — peer connection management
- [ ] `internal/core/network/network.go` — `RealDialer` with `LocalAddr`

**Behavior to preserve:**
- BGP per-peer `LocalAddress` binding via `net.ListenConfig`
- Existing peer connection management
- Reactor Bus subscription API (from spec-reactor-bus-subscribe)

**Behavior to change:**
- BGP currently assumes configured IPs exist — will now wait for `addr/added` before binding
- No listener start/stop in response to interface events
- `local-address` only accepts IP strings — will also accept interface names

## Data Flow (MANDATORY)

### Entry Point
- Bus delivers `interface/addr/added` or `interface/addr/removed` event to reactor's `Deliver()` method
- Format: `ze.Event` with topic string and JSON `[]byte` payload

### Transformation Path
1. **Receive** -- reactor's `Deliver()` dispatches to `interface/` handler
2. **Decode** -- parse JSON payload to extract `address` and `unit` fields
3. **Match** -- check if address matches any peer's `LocalAddress` (or resolved interface unit)
4. **React** -- on `addr/added`: start listener + initiate connections. On `addr/removed`: drain + remove
5. **Interface unit resolution** -- if `local-address` is an interface unit (e.g., `eth0.0`), resolve to the unit's primary IP and re-resolve on events with matching `name` + `unit` metadata

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Bus ↔ Reactor | `consumer.Deliver([]Event)` via registered handler | [ ] |
| Reactor ↔ Network | `net.ListenConfig` / `net.Dialer` binding | [ ] |

### Integration Points
- `internal/component/bgp/reactor/reactor_bus.go` — `OnBusEvent("interface/", handler)`
- `internal/component/bgp/reactor/listener.go` — start/stop listeners dynamically
- `internal/component/bgp/reactor/reactor_peers.go` — peer connection initiation

### Architectural Verification
- [ ] No bypassed layers (Bus → reactor handler → listener, no direct coupling to iface plugin)
- [ ] No unintended coupling (reactor never imports `internal/plugins/iface/`)
- [ ] No duplicated functionality (extends existing listener management)
- [ ] Zero-copy preserved (payload parsed once in handler)

## BGP Subscription Design (from umbrella)

### Event Reactions

| Event | BGP Action |
|-------|------------|
| `interface/addr/added` | Match against peer `LocalAddress`. If match: start listener, attempt outbound connections |
| `interface/addr/removed` | Match against active listeners. If match: drain sessions, remove listener |
| `interface/down` | Mark peers on affected interfaces for reconnection |
| `interface/up` | Resume connection attempts for pending peers |

### Graceful Drain on Address Removal

| Step | Action | Duration |
|------|--------|----------|
| 1 | Stop accepting new connections | Immediate |
| 2 | Send NOTIFICATION (cease, subcode 6) | Immediate |
| 3 | Wait for peers to disconnect or hold timer | Up to hold time (default 90s) |
| 4 | Force-close remaining connections | After timeout |
| 5 | Remove listener | After all connections closed |

### `local-address` Enhancement

| Current | Proposed |
|---------|----------|
| IP string or `"auto"` | IP string, interface unit (`<name>.<unit>`), or `"auto"` |

When `local-address` is an interface unit reference (e.g., `eth0.0`, `eth0.100`), BGP resolves it to the unit's primary IP and re-resolves on address events. The `unit` metadata field in Bus events enables efficient matching.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Bus event `interface/addr/added` | → | BGP starts listener on that address | `TestBGPStartsListenerOnAddrAdded` |
| Bus event `interface/addr/removed` | → | BGP drains sessions on that address | `TestBGPDrainsOnAddrRemoved` |
| Config with `local-address` as interface unit | → | BGP resolves to IP | `test/plugin/iface-bgp-bind.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-4 | `interface/addr/added` event for a peer's `LocalAddress` | BGP starts listener on that address and attempts outbound connections |
| AC-5 | `interface/addr/removed` event for an active listener address | BGP sends NOTIFICATION cease (subcode 6) to peers, drains connections, removes listener |
| AC-14 | Multiple peers share same `LocalAddress` | All peers react to address add/remove events, shared listener created once |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPAddrAddedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | Listener started when matching addr event received | |
| `TestBGPAddrRemovedReaction` | `internal/component/bgp/reactor/reactor_iface_test.go` | Sessions drained when addr removed event received | |
| `TestBGPSharedListener` | `internal/component/bgp/reactor/reactor_iface_test.go` | Multiple peers share one listener for same address | |
| `TestLocalAddressInterfaceUnit` | `internal/component/bgp/reactor/reactor_iface_test.go` | Interface unit (`eth0.0`) resolved to IP address | |
| `TestAddrRemovedDrainSequence` | `internal/component/bgp/reactor/reactor_iface_test.go` | Drain follows correct 5-step sequence | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs in this phase | | | | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-bgp-bind` | `test/plugin/iface-bgp-bind.ci` | BGP session starts after interface IP added | |

### Future (if deferring any tests)
- Chaos test: rapid addr add/remove flapping — defer to chaos framework

## Files to Modify

- `internal/component/bgp/reactor/reactor.go` — register `OnBusEvent("interface/", handler)` before Start
- `internal/component/bgp/reactor/listener.go` — dynamic listener start/stop methods
- `internal/component/bgp/reactor/reactor_peers.go` — peer connection management reacts to address availability
- `internal/component/bgp/schema/ze-bgp-conf.yang` — `local-address` accepts interface unit references (`<name>.<unit>`)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (BGP update) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` — `local-address` |
| CLI commands/flags | [ ] | N/A |
| Functional test | [x] | `test/plugin/iface-bgp-bind.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — BGP interface-aware binding |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — `local-address` accepts interface names |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | No | — (covered by interfaces guide from Phase 2) |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4486.md` — cease subcode 6 |
| 10 | Test infrastructure changed? | No | — |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — interface-aware BGP |
| 12 | Internal architecture changed? | No | — |

## Files to Create

- `internal/component/bgp/reactor/reactor_iface.go` — interface event handler, addr matching, drain logic
- `internal/component/bgp/reactor/reactor_iface_test.go` — unit tests
- `test/plugin/iface-bgp-bind.ci` — functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: addr/added handler** — register Bus handler, match against peer LocalAddress, start listener
   - Tests: `TestBGPAddrAddedReaction`, `TestBGPSharedListener`
   - Files: `reactor_iface.go`, `reactor.go`
   - Verify: tests fail → implement → tests pass
2. **Phase: addr/removed handler** — graceful drain sequence
   - Tests: `TestBGPAddrRemovedReaction`, `TestAddrRemovedDrainSequence`
   - Files: `reactor_iface.go`, `listener.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: local-address interface name** — resolve interface name to IP, re-resolve on events
   - Tests: `TestLocalAddressInterfaceName`
   - Files: `reactor_iface.go`, YANG schema
   - Verify: tests fail → implement → tests pass
4. **Functional test** → `test/plugin/iface-bgp-bind.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-4, AC-5, AC-14 all have implementation with file:line |
| Correctness | NOTIFICATION uses cease subcode 6. Drain follows 5-step sequence. |
| Naming | Handler registered for `"interface/"` prefix (matches Bus topic hierarchy) |
| Data flow | Bus event → handler → match → listener start/stop (no direct iface plugin import) |
| Rule: goroutine-lifecycle | No per-event goroutines. Drain timeout via existing timer patterns. |
| Deadlock safety | Handler never holds `reactor.mu` (per reactor-bus-subscribe learned summary) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/bgp/reactor/reactor_iface.go` exists | `ls -la` |
| `internal/component/bgp/reactor/reactor_iface_test.go` exists | `ls -la` |
| Handler registered for `interface/` | `grep 'OnBusEvent.*interface' reactor.go` |
| `test/plugin/iface-bgp-bind.ci` exists | `ls -la` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | JSON payload from Bus — validate `address` field before matching |
| DoS via rapid events | Ensure drain timeout prevents resource exhaustion from rapid addr removal |
| Deadlock | Handler must not hold reactor.mu during Bus operations |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC 4486 Section 4: "Other Configuration Change" (cease subcode 6)` above NOTIFICATION send for address removal drain.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-4, AC-5, AC-14 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-iface-3-bgp-react.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
