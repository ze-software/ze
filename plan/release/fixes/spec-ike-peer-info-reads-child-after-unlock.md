# Spec: ike-peer-info-reads-child-after-unlock

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | A data race in shipped code: `show vpn ipsec sa` during a MOBIKE move reads Child SA fields another goroutine is writing, so the listing can report a half-moved tunnel and `-race` fails the engine package. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`PeerSession.Info` (`internal/component/ike/engine/reconcile.go`) builds the
snapshot `PeerInfoMap` (`engine/register.go`) hands to every reader of the IKE
peers. It copies `child := ps.childSA` while it holds `ps.mu`, releases the
lock, and only then reads the Child SA's fields: `InboundSPI`, `OutboundSPI`,
`IfID`, `LocalAddr`, `RemoteAddr`, `Mode`, `UDPEncap`, the traffic selectors and
`ESPGroup`.

After a MOBIKE move, `migrateMobikePair` (`engine/mobike.go`) rewrites
`LocalAddr`, `RemoteAddr`, `UDPEncap`, `NATDetected` and the two UDP ports of
that same Child SA, under `ps.mu`. The reads in `Info` take no lock, so they race
that write: a listing taken during a move can pair the new local address with the
old remote one, and the race detector fails the package.

The goal: `Info` reports every Child SA field from one consistent state, read
while `ps.mu` is held, so a concurrent MOBIKE move is seen either whole or not at
all.

Found while bookkeeping `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`,
whose doctor check `espFormSharedSPIPeer` already reads the same fields inside
`ps.mu` for this reason.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`
  → Constraint: readers of IKE state outside the engine use the core leaves
  (`ipsecinventory`, `ikeprobe`); this fix stays inside `ike/engine` and changes
  no reader's contract, only when the snapshot's fields are read.
- [ ] `ai/rules/goroutine-lifecycle.md`
  → Constraint: no goroutine is added; the fix moves reads under an existing lock.

**Key insights:**
- `ps.mu` already guards `ps.childSA` and the fields a MOBIKE move rewrites.
- `espFormSharedSPIPeer` (`engine/doctor_esp_form_fast_path.go`) states the rule
  in its comment: "a pointer copied out under the lock and read after it races
  that write", and compares inside the lock, taking `peersMu` then `ps.mu`, the
  order `PeerInfoMap` takes them.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/reconcile.go` - `Info` locks `ps.mu` for the
  `childSA` and `rekeyCount` copies only, then reads every Child SA field after the
  unlock; `childRemoving` is read under `dataplaneObservation`.
- [ ] `internal/component/ike/engine/mobike.go` - `migrateMobikePair` moves the
  tunnel through `MigrateTunnel`, then writes `LocalAddr`, `RemoteAddr`,
  `UDPEncap`, `NATDetected`, `udpLocalPort` and `udpRemotePort` under `ps.mu`.
- [ ] `internal/component/ike/engine/register.go` - `PeerInfoMap` holds
  `peersMu.RLock` and calls `Info` for each active session.
- [ ] `internal/component/ike/engine/doctor_esp_form_fast_path.go` -
  `espFormSharedSPIPeer` and `espFormChildCarries` read `InboundSPI` and
  `LocalAddr` inside `ps.mu`.

**Behavior to preserve:**
- Every `PeerInfo` field keeps its meaning and its value for a session no move
  touches, `ESPTransformErr` and the negotiated-transform read
  (`child.ESPGroup.Proposals[0]`) included.
- The lock order `peersMu`, then `ps.mu`.

**Behavior to change:**
- `Info` reads the Child SA's fields while it holds `ps.mu`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The session lock | `engine/reconcile.go` `PeerSession.mu` | Guards `childSA`, `pendingChild` and the fields `migrateMobikePair` writes | Extends its use: `Info` holds it across its reads of the Child SA |
| The in-lock comparison | `engine/doctor_esp_form_fast_path.go` `espFormSharedSPIPeer`, `espFormChildCarries` | Reads a Child SA's SPI and local address inside `ps.mu`, against the same MOBIKE write | Followed: `Info` reads its fields the same way |
| The dataplane observation lock | `engine/reconcile.go` `Info`, `dataplaneObservation` | Guards `dataplaneRemoving` | Does not apply: it is a separate lock for a separate field, kept as it is |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator's `show vpn ipsec sa`, or any caller of `PeerInfoMap`.

