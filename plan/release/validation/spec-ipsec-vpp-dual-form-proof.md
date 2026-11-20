# Spec: ipsec-vpp-dual-form-proof

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol validation |
| Priority | P2 |
| Release reason | Thomas brought VPP IPsec into the first release on 2026-11-20 (his decision 2, following ruling 3a, recorded in `plan/release/fixes/spec-ipsec-vpp-policy-interface.md`, "Thomas's rulings"). A first-release VPP IPsec backend owes real-VPP evidence that it receives bare and UDP-encapsulated ESP on one inbound SA, which this spec supplies and the operator selector (`plan/release/fixes/spec-ipsec-dataplane-selector.md`) consumes. |
| Depends | plan/release/fixes/spec-ipsec-vpp-policy-interface.md |
| Phase | 5/5 (inherited sequence, not completion) |
| Updated | 2026-10-22 |

Moved on 2026-11-20 from `plan/post-release/features/spec-ipsec-vpp-dual-form-proof.md`
into `plan/release/validation/`, by Thomas's decision 2 of that day: the bucket
holds evidence that shipped behavior works, which is what this spec produces.
Status stays `blocked`: its dependency, the VPP policy-interface spec, is still open.

## Task and ownership

Own AC-5 split from `plan/release/validation/spec-ipsec-esp-dual-form-receive.md`,
including its VPP A-6/AC-6 assumption evidence, R-4 policy dependency, Phase 4
VPP capability handling and real-VPP harness extension. The parent retains
all XFRM evidence, Stories 1..3, throughput R-1, A-7 bookkeeping, durable
Design links and ledger regeneration. Its complete original design, test
proposals, provenance and dated findings remain the historical record;
VPP passages there are non-owning references to this child.

The production dataplane is XFRM. `spec-ipsec-dataplane-selector` records that
VPP is unavailable to operators and must not become selectable before the
policy-interface and real-ESP prerequisites hold. Its real-VPP proof
prerequisite now names this owner; installation and SA read-back do not clear it.

The post-release text carried only the sections below "Acceptance and proof";
the sections the spec validator requires were added on the move, from what the
policy-interface spec and the evidence run already record. They state no new
design.

## Required Reading

- [ ] `plan/release/validation/spec-ipsec-esp-dual-form-receive.md` - the parent: AC-5's origin, the 2026-09-05 "MEASURE IT" ruling
  → Constraint: a refusal does not replace the measurement
- [ ] `plan/release/fixes/spec-ipsec-vpp-policy-interface.md` - the dependency: AC-1..AC-5, the WAN-interface ruling, ruling 3a and its open C-4
  → Constraint: no IKE policy reaches VPP until that spec lands
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - "Both ESP forms on one inbound state", "Decisions"
- [ ] `docs/architecture/testing/interop.md` - the real VPP proof scenarios the evidence run carries

## Current Behavior

**Source files read:** (2026-11-20)
- [ ] `internal/le/test/deployment/vppevidencerun.go` - `(*VPP).runIPsec`, the real-VPP IPsec producer; one Go producer per scenario
- [ ] `internal/component/ike/dataplane/vpp.go` - the SA half of the VPP backend
- [ ] `internal/component/ike/dataplane/dataplane.go` - `ErrNotSupported`, `SPParams`

**Behavior to preserve:** the XFRM dual-form evidence the parent owns.
**Behavior to change:** none in product code; the run gains the dual-form measurement.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| The real-VPP evidence run | `runIPsec` (`internal/le/test/deployment/vppevidencerun.go`), `scenarioRuns` (`vppevidence.go`) | Installs SAs on a real VPP and reads them back | Extends: sends both ESP forms on one inbound SA and reads the decryption counters |
| The XFRM dual-form proof | `plan/release/validation/spec-ipsec-esp-dual-form-receive.md` | Proves both forms on XFRM | Does not apply: XFRM stays the parent's |

## Data Flow

### Entry Point
- `./le test deployment vpp`, the real-VPP evidence run

### Transformation Path
1. `runIPsec` installs one inbound SA through the VPP backend
2. The run sends bare ESP and UDP-encapsulated ESP (port 4500) for that SA
3. VPP's counters show both decrypted on the same SA

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Evidence run to VPP | GoVPP binary API and the packet generator | No |

### Integration Points
- `scenarioRuns` (`vppevidence.go`), which orders the run's scenarios

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test deployment vpp` | → | `runIPsec` | the run's dual-form check, both forms decrypted on one SA |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-5 | One inbound SA on a real VPP; bare ESP and UDP-encapsulated ESP arrive for it | both decrypt on that SA, read from VPP's counters; or, if measured unable, `ze config verify` refuses VPP with a named capability error |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the `scenarioRuns` order test | `internal/le/test/deployment/vppevidence_test.go` | the dual-form step is part of the run | to be extended |

## Files to Modify
- `internal/le/test/deployment/vppevidencerun.go` - the dual-form measurement in `runIPsec`

## Implementation Steps

1. After the policy-interface spec lands, extend `runIPsec` with the two forms on one SA
2. Run it on a real VPP and record the counters here (A-6)

## Risks & Assumptions

| ID | Assumption | Basis | Validated by | Status |
|----|-----------|-------|--------------|--------|
| A-6 | VPP decrypts both forms on one inbound SA | none: the August source reading was withdrawn | the real-VPP run | unvalidated |

## Acceptance and proof

- AC-5: a real VPP receives bare and UDP-encapsulated ESP on the same inbound
  SA and decrypts both. Extend the current native `(*VPP).runIPsec` producer
  in `internal/le/test/deployment/vppevidencerun.go`, registered by `scenarioRuns`
  in `vppevidence.go`; the older `run_ipsec_evidence` name is provenance. (The
  post-release text named `internal/le/deployment/`; the files are under
  `internal/le/test/deployment/`.)
- Preserve the capability-error obligation: a backend measured unable to
  receive both forms must fail `ze config verify` with a named capability
  error. Refusal is not a substitute for the measurement or support.
- Resolve VPP A-6 with real evidence. The August source-reading claim was
  withdrawn by its own producer; neither the old confirmed cell nor the
  former not-applicable verdict proves reception. Record the result with
  the harness and actual same-SA decryption evidence.
- Retain the policy-interface prerequisite and its AC-1..AC-5 before exposing
  VPP to operators. R-4 records that `SPParams.IfIndex == 0` currently refuses
  IKE policies; removing that accidental guard cannot silently discharge
  the dual-form obligation.

## Decisions and scheduling preserved

The 2026-08-29 withdrawal leaves AC-5 unresolved. The 2026-09-05 ruling is
MEASURE IT, not a choice between measurement and refusal. The 2026-11-20 ruling
(decision 2) places this spec in the first release; it does not start the work
before its dependency clears. The 2026-10-09 owner set-aside ("Set aside the 7
specs waiting on something else") and owner defer-heavy-testing instruction
therefore still apply while the policy-interface spec is open: this child is
blocked, not newly commissioned or ready.

The parent's original Phase 4 refusal-only test proposals and all checklist
references to VPP are governed by that September ruling. No RFC artifact,
verdict, evidence status or tagged-test authorization is changed by this split
or by the move.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test unit pkg path ./internal/le/test/deployment/` and the real-VPP run `./le test deployment vpp`, run by the main thread
