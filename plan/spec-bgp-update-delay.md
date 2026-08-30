# Spec: bgp-update-delay

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - reactor/plugin startup model
4. `internal/component/bgp/plugin/register.go` - peer startup wiring
5. `internal/component/bgp/reactor/` - `StartPeers`, outbound advertisement path

## Task

On BGP process startup (boot or reactor restart), Ze begins advertising routes to
peers the instant each session reaches Established, before the RIB has learned the
routes it will eventually receive from other peers. This causes premature, churny
advertisement: neighbours see an initial route set, then a rapid sequence of
add/withdraw as the local best-path stabilises.

Add a **convergence-hold (read-only) startup mode** for the BGP engine:

- `max-delay` (seconds): after startup, hold the engine in read-only mode. While
  held, sessions may negotiate and reach Established, but the engine does **not**
  run best-path or send outbound UPDATEs. The hold ends when either all *expected*
  peers reach Established or the `max-delay` timer expires, whichever comes first.
- `establish-wait` (seconds, optional, `≤ max-delay`): how long to wait for peers
  to reach Established before finalising the set of *expected* peers. This lets a
  router that boots faster than its neighbours still wait for them.

This is a local-process startup feature. It is distinct from graceful restart
(which concerns a *remote* peer restarting) and from per-peer reconnect backoff.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `docs/architecture/core-design.md` - startup ordering, plugin coordinator
  → Constraint: peer startup is already deferred behind `coord.OnPostStartup` until all tiers finish their 5-stage handshake; the convergence hold layers on top of this, it does not replace it.
- [ ] `docs/architecture/cli/color-system.md` - only if a `show` command exposes read-only state (semantic roles)
  → Constraint: any new operational output uses the 7 semantic roles.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4271 (BGP-4) baseline UPDATE/best-path semantics. `update-delay` is a
  widely-implemented operational behaviour, not a wire-protocol change; no new
  capability or attribute is negotiated. (Create an `rfc/short/` summary during
  implementation only if design surfaces a normative requirement.)
  → Constraint: read-only mode must not alter OPEN/negotiation; only outbound Adj-RIB-Out population and best-path scheduling are gated.

