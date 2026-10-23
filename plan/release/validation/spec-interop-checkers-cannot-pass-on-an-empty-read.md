# Spec: interop-checkers-cannot-pass-on-an-empty-read

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation. These interop scenarios are the evidence for shipped behavior (IKEv2 Child SA rekey, ESP encapsulation form, SRv6 export, OSPF-LDP sync, BGP graceful restart). A checker that returns success on an empty read or never asserts the feature publishes a pass for behavior nobody observed. Split from `plan/post-release/engineering/spec-harness-cannot-pass-on-its-own-failure.md`, which owns the same root cause in the functional runner |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Root cause: an interop checker treats "my read found nothing" or "the session
came up" as success. Two shapes:

| Shape | What happens |
|-------|--------------|
| Empty read returns nil | The checker reads a baseline (ESP SPIs, XFRM state). When the read is empty, it returns success before the comparison that is the scenario's point |
| No feature assertion | The scenario's operations stop at session establishment, so it passes while the feature it is named for never ran |

Why it matters: these scenarios are release proof. `spec-feature-maturity-declared`
requires that IPsec interop checkers "cannot pass vacuously" before the level is
granted, and `spec-rfcgate-2-deferred-unrun-interop-trees` refuses a nightly tier
to a fail-open check. Both depend on this work.

Goal: an empty read is never success, and every scenario asserts the feature
it is named for. A registry-level test stops both shapes coming back.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - suites, scenario directories, assertion each feature type owes
  → Constraint: each scenario owes a feature-specific assertion, not a session check
- [ ] `ai/rules/interop-and-goal-validation.md` - vacuity traps, forced red
  → Decision: every repaired checker gets a forced red recorded once

## Current Behavior (MANDATORY)

Source files read on 2026-10-23:
- [ ] `internal/le/interoplab/ipsec/checkers.go` - `checkChildRekey` (line 716) and `checkResponderRaisesChildRekey` (line 743) run `if len(initial) == 0 { return nil }` after the rekey wait, so an empty SPI baseline passes. `checkESPFormChange` (line 1402) returns nil when Ze's XFRM state read is empty, while every other precondition in the function raises. `checkChildRekeyRefusedKeepsTheSA` (line 1181) already refuses an empty baseline with a reason; the analysis listed it as vacuous, which is wrong
- [ ] `internal/le/interoplab/bgp/checkers.go` - `bgp-srv6-frr` operations assert only that the FRR session establishes (2026-10-20 row); the sibling `bgp-srv6-invalid-sid-ineligible-frr` already waits for the route
- [ ] `internal/le/interoplab/bgp/check_engine.go` - `opBIRDRouteAbsent` reads the whole table and treats a failed query as never-absence (line 160); the shape this spec generalizes
- [ ] `test/interop/scenarios/ospf-ldp-sync-frr/frr.conf` - configures `mpls ldp`, but no scenario `daemons` override sets `ldpd=yes` and the checker waits only for OSPF Full (2026-10-14 row)
- [ ] `test/interop/scenarios/bgp-graceful-restart-frr/ze.conf` - declares `use bgp-gr` while the peer attaches only `announce-routes` (2026-08-16 row, not reproduced)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration <scenario>` and the nightly interop jobs run a scenario through `internal/le/interoplab`

### Transformation Path
1. The lab prepares containers from the scenario directory
2. The scenario's checker or operation list reads peer and kernel state
3. The checker's return decides pass or fail

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Lab to container | `docker exec` reads (`ip xfrm state`, `vtysh`, `birdc`) | [ ] |
| Checker to verdict | returned error | [ ] |

### Integration Points
- `scenarioOperations` registry in `internal/le/interoplab/bgp`
- the IPsec scenario checker table in `internal/le/interoplab/ipsec`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | [ ] | reads still go through `scenarioLab` helpers |
| No duplicated functionality | [ ] | one non-empty read helper replaces per-checker guards |
| Registration over hardcoding | [ ] | each scenario declares its feature assertion in its registry entry |

## Fix design

1. One read helper for every baseline a checker depends on: it returns the
   value or an error naming what was empty and where it was read. Checkers
   stop testing `len(x) == 0` themselves.
2. Each scenario registry entry declares at least one feature operation (route
   present with the attribute, SA replaced, label bound, LDP session up, GR
   state reached). Session establishment alone is never a feature operation.
3. Fix the named scenarios: `bgp-srv6-frr` waits for the exported SRv6 route on
   FRR; `ospf-ldp-sync-frr` ships a `daemons` file with `ldpd=yes` and waits for
   the LDP session; `bgp-graceful-restart-frr` attaches `bgp-gr` to the peer and
   asserts a GR transition.
4. The check that stops it coming back: a unit test over both registries that
   fails when a scenario has no feature operation, and a source test over the
   checker packages that fails on a checker returning success from an empty-read
   branch.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration` on an IPsec rekey scenario | → | `checkChildRekey` via the non-empty read helper | `TestChildRekeyRefusesEmptyBaseline` |
