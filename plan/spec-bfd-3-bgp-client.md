# Spec: bfd-3-bgp-client

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-bfd-2-transport-hardening |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `.claude/rules/plugin-design.md` -- plugin boundary naming, DispatchCommand contract
4. `.claude/rules/config-design.md` -- YANG grouping vs augment
5. `plan/learned/555-bfd-skeleton.md`, `plan/learned/556-bfd-1-wiring.md`, the Stage 2 learned summary once written
6. `rfc/short/rfc5882.md` -- BFD client semantics (THE RFC for this spec)
7. `docs/guide/bfd.md` -- sketched `bgp peer { bfd { ... } }` UX
8. Source files: `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/session*.go`, `internal/component/bgp/config/peers.go`, `internal/component/bgp/schema/ze-bgp-conf.yang`, `internal/plugins/bfd/api/service.go`, `internal/plugins/bfd/api/events.go`

## Task

Stage 3 wires the BGP reactor to the BFD Service so that a peer block can opt into BFD liveness detection. Today the BFD plugin exists (Stage 1), its transport is production-hardened (Stage 2), but no BGP peer ever calls `Service.EnsureSession`. Operators cannot turn on BFD for BGP; dead-peer detection still relies exclusively on the BGP hold-time (default 90 s). Sub-second peer teardown is the whole point of BFD on BGP.

RFC 5882 §2 describes the client contract: a BGP peer registered with BFD subscribes to state changes and, on receiving a Down event, tears the session down with diagnostic `Hold Timer Expired` (or a ze-specific equivalent) **without waiting for the BGP hold timer**. On Up, the BGP FSM continues its normal lifecycle.

Stage 3 delivers:

1. YANG augment (`ze:augment`) adding a `bfd { ... }` container under `bgp peer connection` (single-hop) and `bgp peer { bfd { ... } }` (mode selector: single-hop vs multi-hop, inherited profile, min-ttl).
2. Config plumbing from tree → `reactor.Peer` → new `bfdConfig` field.
3. Reactor-side client: when a peer starts, if BFD is configured, call `Service.EnsureSession` with a `SessionRequest` built from the peer's `(peer, local, interface, vrf, mode, min-ttl, profile overrides)`. Subscribe to state changes, teardown on Down, release on peer stop.
4. Service discovery: the BGP plugin must obtain `api.Service` without importing `bfd/engine`. Stage 3 introduces a registry contract -- the BFD plugin registers its Service in a lookup table, the BGP plugin fetches it via a minimal interface shim.
5. `.ci` functional test: two ze processes, one as BGP/BFD peer, one as BGP/BFD local. Bring the BFD session up, kill the peer, assert the BGP session tears down in < 2 seconds (vs 90 s hold timer).
6. FRR `bfdd` interop scenario under `test/interop/scenarios/`: two netns, ze on one side, FRR on the other, BGP + BFD on both. Assert the same sub-second failover.

**Service discovery design (key decision):**

