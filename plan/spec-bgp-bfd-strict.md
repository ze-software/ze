# Spec: bgp-bfd-strict

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

Anchor refresh (2026-07-22 plan review, design unchanged, feature not
landed; citations below updated in-body): `BFDSettings` struct now
`peer_settings.go` (`MultiHop` at `:199`). `peer_bfd.go` is still a
pure failure detector with no `Strict`/`HoldTime`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/reactor/peer_bfd.go` - current BFD/BGP integration
4. `internal/component/bgp/reactor/peer_settings.go` - `BFDSettings`
5. `internal/component/bfd/` - BFD plugin session API

## Task

Today Ze's BFD-for-BGP is a **failure detector only**: the BFD session is opened
*after* the BGP session already reaches Established, and BFD Up/Init transitions
are ignored. A peer whose control plane is reachable but whose forwarding path is
broken can therefore establish BGP and black-hole traffic until the BGP hold timer
(seconds) fires.

Add **BFD strict mode** for a BGP peer: gate BGP session establishment on BFD
being Up. In strict mode the peer must not be brought to Established (or must be
held down) until the BFD session to that neighbour is Up, optionally bounded by a
`hold-time` after which the peer gives up waiting and follows normal behaviour.

- `bfd strict` (valueless): require BFD Up before the BGP session is considered up.
- `bfd strict hold-time <sec>` (optional): maximum time to wait for BFD Up before
  falling back.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor/FSM and plugin service discovery
  → Constraint: the BGP reactor reaches the BFD plugin via `api.GetService()` (in-process), never by importing the BFD package.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 5880/5882 (BFD) - BFD state machine (Up/Init/Down/AdminDown).
- [ ] RFC 9384 - BGP Cease NOTIFICATION subcodes; subcode 10 ("BFD Down") is already used on teardown (`peer_bfd.go`).
  → Constraint: strict mode changes *when* the BGP session is allowed up; it does not change the wire OPEN exchange.

**Key insights:**
- BFD strict is a local establishment policy. No capability is negotiated with the peer.
- The BFD plugin already exposes a session with a subscribable state channel; strict mode consumes Up (not only Down) transitions.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `startBFDClient` is "Called from the FSM callback on StateEstablished" (peer_bfd.go); `runBFDSubscriber` acts only on Down/AdminDown and explicitly ignores Up/Init: "BFD is a failure detector, not a session driver" (peer_bfd.go). Down triggers `Teardown` with `NotifyCeaseBFDDown` (peer_bfd.go).
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `BFDSettings` has only `Enabled`, `MultiHop`, `Profile`, `MinTTL`, `Interface` (peer_settings.go, `MultiHop` at :199); no `Strict`/`HoldTime`.
- [ ] `internal/component/bfd/` - session API surfaced via `api.GetService()`, `EnsureSession`, `Subscribe` (used at peer_bfd.go).

**Behavior to preserve:**
- Default (non-strict) BFD remains a pure failure detector: session opens post-Established, only Down tears down.
- Teardown on BFD Down keeps RFC 9384 Cease subcode 10 (`peer_bfd.go`).
- If the BFD plugin is not loaded, a BFD-configured peer still runs (current warn-and-continue at `peer_bfd.go`) — except that in strict mode this is a config/runtime error surfaced to the operator, not a silent continue.

**Behavior to change:**
- When `strict` is set, open the BFD session earlier (before declaring the peer up) and hold the BGP session down until BFD is Up (or hold-time elapses).

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves under the peer `bfd` container: `bfd strict` and `bfd strict hold-time <sec>`, in the BGP component/plugin YANG (same block that yields today's `bfd { profile, ... }`).
- Resolved via File → Tree → `ResolveBGPTree()` → `PeersFromTree()` into `BFDSettings`.

### Transformation Path
1. YANG `bfd strict [hold-time N]` parsed into new `BFDSettings.Strict bool` / `BFDSettings.HoldTime uint32`.
2. On peer bring-up, if `Strict`, the reactor opens the BFD session (via `api.GetService().EnsureSession`) *before* allowing the FSM to settle at Established, arming a hold-time timer.
3. The subscriber now also handles Up: on BFD Up, the peer is allowed to reach/announce Established.
4. If hold-time elapses without BFD Up, fall back to normal establishment (documented, deterministic behaviour).
5. Steady state: an established strict peer still tears down on BFD Down exactly as today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Reactor | YANG `strict`/`hold-time` → `BFDSettings` via `PeersFromTree` | [ ] |
| Reactor ↔ BFD plugin | `api.GetService().EnsureSession` opened earlier in strict mode | [ ] |
| BFD ↔ FSM | Up transition permits Established; Down still tears down | [ ] |

### Integration Points
- `startBFDClient` (`peer_bfd.go`) - split so the session can be opened pre-Established in strict mode.
- `runBFDSubscriber` (`peer_bfd.go`) - handle Up/Init in strict mode.
- `BFDSettings` (`peer_settings.go`) - add `Strict`, `HoldTime`.
- FSM Established gate - consult BFD-up state for strict peers.

### Architectural Verification
- [ ] No bypassed layers (BFD reached only via `api.GetService()`)
- [ ] No unintended coupling (BGP reactor does not import the BFD package)
- [ ] No duplicated functionality (reuse existing session/subscription; add a gate)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — strict state surfaced through existing peer status, not a new field bolted onto a core struct outside `BFDSettings`.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The BFD session can be opened before the BGP FSM reaches Established | `EnsureSession` is not gated on BGP state (peer_bfd.go) | strict mode needs deeper FSM changes | read `EnsureSession` + FSM bring-up during audit | unvalidated |
| A-2 | There is a single FSM point where "declare peer up" can consult BFD state | reactor FSM | gate is invasive if dispersed | trace FSM Established transition | unvalidated |
| A-3 | Operators want fallback-after-hold-time, not permanent hold | operational norm; matches `hold-time` framing | permanent-hold semantics differ | design confirmation with user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Opening BFD before Established races with multi-hop path/TTL setup | strict peer never sees BFD Up | derive BFD params from settings already available pre-Established; unit test the ordering |
| R-2 | Strict peer with BFD plugin absent hangs forever | peer stuck not-up | strict + no BFD plugin = doctor check failure + explicit config error, not silent continue |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set protocols bgp neighbor X bfd strict` | → | reactor opens BFD pre-Established, gates on Up | `test/plugin/bgp-bfd-strict.ci` |
| BFD reaches Up | → | strict peer allowed to Established | `test/plugin/bgp-bfd-strict.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bfd strict`, BFD never comes Up, no hold-time | BGP peer held not-Established |
| AC-2 | `bfd strict`, BFD reaches Up | BGP peer proceeds to Established |
| AC-3 | `bfd strict hold-time 5`, BFD not Up after 5s | peer falls back to normal establishment |
| AC-4 | strict peer Established, BFD goes Down | peer torn down with Cease subcode 10 (unchanged) |
| AC-5 | `bfd strict` with BFD plugin not loaded | config verify / doctor surfaces an error; peer not silently run |
| AC-6 | no `strict` leaf | current failure-detector behaviour unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables `bfd strict` on a peer with a broken forwarding path | config → BFD opened early → BFD never Up → BGP held down | `test/plugin/bgp-bfd-strict.ci` |
| 2 | forwarding path recovers, BFD comes Up | subscriber Up → FSM allowed to Established | `test/plugin/bgp-bfd-strict.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBFDStrictHoldsUntilUp` | `internal/component/bgp/reactor/peer_bfd_test.go` | strict peer not Established until BFD Up | |
| `TestBFDStrictHoldTimeFallback` | `internal/component/bgp/reactor/peer_bfd_test.go` | fallback after hold-time | |
| `TestBFDStrictTeardownOnDown` | `internal/component/bgp/reactor/peer_bfd_test.go` | Down still tears down with subcode 10 | |
| `TestBFDSettingsStrictParse` | `internal/component/bgp/.../config_test.go` | YANG strict/hold-time → BFDSettings | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hold-time | 1-4294967295 (design may narrow) | max | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-bfd-strict` | `test/plugin/bgp-bfd-strict.ci` | strict peer gated on BFD Up, falls back on hold-time | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-bfd-strict-peer` | `test/interop/scenarios/` | FRR (bfdd) | Ze holds BGP down until BFD Up against a real BFD peer | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/reactor/peer_bfd.go` - open BFD early in strict mode; handle Up
- `internal/component/bgp/reactor/peer_settings.go` - add `Strict`, `HoldTime` to `BFDSettings`
- `internal/component/bgp/reactor/` - FSM Established gate consulting BFD-up state
- BGP config resolution (`internal/component/bgp/config/`) - parse `bfd strict [hold-time]`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | BGP `bfd` container in the owning `yang/`; `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `strict` valueless; `hold-time` `range` |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Doctor check for runtime dependencies | [ ] yes | strict requires the BFD plugin loaded — `ai/rules/repo-maintenance.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/bgp-bfd-strict.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `test/plugin/bgp-bfd-strict.ci` - functional test
- (unit tests extend existing `peer_bfd_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `strict`/`hold-time` YANG + `BFDSettings` fields; recognise strict at bring-up (no gate yet); failing `test/plugin/bgp-bfd-strict.ci`.
2. **Phase: Early session + Up handling** — open BFD pre-Established for strict peers; handle Up in `runBFDSubscriber`.
   - Tests: `TestBFDStrictHoldsUntilUp`
3. **Phase: Establish gate + hold-time fallback** — gate FSM Established on BFD-up; arm hold-time.
   - Tests: `TestBFDStrictHoldTimeFallback`
4. **Phase: Doctor check** — strict without BFD plugin is an error.
5. **Functional + interop tests**
6. **Full verification** → `make ze-precommit-verify`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | Down teardown still uses Cease subcode 10; no regression to non-strict path |
| Data flow | BFD reached only via `api.GetService()` |
| Doctor checks | strict + missing BFD plugin flagged |
| Registration over hardcoding | strict state exposed via existing peer status |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| strict gate | `go test ./internal/component/bgp/reactor -run BFDStrict` |
| interop | `NN-bfd-strict-peer` scenario passes against FRR bfdd |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | hold-time range enforced |
| Resource exhaustion | held-down strict peers do not leak BFD sessions/timers |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