### Transformation Path
1. `PeerInfoMap` takes `peersMu` and calls `Info` for each session.
2. `Info` reads the session and its current Child SA into a `PeerInfo` value.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | the snapshot is built in the engine's process | No |

### Changed-Wire Census

**Search evidence:** no wire changes: the fix moves reads under a lock, and the
`PeerInfo` value and every rendering of it keep their fields and format.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | - | - | the change is inside `Info` | preserve |

### Integration Points
- `PeerInfoMap` - unchanged; it calls the fixed `Info`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | same path, reads moved under the lock |
| No unintended coupling (components stay isolated) | Yes | inside `ike/engine` |
| No duplicated functionality (extends existing, does not recreate) | Yes | uses `ps.mu` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | a display snapshot |
| Registration over hardcoding, outbound | N-A | no command, view or family |
| Registration over hardcoding, inbound | N-A | no list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing `Info` calls while holding `ps.mu` takes `ps.mu` or `peersMu` again | `Info` reads fields and calls `resolveESPTransforms`, `IdentityOf` and `getESPGroup` | a deadlock under the lock | read each callee before the change | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Holding `ps.mu` longer delays the owner loop | none expected: the reads are memory copies | keep the transform resolution outside the lock, on copied values |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a deadlock in `show vpn ipsec sa`, or the race stays |
| How is it reverted? | single commit revert |
| Who else touches this path? | the MOBIKE migration and the doctor's shared-SPI check |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `PeerInfoMap` while a MOBIKE move rewrites the Child SA | → | `PeerSession.Info` | `TestPeerInfoReadsTheChildSAUnderTheSessionLock` (`internal/component/ike/engine/reconcile_test.go`, run with `-race`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `Info` called in a loop while another goroutine writes a Child SA's `LocalAddr`, `RemoteAddr` and `UDPEncap` under `ps.mu`, as `migrateMobikePair` does | every `PeerInfo` reports either the old pair of addresses or the new pair, never one of each, and the race detector reports nothing | `childRemoving` stays read under `dataplaneObservation`, because that field has its own lock |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerInfoReadsTheChildSAUnderTheSessionLock` | `internal/component/ike/engine/reconcile_test.go` | AC-1, red under `-race` before the fix | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | N/A: a race shows only under the race detector, which the unit test runs | | |

## Files to Modify
- `internal/component/ike/engine/reconcile.go` - `Info` reads the Child SA's fields while it holds `ps.mu`.
- `internal/component/ike/engine/reconcile_test.go` - `TestPeerInfoReadsTheChildSAUnderTheSessionLock`.

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | N-A | none change |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | output unchanged |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new dependency |
| Doctor code per problem class | N-A | no new problem class an operator meets |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | output unchanged |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-ike-peer-info-reads-child-after-unlock.md` lists them; the main thread runs it at design |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- write `TestPeerInfoReadsTheChildSAUnderTheSessionLock` and show it red under `-race`.
   - Tests: `TestPeerInfoReadsTheChildSAUnderTheSessionLock`
   - Files: `reconcile_test.go`
   - Verify: the race detector names the read in `Info` and the write the test makes
2. **Phase: Fix** -- read the Child SA's fields inside `ps.mu` in `Info`.
   - Tests: the same, green under `-race`
   - Files: `reconcile.go`
   - Verify: red before, green after

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 has an implementation at file:line |
| Correctness | every Child SA field `Info` reports is read inside `ps.mu`; nothing under the lock takes `peersMu` or `ps.mu` again |
| Rule: goroutine-lifecycle | no goroutine added |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No Child SA field read after the unlock in `Info` | `TestPeerInfoReadsTheChildSAUnderTheSessionLock` under `-race` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | the lock is held for memory copies only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `field-read-after-its-lock-is-released.md` | 2026-11-12 | `PeerSession.Info` reads Child SA fields after `ps.mu` is released |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read the fields inside `ps.mu` | copy the Child SA under the lock; make the fields atomic | the session lock already guards the write, and `espFormSharedSPIPeer` reads the same way; a struct copy would also copy the keys a Child SA holds |

## Known Limitations
- None known.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change
- [ ] `./le test unit pkg path ./internal/component/ike/engine/ run TestPeerInfoReadsTheChildSAUnderTheSessionLock`, which runs under the race detector

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + edited spec + the journal row marked fixed
- [ ] **Commit B:** `remove plan/release/fixes/spec-ike-peer-info-reads-child-after-unlock.md` only, in the same `./le commit create` script