The BGP plugin MUST NOT import `internal/plugins/bfd/engine`. Stage 3 introduces a lightweight `api.ServiceLookup` type that the BFD plugin registers at startup. The BGP plugin imports `internal/plugins/bfd/api` only (already public) and calls `api.Lookup("default")` or similar. Concretely: a package-level `atomic.Pointer[Service]` in `api`, set by `bfd.RunBFDPlugin` and read by any client. Same-process only (external BGP plugins don't need this yet; they would use DispatchCommand text protocol when that use case arrives).

→ Decision: in-process lookup via `api.SetService`/`api.GetService`. Rejected: `DispatchCommand` text protocol (too noisy for the hot path), registry in `internal/component/plugin/registry` (pulls BGP into the registry surface). The api package is intentionally tiny and this is exactly what it is for.

**Explicitly out of Stage 3 scope:**

- Non-BGP clients (OSPF, static routes). When they land they use the same `api.Service` surface with zero plumbing change to BFD.
- Operator CLI (`show bfd sessions`) -- `spec-bfd-4-operator-ux`.
- Auth -- `spec-bfd-5-authentication`.
- Echo mode -- `spec-bfd-6-echo-mode`.
- External (forked) BGP plugin BFD support. Requires a DispatchCommand protocol shim; tracked as a new deferral `spec-bfd-3b-external-bgp-bfd`.

→ Constraint: the existing BGP peer lifecycle (Idle → Connect → OpenSent → OpenConfirm → Established → Idle) is the stable contract. Stage 3 inserts BFD as a parallel observer that can *trigger* a transition to Idle, but never blocks or alters the existing transitions driven by the BGP FSM.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/bfd.md` -- BFD design, Service contract
  → Constraint: client contract is RFC 5882 §2. BGP is the first real client; the contract must hold.
- [ ] `docs/architecture/core-design.md` -- component boundaries
  → Constraint: BGP must not import `bfd/engine`.
- [ ] `.claude/rules/plugin-design.md` -- plugin boundary naming, DispatchCommand
  → Constraint: no "bgpBFDClient" type. The helper is `bfdClient` living under `reactor/`, opaque to the rest of the reactor.
- [ ] `.claude/rules/config-design.md` -- YANG grouping vs augment rules
  → Decision: use `ze:augment` (cross-component) to add `bfd { ... }` under `bgp peer`. A grouping would force the BGP schema to import ze-bfd-conf, which is backwards for a plugin-owned feature.
- [ ] `.claude/rules/go-standards.md` -- env var registration
  → Constraint: no new env vars for Stage 3; BFD behaviour is config-driven.

### RFC Summaries

- [ ] `rfc/short/rfc5882.md` -- BFD for BGP application contract
  → Constraint: Section 2 -- on BFD Down, the BGP session MUST be declared down without waiting for hold timer.
  → Constraint: Section 3.1 -- BFD liveness is independent of the BGP hold timer; BGP session may still be declared down via hold timer if BFD is not configured or not yet Up.

### Source files

- [ ] `internal/component/bgp/reactor/peer.go` -- Peer struct and lifecycle
  → Constraint: Run() is the canonical lifecycle; BFD client plumbing belongs here.
- [ ] `internal/component/bgp/reactor/session*.go` -- FSM and state transitions
  → Constraint: a BFD Down must drop the session the same way a hold-timer expiry does -- reuse the existing teardown path.
- [ ] `internal/component/bgp/config/peers.go` -- tree → PeerConfig
  → Constraint: extend PeerConfig with a `BFD *BFDConfig` field; builder copies from tree.
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer container shape
  → Constraint: the augment target is `/bgp/peer/connection` (or wherever `local/remote` live); verify path during implementation.

## Current Behavior (MANDATORY)

**Source files read:** (filled during /implement after Stage 2 lands)

- [ ] `internal/component/bgp/reactor/peer.go`
- [ ] `internal/component/bgp/reactor/session.go`
- [ ] `internal/component/bgp/config/peers.go`
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang`
- [ ] `internal/plugins/bfd/api/service.go` / `events.go`
- [ ] `internal/plugins/bfd/bfd.go` -- where to publish the Service

**Behavior to preserve:**

- BGP peer state machine transitions are untouched.
- BGP hold timer still fires; BFD is additive, not a replacement.
- Existing peer configs without a `bfd { }` block behave exactly as today.
- BGP reactor tests that don't touch BFD stay green.

**Behavior to change:**

- A peer with `bfd { ... }` obtains a `SessionHandle` during `Peer.Run` startup, subscribes to state changes, tears down on Down, releases the handle on peer stop.

## Data Flow

### Entry Point

- YANG config at `bgp peer { connection { bfd { ... } } }` (augmented).
- Reactor startup calls into `bfdClient.start(peer)` which calls `api.GetService().EnsureSession(req)`.

### Transformation Path

1. Config parse: tree → `PeerConfig.BFD` (new `*BFDConfig` field).
2. Reactor startup: `Peer.Run` detects `cfg.BFD != nil`, constructs `SessionRequest`, calls `api.GetService().EnsureSession`.
3. Reactor subscription: `handle.Subscribe()` returns a channel; a dedicated goroutine reads state changes and injects a `bfdDown` event into the Peer's existing event loop.
4. Peer teardown: when the event loop receives `bfdDown`, it runs the same teardown path as hold-timer expiry.
5. Peer stop: `handle.Unsubscribe` and `Service.ReleaseSession` release the session.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ BGP reactor | `PeerConfig.BFD` struct | [ ] |
| BGP reactor ↔ BFD api | `api.GetService()` + `EnsureSession`/`Subscribe` | [ ] |
| BFD state channel ↔ Peer event loop | dedicated subscriber goroutine bridges channels | [ ] |

### Integration Points

- `api.SetService` called from `bfd.RunBFDPlugin` Start.
- `api.GetService` called from `bgp.reactor.Peer.Run`.

### Architectural Verification

- [ ] No bypassed layers: BGP uses Service contract only
- [ ] No unintended coupling: `internal/component/bgp/` does not import `internal/plugins/bfd/engine`
- [ ] No duplicated functionality: peer teardown reuses the existing hold-timer path
- [ ] Zero-copy preserved: N/A (no wire encoding changes)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `bgp peer P { connection { bfd { enabled true; mode single-hop } } }` | → | `reactor.Peer.startBFDClient` calls `api.GetService().EnsureSession` | `test/plugin/bgp-bfd-failover.ci` |
| BFD Down event | → | `Peer.eventLoop` processes `bfdDown`, tears down BGP session | `test/plugin/bgp-bfd-failover.ci` (same test, asserts sub-2s BGP teardown) |
| Peer stop | → | `reactor.Peer.stopBFDClient` calls `Service.ReleaseSession` | `internal/component/bgp/reactor/peer_bfd_test.go` (fake Service asserts Release called once) |
| FRR bfdd interop | → | Two netns, ze↔FRR, BGP+BFD both sides, assert sub-second failover | `test/interop/scenarios/bgp-bfd-frr/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer config has no `bfd` block | Behaviour identical to today; no `EnsureSession` call; no goroutine spawned |
| AC-2 | Peer config has `bfd { enabled true }` with defaults | Reactor calls `EnsureSession` at startup; handle stored on Peer |
| AC-3 | BFD session reaches Up while BGP is Established | No change to BGP state; informational log "BFD up for peer X" |
| AC-4 | BFD session goes Down while BGP is Established | BGP session tears down within one event-loop tick; NOTIFICATION sent if the TCP is still alive; peer state transitions to Idle |
| AC-5 | BGP session tears down for another reason (e.g., operator shutdown) | `ReleaseSession` is called exactly once; no leaked handle |
| AC-6 | Two peers sharing the same BFD key (same local/peer/vrf/mode) | Refcount bumps; a single BFD session serves both; releasing one peer leaves the session up |
| AC-7 | Peer config reloads `bfd { enabled false }` | Reactor releases the handle cleanly; subsequent Down events no longer teardown BGP |
| AC-8 | `api.GetService()` returns nil because BFD plugin not loaded | Peer startup logs a warning, skips BFD wiring, BGP proceeds without BFD (not a fatal error) |
| AC-9 | FRR bfdd interop | Sub-second BGP failover when FRR's BFD link drops |
| AC-10 | YANG augment validates | `ze config validate` accepts `bgp peer { connection { bfd { ... } } }`; rejects unknown keys inside the block |
| AC-11 | `plan/deferrals.md` row `spec-bfd-3-bgp-client` | Marked `done` pointing to the learned summary |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerStartWithoutBFDConfig` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-1 | |
| `TestPeerStartCallsEnsureSession` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-2: fake Service counts EnsureSession calls | |
| `TestPeerTeardownOnBFDDown` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-4: fake Service emits StateChange{Down}, assert peer transitions to Idle | |
| `TestPeerReleasesOnStop` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-5 | |
| `TestTwoPeersShareSession` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-6 (in-memory Service counts refcounts) | |
| `TestPeerReloadDisablesBFD` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-7 | |
| `TestPeerHandlesNilService` | `internal/component/bgp/reactor/peer_bfd_test.go` | AC-8 | |
| `TestAPISetGetService` | `internal/plugins/bfd/api/service_test.go` | `SetService`/`GetService` concurrent-safe via atomic.Pointer | |
| `TestConfigBFDBlockParses` | `internal/component/bgp/config/peers_test.go` | AC-10 (positive): a valid bfd block produces a non-nil BFDConfig | |
| `TestConfigBFDUnknownKey` | `internal/component/bgp/config/peers_test.go` | AC-10 (negative): unknown keys rejected with "closest match" suggestion | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| mode | single-hop/multi-hop | "multi-hop" | invalid string | N/A |
| min-ttl (multi-hop) | 1-255 | 255 | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-bfd-failover` | `test/plugin/bgp-bfd-failover.ci` | Two ze processes, one peer each, BFD single-hop over loopback; kill peer; assert BGP session drops in < 2 s. | |
| `bgp-bfd-shared-session` | `test/plugin/bgp-bfd-shared-session.ci` | Two peers on same loopback share one BFD session; one peer leaves; assert BFD stays up for the other. | |
| `bgp-bfd-frr-interop` | `test/interop/scenarios/bgp-bfd-frr/` | Namespace-based scenario with FRR `bfdd` + `bgpd` on one side, ze on the other. | |

### Future
- None deferred.

## Files to Modify

- `internal/plugins/bfd/api/service.go` -- add `SetService` / `GetService` using `atomic.Pointer[Service]`
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add `ze:augment` for `bfd { ... }` under peer connection
- `internal/component/bgp/config/peers.go` -- parse the new block into `PeerConfig.BFD`
- `internal/component/bgp/reactor/peer.go` -- `startBFDClient`/`stopBFDClient` helpers
- `internal/component/bgp/reactor/peer_bfd.go` -- new file, BFD client glue on a peer (or merged into peer.go if short)
- `internal/plugins/bfd/bfd.go` -- publish Service via `api.SetService` on `OnStarted`
- `plan/deferrals.md` -- close Stage 3 row

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `internal/component/bgp/schema/ze-bgp-conf.yang` + corresponding config-resolve changes |
| CLI commands/flags | [ ] No | - |
| Editor autocomplete | [ ] Yes (automatic via YANG) | - |
| Functional test | [ ] Yes | `test/plugin/bgp-bfd-failover.ci`, `test/plugin/bgp-bfd-shared-session.ci` |
| FRR interop | [ ] Yes | `test/interop/scenarios/bgp-bfd-frr/` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` -- add "BFD fast failover for BGP peers" |
| 2 | Config syntax changed? | [ ] Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command changed? | [ ] No | - |
| 4 | API/RPC changed? | [ ] No | - |
| 5 | Plugin changed? | [ ] Yes | `docs/guide/plugins.md` -- BFD consumer docs |
| 6 | User guide page? | [ ] Yes | `docs/guide/bfd.md` -- flip the "future-state" block to real syntax; `docs/guide/bgp.md` -- add BFD example |
| 7 | Wire format? | [ ] No | - |
| 8 | Plugin SDK/protocol? | [ ] No | - |
| 9 | RFC behavior? | [ ] Yes | `rfc/short/rfc5882.md` -- BGP client now implemented |
| 10 | Test infrastructure? | [ ] Yes (FRR interop scaffold) | `docs/functional-tests.md` |
| 11 | Daemon comparison? | [ ] Yes | `docs/comparison.md` -- BFD for BGP row |
| 12 | Internal architecture? | [ ] Yes | `docs/architecture/bfd.md`, `docs/architecture/bgp/peer-lifecycle.md` if it exists |
| 13 | Route metadata? | [ ] No | - |

## Files to Create

- `internal/component/bgp/reactor/peer_bfd.go` -- peer↔BFD client glue
- `internal/component/bgp/reactor/peer_bfd_test.go` -- unit tests with a fake Service
- `internal/plugins/bfd/api/service_test.go` -- test for `SetService`/`GetService`
- `test/plugin/bgp-bfd-failover.ci`
- `test/plugin/bgp-bfd-shared-session.ci`
- `test/interop/scenarios/bgp-bfd-frr/` -- scenario directory (layout follows existing scenarios under `test/interop/scenarios/`)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist |
| 6. Fix issues | Fix review findings |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist |
| 10. Security review | Security Review Checklist |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: api.SetService/GetService** -- atomic.Pointer[Service] in `bfd/api`.
   - Tests: `TestAPISetGetService`
   - Files: `internal/plugins/bfd/api/service.go`, `internal/plugins/bfd/api/service_test.go`
2. **Phase: Publish Service from plugin** -- `bfd.RunBFDPlugin` calls `api.SetService` in `OnStarted`, clears in deferred cleanup.
   - Tests: manual (exercised by Phase 5 functional test)
   - Files: `internal/plugins/bfd/bfd.go`
3. **Phase: YANG augment** -- add `bfd { enabled, mode, profile, min-ttl, interface }` under peer connection; config-resolve extension.
   - Tests: `TestConfigBFDBlockParses`, `TestConfigBFDUnknownKey`
   - Files: `internal/component/bgp/schema/ze-bgp-conf.yang`, `internal/component/bgp/config/peers.go`
4. **Phase: Peer BFD client** -- `startBFDClient`, `stopBFDClient`, subscriber goroutine, event injection.
   - Tests: `TestPeerStartWithoutBFDConfig`, `TestPeerStartCallsEnsureSession`, `TestPeerTeardownOnBFDDown`, `TestPeerReleasesOnStop`, `TestPeerReloadDisablesBFD`, `TestPeerHandlesNilService`, `TestTwoPeersShareSession`
   - Files: `internal/component/bgp/reactor/peer_bfd.go`, `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/peer_bfd_test.go`
5. **Phase: Functional tests** -- two `.ci` tests driving real daemons.
6. **Phase: FRR interop scenario** -- namespace scaffolding, BFD + BGP config for both sides, pass criteria.
7. **Phase: Docs** -- update every file from the Documentation checklist.
8. **Phase: Close spec** -- audit tables, learned summary, close deferral row.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Completeness | Every AC has implementation at a file:line |
| Correctness | BFD Down truly tears BGP in < 2 s (assert in functional test) |
| Naming | `bfdClient`, not `bgpBFDClient`; `startBFDClient`, not `setupBFD` |
| Data flow | BGP ↔ BFD via `api.Service` only; no imports of `bfd/engine` |
| Rule: no-layering | No "BFD disabled flag" feature flag path -- if config says enabled, it enables |
| Rule: plugin-design | Boundary naming respected (`dispatchCommand`, not `dispatchRIBCommand`) -- not applicable since we are NOT using DispatchCommand; this is an in-process Service |
| Rule: sibling call-site audit | Every call site of `Peer.teardown` / `Peer.stop` audited -- BFD Down must reuse the same path |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| `api.SetService`/`GetService` implemented | `grep -n 'SetService\|GetService' internal/plugins/bfd/api/service.go` |
| YANG augment validates | `bin/ze config validate <sample config with bfd block>` succeeds |
| Peer reacts to BFD Down | `bgp-bfd-failover.ci` passes |
| Refcount sharing works | `bgp-bfd-shared-session.ci` passes |
| FRR interop | `test/interop/scenarios/bgp-bfd-frr/` passes |
| No forbidden imports | `grep -rn 'bfd/engine' internal/component/bgp/` returns empty |
| Docs updated | Each file from Documentation table has a diff |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | YANG enforces mode enum, min-ttl range; parser rejects unknown keys |
| Resource exhaustion | BFD subscriber goroutine exits on peer stop; no goroutine leak on reload |
| Hold-timer interaction | Hold timer still fires; BFD is additive |
| Failure isolation | A failing BFD subscription never blocks the BGP FSM |

### Failure Routing

| Failure | Route to |
|---------|----------|
| FRR scenario flakes | Investigate; do not disable. Lower jitter bounds or increase detection multiplier if FRR is slow |
| Reactor race under peer reload | `make ze-race-reactor` + fix |
| Config parse accepts unknown key | Fix rejection in `peers.go` |
| 3 fix attempts fail | STOP, ask user |

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

- In-process `atomic.Pointer[Service]` picked over DispatchCommand text protocol because BGP↔BFD is a hot path. External (forked) BGP plugin BFD support is deferred to a new `spec-bfd-3b-external-bgp-bfd`.
- Subscriber goroutine per peer is acceptable (per-session lifecycle, not per-event).

## RFC Documentation

- `// RFC 5882 Section 2: "Once BFD has detected a failure of the forwarding path, the BGP session SHOULD be declared down..."` above the teardown path.

## Implementation Summary

### What Was Implemented
- (filled during /implement)

### Bugs Found/Fixed
- (filled during /implement)

### Documentation Updates
- (filled during /implement)

### Deviations from Plan
- (filled during /implement)

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes (includes `make ze-test` -- lint + all ze tests)
- [ ] Feature code integrated
- [ ] FRR interop test passes
- [ ] Docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC 5882 annotations added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (external BGP plugin support deferred)
- [ ] Single responsibility
- [ ] Explicit

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests present
- [ ] Functional tests present

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary included in commit
