# Spec: startup-resilience

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/radius/client.go` - the verified-good reference pattern (lazy resolve per exchange)

## Task

**Audit skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Invariant to establish and enforce: **daemon startup and config apply must never
block on, or fail because of, an unreachable external service.** An appliance
must boot to a functional state (CLI reachable, config committed, local
forwarding up) even when its RADIUS/TACACS servers, RPKI validators, BMP
collectors, or management hub are down, and must converge when they return.

Reference: osvbng 424e2d0/2259ce6 fixed exactly this class (an unreachable
RADIUS server blocked daemon startup; BGP re-apply panicked; VRF table re-apply
was non-idempotent) as one "startup and config re-apply resilience" effort.

Ze status: the RADIUS client is already safe, VERIFIED 2026-07-10: server
addresses are resolved lazily per exchange inside `Client.Exchange`
(`internal/component/radius/client.go:134` `net.ResolveUDPAddr` at send time,
UDP writes with retry/backoff after), so no startup-time dial exists on that
path. It is the reference pattern for this audit. The remaining external-service
touchpoints have NOT been audited; each is an assumption row below with a
validation method. The audit produces: (a) a verified table of
connect-at-startup vs connect-lazily behaviour per touchpoint, (b) fixes for any
touchpoint that blocks or fails startup/config-apply when its peer is
unreachable, (c) functional tests pinning the invariant (daemon starts with
blackholed service addresses).

Audit surface (each row = read the producer, then classify):

| Touchpoint | Where to look | Question |
|-----------|----------------|----------|
| TACACS+ | `internal/component/tacacs/` | does config apply or first auth dial/block? timeout bounded? |
| RPKI (RTR) | `internal/component/bgp/plugins/rpki/` | validator connect at startup? retry loop detached from apply? |
| BMP collectors | `internal/component/bgp/plugins/bmp/` | collector dial blocking session/config paths? |
| Managed-node client | `internal/component/managed/` | hub unreachable at boot: does the node still finish local startup? |
| NTP | `internal/plugins/ntp/` | unreachable servers: any startup coupling? |
| RADIUS (component) | `internal/component/radius/client.go` | VERIFIED lazy (:134); document as the reference pattern |
| L2TP authradius plugin | `internal/component/l2tp/plugins/authradius/` | plugin setup path: config-time network activity? |
| DNS in config apply | `internal/component/resolve/`, any config path resolving hostnames | does an apply block on DNS for a dead resolver? |

Scope boundary: osvbng's sibling theme (idempotent re-apply of dataplane objects
on restart, their bond/subinterface/VRF-table fixes) is related but distinct;
if the audit finds Ze re-apply idempotency gaps, they get their own spec rather
than growing this one (see R-1).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/engine startup ordering.
  → Constraint: identify which lifecycle stage each touchpoint's first network activity runs in.
- [ ] `ai/rules/doctor-checks.md` - unreachable-service detection belongs in doctor/health, not in startup failures.
- [ ] `ai/rules/qemu-testing.md` - boot-with-blackholed-services evidence is Linux/QEMU territory.

**Key insights:**
- The invariant is testable cheaply: point every external-service address at a
  blackhole (drop, not reject, so timeouts are exercised) and assert the daemon
  reaches ready + commits config within its normal budget.

## Current Behavior (MANDATORY)

**Source files read:** (only the RADIUS row verified so far; the rest is the audit)
- [ ] `internal/component/radius/client.go` - `Exchange` resolves the server per call (:134) and writes UDP with bounded retries (:147-179); no constructor-time network activity on this path (verified 2026-07-10).
- [ ] `internal/component/tacacs/` - (audit at design time)
- [ ] `internal/component/bgp/plugins/rpki/` - (audit at design time)
- [ ] `internal/component/bgp/plugins/bmp/` - (audit at design time)
- [ ] `internal/component/managed/` - (audit at design time)
- [ ] `internal/plugins/ntp/` - (audit at design time)

**Behavior to preserve:**
- Services that already connect lazily / retry in the background keep their semantics.
- Legitimate hard failures (invalid config, missing local resources) still fail fast; this spec is about UNREACHABLE PEERS only.

**Behavior to change:**
- Any touchpoint where an unreachable peer blocks or fails startup/config-apply moves to lazy/background-retry connect semantics.

## Data Flow (MANDATORY)

### Entry Point
- Daemon startup sequence and config-apply transactions that reach external-service client setup.

### Transformation Path
1. Audit classifies each touchpoint: lazy (good) / eager-bounded (acceptable?) / eager-blocking (fix).
2. Fixes move first network contact out of the startup/apply critical path (dial on first use, or background retry loop with health reporting).
3. Health/doctor surfaces unreachable peers instead of startup errors.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| startup ↔ external services | per-touchpoint connect timing | [ ] |
| health ↔ operator | unreachable peer reported via health/doctor, not boot failure | [ ] |

### Integration Points
- Per-touchpoint client constructors/config-apply handlers (from the audit table).
- `internal/core/health/` - reporting surface for unreachable-but-tolerated peers.

### Architectural Verification
- [ ] No bypassed layers (fixes stay inside the owning component/plugin)
- [ ] No unintended coupling (no central "connection manager" invented without design approval)
- [ ] No duplicated functionality (reuse existing retry/backoff helpers where present)
- [ ] Registration over hardcoding - health/doctor checks register in owning packages

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | TACACS+ dials lazily or with bounded timeout off the startup path | unaudited | fix in scope | read `internal/component/tacacs/` producers | unvalidated |
| A-2 | RPKI RTR connects in a background loop that cannot block config apply | unaudited | fix in scope | read `internal/component/bgp/plugins/rpki/` producers | unvalidated |
| A-3 | BMP collector dial cannot block session establishment or apply | unaudited | fix in scope | read `internal/component/bgp/plugins/bmp/` producers | unvalidated |
| A-4 | Managed-node client tolerates an unreachable hub at boot | unaudited | fix in scope | read `internal/component/managed/` connect path | unvalidated |
| A-5 | NTP plugin startup is decoupled from server reachability | unaudited | fix in scope | read `internal/plugins/ntp/` producers | unvalidated |
| A-6 | No config-apply path blocks on DNS resolution of a peer hostname | unaudited | fix in scope | grep resolve calls in apply paths | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope creep into re-apply idempotency | audit findings unrelated to reachability | file findings into a separate skeleton spec; keep this one reachability-only |
| R-2 | Making a dial lazy hides a real misconfiguration | operators miss dead servers | pair every laziness fix with a health/doctor signal |
| R-3 | Background retry loops leak goroutines across config reloads | goroutine growth in tests | reload-cycle unit tests per fixed touchpoint |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| daemon boots with all external-service addresses blackholed | → | daemon reaches ready; config commits; health reports unreachable peers | `test/plugin/startup-unreachable-services.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | audit complete | every touchpoint row classified with producer `file:line` evidence in this spec |
| AC-2 | all configured external services blackholed | daemon startup reaches ready within its normal budget |
| AC-3 | config apply referencing an unreachable service | apply succeeds (or fails only for non-reachability reasons) |
| AC-4 | service becomes reachable later | touchpoint converges without restart |
| AC-5 | unreachable peer | surfaced via health/doctor, not silent |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | boots an appliance while the RADIUS/TACACS/hub network is down | startup → lazy clients → ready; health shows degraded peers | `test/plugin/startup-unreachable-services.ci` |
| 2 | restores the service network | background retries → convergence | (fill during design; may extend the same .ci) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per-touchpoint (named at design after audit) | owning packages | no blocking dial in constructor/apply | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| connect/retry timeouts | per-touchpoint (audit) | design | design | design |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `startup-unreachable-services` | `test/plugin/startup-unreachable-services.ci` | boot with blackholed services reaches ready | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - reachability/lifecycle behaviour, no wire format change | - | - | functional + QEMU coverage | - |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- Per-audit outcome; candidates: `internal/component/tacacs/`, `internal/component/bgp/plugins/rpki/`, `internal/component/bgp/plugins/bmp/`, `internal/component/managed/`, `internal/plugins/ntp/`

## Files to Create
- `test/plugin/startup-unreachable-services.ci` - invariant test
- (possible) sibling skeleton spec for re-apply idempotency findings (R-1)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **AUDIT (not started)** - read each touchpoint's producer, fill the classification table (AC-1), convert A-1..A-6 to confirmed/broken. Present the table before fixing anything.
2. **(conditional) FIX + TEST** - per broken touchpoint, in dependency order; fill phases at design time.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- Re-apply idempotency (osvbng's sibling theme) is explicitly out of scope; findings route to a new spec.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN (audit) completed and approved before fixes
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)
- [ ] Every touchpoint classified with producer evidence

### Quality Gates (SHOULD pass)
- [ ] Health/doctor signal exists for every tolerated-unreachable peer

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