| scenario registry load | → | feature-operation declaration | `TestEveryScenarioAssertsItsFeature` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `checkChildRekey` or `checkResponderRaisesChildRekey` with an empty SPI baseline | Scenario fails, naming the empty read |
| AC-2 | `checkESPFormChange` with an empty Ze XFRM state | Scenario fails, naming the empty read |
| AC-3 | `bgp-srv6-frr` where FRR exports no SRv6 route | Scenario fails |
| AC-4 | `ospf-ldp-sync-frr` with ldpd not running | Scenario fails; with ldpd running it passes only after the LDP session is up |
| AC-5 | `bgp-graceful-restart-frr` with `bgp-gr` detached | Scenario fails |
| AC-6 | A scenario registered with only a session wait | `TestEveryScenarioAssertsItsFeature` fails and names the scenario |
| AC-7 | Each repaired checker | Forced red recorded once (break the producer, rebuild the image, red, restore, green) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation |
|----|------------|-------|------------|
| A-1 | The empty SPI read is a race with `waitChild`, not a normal state | 2026-09-05 row warns against fixing blind | read `waitChild` and run the scenario before AC-1 |
| A-2 | FRR in the lab image can run ldpd | FRR ships ldpd | AC-4 run |

### Risks
| Risk | Mitigation |
|------|------------|
| A repaired scenario goes red on a real product defect | Intended: it is then a release fix, routed to `plan/release/fixes/` |
| The Docker kernel lacks VRF, so `bgp-srv6-frr` cannot export | That is the 2026-10-20 cause; run on Ze's own kernel (lab rule) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChildRekeyRefusesEmptyBaseline` | `internal/le/interoplab/ipsec/` | AC-1 | [ ] |
| `TestESPFormChangeRefusesEmptyState` | `internal/le/interoplab/ipsec/` | AC-2 | [ ] |
| `TestEveryScenarioAssertsItsFeature` | `internal/le/interoplab/` | AC-6 | [ ] |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-srv6-frr` | `test/interop/scenarios/bgp-srv6-frr/` | FRR | AC-3 | [ ] |
| `ospf-ldp-sync-frr` | `test/interop/scenarios/ospf-ldp-sync-frr/` | FRR | AC-4 | [ ] |
| `bgp-graceful-restart-frr` | `test/interop/scenarios/bgp-graceful-restart-frr/` | FRR | AC-5 | [ ] |

## Files to Modify
- `internal/le/interoplab/ipsec/checkers.go` - non-empty read helper, three checkers
- `internal/le/interoplab/bgp/checkers.go` - feature operations
- `test/interop/scenarios/ospf-ldp-sync-frr/`, `bgp-graceful-restart-frr/` - daemons file, attach block
- `docs/architecture/testing/interop.md` - the two rules

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env var, doctor, metrics, BGP family | N-A | lab tooling only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 16 | Changed files referenced by doc anchors? | Yes | derive with `./le spec citation anchors spec plan/release/validation/spec-interop-checkers-cannot-pass-on-an-empty-read.md` |
| 1-9, 11-15, 17 | Product surfaces | No | no product change |

## Implementation Steps
1. Read `waitChild` and run the rekey scenario to settle A-1.
2. Non-empty read helper; AC-1, AC-2.
3. Feature-operation declaration and registry test; AC-6.
4. Repair the three scenarios; AC-3 to AC-5.
5. Forced red per checker; AC-7.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `balance-assertion-vacuous-without-a-loan.md` | 2026-09-08 | `checkChildRekey`, `checkResponderRaisesChildRekey` return nil on empty baseline |
| `green-that-could-not-have-been-red.md` | 2026-09-05 | `checkESPFormChange` returns nil on empty XFRM read |
| `green-that-could-not-have-been-red.md` | 2026-10-20 | `bgp-srv6-frr` asserts only session establishment |
| `test-against-broken-path.md` | 2026-10-14 | `ospf-ldp-sync-frr` never starts ldpd, waits only for OSPF Full |
| `test-against-broken-path.md` | 2026-08-16 | `bgp-graceful-restart-frr` declares `bgp-gr` the peer never attaches |

## Related specs
- `plan/release/validation/spec-rfcgate-2-deferred-unrun-interop-trees.md` (ready) asks for the two IPsec fail-open checks to be fixed before a nightly tier is granted; its research names the retired Python handlers. This spec fixes the native successors.
- `plan/release/validation/spec-interop-suite-red.md` owns the `bgp-srv6-frr` session and config defect; this spec adds the feature assertion that would have shown it.
- `plan/release/validation/spec-feature-maturity-declared.md` lists "its checkers cannot pass vacuously" as an IPsec interop maturity criterion.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Scoped tests pass: `./le job run label interoplab command go test ./internal/le/interoplab/...`, then `./le test integration` on each named scenario, then `./le verify current fast`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
