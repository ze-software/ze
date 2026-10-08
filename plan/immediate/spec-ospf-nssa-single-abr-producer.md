# Spec: ospf-nssa-single-abr-producer -- one ABR producer for NSSA defaults, and the missing NSSA interop proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `spec-ospf-rfc3101-nssa-defaults` by the owner on 2026-10-08. The
parent kept AC-1 through AC-12, which are implemented, and closed on them the same
day; its record is in git history.

-> Decision (owner, 2026-10-08): AC-13/AC-14 (single ABR default producer), the three missing interop scenarios and `test/ospf/ospf-nssa-no-summary-default.ci` move out of `spec-ospf-rfc3101-nssa-defaults` into this spec.

Four sites compute "am I an area border router" independently, per the parent's
2026-09-05 note: `lsdb.isAreaBorderRouter` (`internal/plugins/ospf/lsdb/origination.go`),
`v6IsAreaBorderRouter` (`internal/plugins/ospf/origination_v6.go`), `ospfspf.IsABR`
in `applyNSSADefaults` (`internal/plugins/ospf/nssa.go`), and `IsABR` in
`Computer.Run` (`internal/plugins/ospf/spf/computer.go`). Their snapshots and update
times can disagree across a backbone transition, so what Ze advertises in its
Router-LSA B-bit and what it originates as an NSSA default can disagree. The owner
decided on 2026-08-02 (parent R-2) to UNIFY: the Router-LSA B-bit determination
becomes the single producer and both default-route consumers read it. The second
consumer is the summary originator, reached through `IsABR(in.Areas)` in
`spf/summary.go` and `origination_v6_summary.go`, so the producer decides every
stub, totally-stubby, NSSA and normal area's summary set. That blast radius is why
the parent cut this into its own package.

The parent also named three interop scenarios and one functional test that were
never written. They are this spec's evidence obligations. The facts above are the
parent's claims; the design phase re-reads each producer.

## Acceptance Criteria

Moved verbatim from `spec-ospf-rfc3101-nssa-defaults`:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-13 | Any reachable router state, including mid-transition, with at least one attached NSSA | What Ze ADVERTISES and what Ze ORIGINATES agree: whenever the self Router-LSA carries the B-bit, every attached NSSA holds its required default (Type-7 for a regular NSSA, Type-3 or `::/0` for a no-summary one), and whenever the B-bit is clear, Ze originates no border-router default in any area. The single-producer refactor is the means of achieving this, not the assertion |
| AC-14 | A backbone interface transitioning down then up on a router attached to a no-summary NSSA | The area never holds zero defaults as a result of the two consumers disagreeing. Any remaining absence is bounded by the single producer's own update, not by a race between producers |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-11-stub-nssa.md` - stub and NSSA area behavior
- [ ] `docs/architecture/ospf/ospfv3-6-interop-coverage.md` - OSPFv3 interop coverage

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3101.md` - NSSA default origination and install gates

## Current Behavior (MANDATORY)

**Source files read:** (the design phase reads each before writing this section)
- [ ] `internal/plugins/ospf/lsdb/origination.go` - [`isAreaBorderRouter`; read at design time]
- [ ] `internal/plugins/ospf/origination_v6.go` - [`v6IsAreaBorderRouter`; read at design time]
- [ ] `internal/plugins/ospf/nssa.go` - [`applyNSSADefaults`; read at design time]
- [ ] `internal/plugins/ospf/spf/computer.go` - [`Computer.Run`; read at design time]

## Data Flow (MANDATORY)

### Entry Point
- [Where data enters: written at design time]

### Transformation Path
1. [written at design time]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → peer daemon | Type-7 and Type-3 default LSAs on the wire, verified against FRR | No |

### Integration Points
- [written at design time]

## Risks & Assumptions

[written at design time]

Carried from `spec-ospf-rfc3101-nssa-defaults` at its closure (2026-10-08), unvalidated,
because only the interop scenarios this spec now owns can reach it:

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-7 | An FRR ABR's Type-7 default is P-clear, so it is exactly the LSA RFC3101-2.4-4 requires Ze to refuse | RFC 3101 Section 2.4: "The Type-7 default LSA originated by an NSSA border router must have the P-bit clear." FRR is presumed conformant | The negative direction of the install gate needs an injected P-clear default rather than a peer-originated one | `ospf-nssa-two-abr-frr` and its v6 twin assert the received LSA's P-bit before asserting Ze refuses it | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [written at design time] | → | single ABR producer | `TestOSPFNSSADefaultAgreesWithRouterLSABBit` |

## 🧪 TDD Test Plan

### Unit Tests
Moved verbatim from the parent:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFNSSADefaultAgreesWithRouterLSABBit` | `internal/plugins/ospf/nssa_ac14_16_test.go` | AC-13: table-driven over reachable states. For each, assert the self Router-LSA B-bit and the per-area default set agree in both directions, both families | |
| `TestOSPFNSSANoSummaryDefaultSurvivesBackboneFlap` | `internal/plugins/ospf/nssa_ac14_16_test.go` | AC-14: a backbone down/up transition never leaves a no-summary NSSA holding zero defaults through producer disagreement | |

### Functional Tests
Moved verbatim from the parent:

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-nssa-no-summary-default` | `test/ospf/ospf-nssa-no-summary-default.ci` | An operator configures a no-summary NSSA and the daemon originates a Type-3 default rather than a Type-7 | |

### Interop Tests (Scope: protocol)
Moved verbatim from the parent:

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-v6-nssa-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospf6d` | AC-1 and AC-3 for OSPFv3: Ze as a dual-area v6 NSSA ABR originates a 0x2007 default and FRR installs `::/0` as an NSSA route | |
| `ospf-nssa-two-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospfd` | AC-6 and AC-7 for OSPFv2: FRR is a second NSSA ABR configured with `area X nssa default-information-originate`, so Ze receives a P-clear Type-7 default and must not install it. A no-summary variant proves AC-7 | |
| `ospf-v6-nssa-two-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospf6d` | AC-6 and AC-7 for OSPFv3 through the same two-ABR topology | |

Each new scenario asserts on BOTH sides: FRR's route table via `FRROSPF6.wait_ospf6_route` or
`FRROSPF.wait_ospf_route`, and Ze's own state via `docker_exec_quiet(ZE_CONTAINER, ["ze",
"show", "ospf", "database", "nssa-external"])` and `["ze", "show", "ospf", "route"]`. The
receive-side scenarios must show the LSA PRESENT in Ze's LSDB while `0.0.0.0/0` (or `::/0`)
is ABSENT from Ze's route table: that pair is what distinguishes "gate worked" from "LSA
never arrived", which is the vacuity trap `ai/rules/interop-and-goal-validation.md` names.

The AC-1, AC-3, AC-6 and AC-7 these scenarios prove are the parent's ACs, implemented
there; this spec owes their interop proof.

## Files to Modify
- [written at design time; the parent named `lsdb/origination.go`, `origination_v6.go`, `nssa.go` and `spf/computer.go`]

### Integration Checklist
- [written at design time]

### Documentation Update Checklist (BLOCKING)
- [written at design time]

## Implementation Steps
1. [written at design time]

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
