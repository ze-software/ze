# Spec: ipsec strongswan interop reds

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Four shipped IPsec interop scenarios are red against strongSwan in `./le test integration interop-ipsec` on Ze's own kernel, on Linux and in the Mac `docker-lab` guest; a red interop scenario on a shipped capability is a release defect until it is diagnosed and fixed. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-lab-docker-cleanup-on-macos` AC-5 asked for `./le test qemu docker-lab
lab "test integration interop-ipsec" timeout 3600s` to exit 0 on the Mac. Its
cleanup and label half passed; the run itself ended `46 passed, 4 failed`
(rerun 3, 2026-10-23, Ze's docker-host image, kernel 7.2.9-ze, strongSwan
6.1.0 built from source). The four reds are IPsec defects, not cleanup ones.

-> Decision (owner, 2026-10-23): the `exit 0` half of that AC moves here. This
spec owns making the four scenarios pass, on the Linux host and through the Mac
`docker-lab` route. Each red is diagnosed at its producer first: the cause may
be Ze, the scenario's checker or fixture, or the strongSwan 6.1.0 build, and
the fix goes where the cause is.

-> Decision (closer, 2026-10-23): this spec owns the `mobike-initiator` red.
`plan/release/fixes/spec-ipsec-11-mobike.md` AC-12 and
`plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` AC-12 name the
same scenario as evidence for their own goals (MOBIKE behavior, a Ze-kernel
Docker host); they cite the passing run AC-2 here records and carry no fix of
their own for this red, so the work is not duplicated.

## Required Reading

- [ ] `docs/architecture/testing/interop.md` - the interop-ipsec suite, its scenario directories and what each assertion owes
  → Constraint: a fix that makes a scenario pass is proven by forcing it red with the fix reverted and the image rebuilt (`ai/rules/interop-and-goal-validation.md`).
- [ ] `docs/architecture/testing/qemu-integration.md` - "Docker labs in the Ze-kernel guest", the Mac route each AC also runs on

## Current Behavior

- [ ] `internal/le/interoplab/ipsec/checkers.go` - the scenario table maps each of the four names to its checker: `checkIKEPaddedProbeStrongSwan`, `checkMOBIKEInitiator`, `checkESPIntegrityOnlyNull` (waits for `swanNullESPProposal`) and `checkMTUNegotiatedTransform` (waits for `swanSecondESPProposal`).
- [ ] `test/interop-ipsec/scenarios/ike-padded-probe-strongswan/ze.conf` - the fixture sets no `unmatched`, so Ze's SPD catch-all DISCARD applies.

## Data Flow

### Entry Point
- `./le test integration interop-ipsec` (Linux host) and `./le test qemu docker-lab lab "test integration interop-ipsec" timeout 3600s` (Mac).

### Transformation Path
1. The lab builds the Ze and strongSwan 6.1.0 images and starts each scenario's containers.
2. Ze and charon negotiate IKE and ESP from the scenario's `ze.conf` and `swanctl.conf`.
3. The checker reads charon's log, Ze's CLI over `docker exec`, and probe traffic, and returns its verdict.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ strongSwan | IKEv2 and ESP on the lab network | No, AC-1 to AC-4 |
| checker ↔ Ze CLI | `docker exec` into the Ze container | No, AC-2 |

### Integration Points
- The existing scenario table and checkers; no new scenario.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-ipsec` | → | `checkIKEPaddedProbeStrongSwan`, `checkMOBIKEInitiator`, `checkESPIntegrityOnlyNull`, `checkMTUNegotiatedTransform` | the four scenarios, AC-1 to AC-4 |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | the strongSwan 6.1.0 source build changed charon's log line, so AC-3 and AC-4 wait for text that no longer appears | read charon's log from the failing run before changing the checker; a checker change keeps the assertion's strength |

## Acceptance Criteria

Each AC holds in `./le test integration interop-ipsec` on the Linux host and in
`./le test qemu docker-lab lab "test integration interop-ipsec" timeout 3600s`
on the Mac, with the commit, the kernel and the strongSwan version recorded.

| AC ID | Scenario | Expected Behavior |
|-------|----------|-------------------|
| AC-1 | `ike-padded-probe-strongswan` | the scenario passes: the prober reports `ike`, and no `sendto: operation not permitted` is left under Ze's SPD catch-all DISCARD |
| AC-2 | `mobike-initiator` | the scenario passes: the `docker exec` Ze CLI readback of peer state finishes inside its timeout and reports the moved address |
| AC-3 | `esp-integrity-only-null` | the scenario passes: charon logs its `selected proposal: ESP:...` line and the checker reads it before its timeout |
| AC-4 | `mtu-negotiated-transform` | the scenario passes: charon logs its `selected proposal: ESP:...` line and the checker reads it before its timeout |

## Current Evidence

| Scenario | Last run | Symptom |
|----------|----------|---------|
| `ike-padded-probe-strongswan` | AC-5 rerun 2 (2026-10-22) and rerun 3 (2026-10-23), Ze's docker-host image | `prober is icmp, want ike`; `sendto: operation not permitted` to 172.28.0.7 beside Ze's `SPD catch-all DISCARD is in force`. The fixture sets no `unmatched`, so the XFRM discard policy answers EPERM; why `ike` was not chosen before `icmp` is unread |
| `mobike-initiator` | the same two runs; first seen 2026-10-20 on the Alpine guest | the `docker exec` Ze CLI readback `did not finish within 30s (exit 124)`, so peer state is never measured; `mobike-responder` passes on the same kernel |
| `esp-integrity-only-null` | rerun 3 (2026-10-23); also red on the Linux host since strongSwan 6.1.0 | times out waiting for charon's `selected proposal: ESP:...` line |
| `mtu-negotiated-transform` | rerun 3 (2026-10-23); also red on the Linux host since strongSwan 6.1.0 | times out waiting for charon's `selected proposal: ESP:...` line |

## Citations

| Source | What it holds |
|--------|---------------|
| `plan/journal/conflicting-feature-contracts.md`, row 2026-10-20 (AC-5 run) | the `ike-padded-probe-strongswan` red and the discard-default fixture gap |
| `plan/journal/gate-verdict-depends-on-the-machine.md`, row 2026-10-09 | the `mobike-initiator` CLI readback hang on kernel 7.2.9-ze |
| `plan/handover/20-gobgp-v4.md`, "IPsec on strongSwan 6.1.0" row | `esp-integrity-only-null` and `mtu-negotiated-transform` red on the Linux host on 6.1.0 |
| `plan/journal/constant-reported-as-measured-state.md`, row 2026-09-11 | `show vpn ipsec sa` reports the configured ESP proposal, not the negotiated one; read it before changing what AC-3 and AC-4's checkers compare |
| `plan/release/fixes/spec-ipsec-11-mobike.md` AC-12, `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` AC-12 | consumers of the AC-2 run |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a failing test at the producer of each cause found | the package that holds the cause | the fix for that red | |

## Files to Modify

- `internal/le/interoplab/ipsec/checkers.go` - only if a checker is the cause.
- The Ze or strongSwan-image producer each diagnosis names.

## Implementation Steps

1. Diagnose each red from its failing run's logs, naming the producing function.
2. Failing test at that producer, fix, then `./le test integration interop-ipsec` on Linux.
3. The Mac route run, recorded per AC.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test integration interop-ipsec` passes the four scenarios on Linux and on the Mac route