**Key insights:**
- Nothing is sent on the wire that a peer can detect as "read-only"; the hold is purely a local decision to defer best-path + outbound UPDATE generation.
- The hold must still allow inbound UPDATEs to be received and stored (Adj-RIB-In), otherwise convergence can never be detected.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugin/register.go` - `coord.OnPostStartup` calls `bgpReactor.StartPeers()` with no convergence timer and no advertisement/best-path suppression (register.go). Peers are started unconditionally once plugin startup completes.
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - graceful restart is per-remote-peer only (stale marking, restart timer, EOR-driven purge); no local-process startup deferral of best-path or UPDATE.
- [ ] `internal/component/bgp/reactor/` - outbound advertisement / best-path path (exact function to be located during design; `internal/component/bgp/rib/` holds the outbound side).

**Behavior to preserve:**
- Peer startup remains gated behind `coord.OnPostStartup` (register.go); the hold is additive.
- When no `update-delay` is configured, behaviour is byte-for-byte unchanged: peers start and advertise immediately.
- Inbound UPDATE processing and RIB population are unaffected while held.
- Graceful restart semantics (gr plugin) are untouched.

**Behavior to change:**
- When `update-delay max-delay` is set, the engine defers best-path and outbound UPDATE generation until the hold releases.

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves under the BGP `parameters` container, e.g. `parameters update-delay max-delay <sec>` and `parameters update-delay establish-wait <sec>`, defined in the BGP component/plugin YANG.
- Resolved through the standard config path: File → Tree → `ResolveBGPTree()` → `map[string]any` → `reactor.PeersFromTree()` / reactor settings.

### Transformation Path
1. YANG leaves parsed into reactor/global BGP settings (a new `UpdateDelay` settings struct: `MaxDelay`, `EstablishWait`).
2. On `StartPeers()` (register.go), if `MaxDelay > 0`, the reactor enters read-only mode and arms the hold timer.
3. Sessions negotiate and reach Established normally; inbound UPDATEs populate Adj-RIB-In / RIB, but best-path scheduling and outbound Adj-RIB-Out generation are suppressed.
4. Hold-release condition evaluated on each peer Established transition and on timer expiry: release when expected peers are all Established or `max-delay` elapses.
5. On release: run best-path once, then generate and send the initial outbound UPDATE set to all peers; resume normal steady-state advertisement.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Reactor | YANG leaves → `UpdateDelay` settings via `PeersFromTree` | [ ] |
| Reactor ↔ RIB/best-path | read-only flag gates best-path + Adj-RIB-Out generation | [ ] |
| FSM ↔ Reactor | Established transitions drive hold-release evaluation | [ ] |

### Integration Points
- `bgpReactor.StartPeers()` (register.go) - arm the hold here.
- Outbound advertisement / best-path scheduler in `internal/component/bgp/rib/` - honour the read-only flag.
- Per-peer FSM Established callback - notify the hold evaluator.

### Architectural Verification
- [ ] No bypassed layers (config flows through `PeersFromTree`, not a side channel)
- [ ] No unintended coupling (gr plugin untouched; read-only is a reactor concern)
- [ ] No duplicated functionality (reuse existing best-path scheduling; add a gate, not a parallel path)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — the read-only state and any `show` view register through existing registries; no new per-feature switch case added to a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Outbound UPDATE generation has a single choke point that can be gated | `internal/component/bgp/rib/` outbound path | If dispersed, gating is invasive | grep/read the Adj-RIB-Out generation path during audit | unvalidated |
| A-2 | Inbound processing continues while held so convergence is observable | reactor FSM independent of outbound | Convergence never detected → always times out (still safe) | unit test with held reactor receiving UPDATEs | unvalidated |
| A-3 | "Expected peers" = configured peers admin-enabled at startup | operator intent | Wrong set → premature or delayed release | design confirmation with user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Read-only mode deadlocks steady-state advertisement if release never fires | peers Established but no routes advertised | max-delay timer is a hard backstop; unit test asserts release-by-timer |
| R-2 | Held reactor delays legitimate fast convergence in small topologies | operator reports slow first advertisement | feature is opt-in (max-delay default 0 = off) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set protocols bgp parameters update-delay max-delay 30` | → | reactor enters read-only mode in `StartPeers` | `test/plugin/bgp-update-delay.ci` |
| peers reach Established before timer | → | hold releases, initial UPDATE sent | `test/plugin/bgp-update-delay.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `max-delay 30`, one expected peer | engine holds read-only; no outbound UPDATE until peer Established or 30s elapse |
| AC-2 | all expected peers reach Established at t<max-delay | hold releases at that moment; best-path runs once; initial UPDATE set sent |
| AC-3 | no peer reaches Established | hold releases at exactly `max-delay`; engine advertises whatever it has |
| AC-4 | `establish-wait 40` with `max-delay 30` | config rejected at verify: establish-wait must be ≤ max-delay |
| AC-5 | `max-delay 0` or leaf absent | behaviour unchanged: immediate advertisement |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures `update-delay max-delay 30` and boots with a slow neighbour | config → reactor read-only → hold until Established/timer → single converged UPDATE | `test/plugin/bgp-update-delay.ci` |
| 2 | sets `establish-wait 40 max-delay 30` | config verify rejects with a clear error | `test/plugin/bgp-update-delay-validation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateDelayHoldsUntilEstablished` | `internal/component/bgp/reactor/update_delay_test.go` | read-only until expected peers Established | |
| `TestUpdateDelayReleasesOnTimer` | `internal/component/bgp/reactor/update_delay_test.go` | release at max-delay with no peers up | |
| `TestUpdateDelaySuppressesOutbound` | `internal/component/bgp/reactor/update_delay_test.go` | no Adj-RIB-Out generation while held | |
| `TestUpdateDelayValidation` | `internal/component/bgp/.../config_test.go` | establish-wait ≤ max-delay | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-delay | 0-3600 | 3600 | N/A (0 = disable) | 3601 |
| establish-wait | 1-3600 (≤ max-delay) | max-delay | 0 | max-delay+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-update-delay` | `test/plugin/bgp-update-delay.ci` | configured hold releases on convergence and on timer | |
| `bgp-update-delay-validation` | `test/plugin/bgp-update-delay-validation.ci` | invalid establish-wait rejected | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-update-delay-peer` | `test/interop/scenarios/` | FRR/GoBGP | held Ze does not advertise until converged; peer sees a single stable UPDATE set | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/plugin/register.go` - arm hold in `StartPeers`
- `internal/component/bgp/reactor/` - read-only state, hold timer, release evaluator, outbound gate
- `internal/component/bgp/rib/` - honour read-only flag in Adj-RIB-Out / best-path scheduling
- BGP config resolution (`internal/component/bgp/config/`) - parse new leaves, validate establish-wait ≤ max-delay

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | BGP component/plugin `yang/` - `update-delay { max-delay, establish-wait }`; read `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `range 0..3600` / `1..3600`; custom validator for establish-wait ≤ max-delay |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/bgp-update-delay.ci` |
| Prometheus counters/metrics | [ ] maybe | a gauge for "read-only active" and a counter for release-reason (converged/timeout) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `internal/component/bgp/reactor/update_delay.go` - hold state machine
- `internal/component/bgp/reactor/update_delay_test.go` - unit tests
- `test/plugin/bgp-update-delay.ci` - functional test
- `test/plugin/bgp-update-delay-validation.ci` - validation test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add YANG leaves + settings struct, arm a no-op hold in `StartPeers`, write failing `test/plugin/bgp-update-delay.ci`.
   - Verify: config accepted, entry point reached, wiring test fails on stubbed behaviour.
2. **Phase: Read-only gate** — suppress best-path + Adj-RIB-Out while held.
   - Tests: `TestUpdateDelaySuppressesOutbound`
3. **Phase: Release logic** — evaluate on Established + timer; run best-path once and flush initial UPDATEs.
   - Tests: `TestUpdateDelayHoldsUntilEstablished`, `TestUpdateDelayReleasesOnTimer`
4. **Phase: Validation** — establish-wait ≤ max-delay at config verify.
   - Tests: `TestUpdateDelayValidation`
5. **Functional + interop tests**
6. **Full verification** → `./le verify current mode full`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | timer never leaks; release is idempotent; no double initial-flush |
| Data flow | inbound processing unaffected while held |
| Registration over hardcoding | read-only `show` view (if any) registered, not hardcoded into the CLI Model |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| read-only hold in reactor | `go test ./internal/component/bgp/reactor -run UpdateDelay` |
| config validation | `test/plugin/bgp-update-delay-validation.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | max-delay/establish-wait range enforced in YANG + verify |
| Resource exhaustion | held reactor still bounds Adj-RIB-In growth |

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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
