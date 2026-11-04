# Spec: Home CPE 16e live MRU Nak proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | The LNS MRU interop scenario does not exercise its promised oversized-peer Configure-Nak. |
| RFC gaps | - |
| Depends | Peer-image alignment: `plan/release/validation/spec-home-cpe-16h-subscriber-radius-pppd.md` |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

## Task

Own finding F-5 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Make `l2tp-lns-mru-xl2tpd` prove a peer request greater than 1492 is Nakked to 1492 on the wire, then converges and carries traffic.
This is a planning skeleton; no new live proof is claimed.
Earlier race and interop results remain historical review evidence; this split reran neither.

## Required Reading

- [ ] `docs/labs/l2tp-interop.md`, policy scenarios and peer-source basis.
  → Constraint: retain one-attempt packet evidence and matching packet identifiers.
- [ ] `docs/architecture/testing/interop.md`, interop discrimination rules; RFC 1661 Section 6.1 in repository authoritative text.
  → Constraint: a scenario that omits the oversized request cannot prove the Nak behavior.
- [ ] The pinned pppd source that writes MRU options and handles Configure-Nak.
  → Constraint: choose an explicit MRU pppd actually transmits, rather than assuming an option at its default of 1500.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/le/interoplab/l2tp/lns_policy.go`, `lnsMRUEvidence` | Requires Ze's MRU1492/CHAP request and matching Ack, then CHAP success; does not require an oversized peer request or MRU Nak. |
| `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/ppp-options` | Requests MRU1492. |
| `internal/le/interoplab/l2tp/lns_policy_test.go`, `TestLNSMRUEvidence` | Covers the present request/Ack evidence and several invalid traces. |

## Existing Mechanisms

| Mechanism | Where | Direction |
|-----------|-------|-----------|
| Decoded peer packet correlation | `lnsTracePacket` in `lns_policy.go` | Extend the existing ordered packet checks. |
| Live data and cleanup controls | `checkLNSMRU` in `lns_policy.go` | Preserve measured client interface, ping, RADIUS and daemon checks. |

## Data Flow

Scenario ppp-options → pppd Configure-Request → Ze Configure-Nak → peer retry/Ack → CHAP/IPCP → kernel PPP interface and traffic → typed checker.
Follow packet identifiers in the peer trace; an echo of scenario configuration is not wire evidence.

## Draft Acceptance Criteria

| ID | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| AC-1 | Ze MRU1492; pinned pppd explicitly requests a verified value in 1493..1499. | Peer trace contains that request, Ze Nak1492 with its identifier, and the resulting retry/Ack sequence. | A peer that accepts 1492 continues because negotiation has converged. |
| AC-2 | The sequence in AC-1 completes. | CHAP succeeds, the PPP interface has the expected MTU, and bidirectional data traffic succeeds. | Supported subscriber traffic remains usable; dropping all packets fails the test. |
| AC-3 | Trace lacks the oversized request, Nak, matching identifier or convergence. | Checker fails with the missing evidence named. | The complete valid trace still passes. |
| AC-4 | Peer explicitly requests 1500 in shared PPP unit coverage. | Existing explicit1500 boundary remains covered despite pppd's default-option omission. | Live and unit carriers prove their separate reachable inputs. |

## 🧪 TDD Test Plan

| Planned test / carrier | Proof |
|------------------------|-------|
| `TestLNSMRUEvidenceNakSequence` in `lns_policy_test.go` | Valid sequence plus missing Nak, wrong identifier and absent-convergence negative fixtures. |
| Existing `TestLNSMRUEvidence` | Preserve its CHAP, one-attempt and wrong-MRU assertions. |
| Live `l2tp-lns-mru-xl2tpd` | Actual oversized request, Nak and successful data session. |

Record a forced-red run with the Nak behavior broken in the rebuilt Ze artifact, then restored green. Obtain independent `ze-test-judge` approval for protected test edits.

## Files to Modify

- `internal/le/interoplab/l2tp/lns_policy.go` and `lns_policy_test.go` — strengthen the registered checker and its fixtures.
- `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/ppp-options` — explicit oversized MRU supported by pinned pppd.
- `docs/labs/l2tp-interop.md` — state the actual Nak proof and retained red/green evidence; update matching wiki coverage.

## Risks & Assumptions

| ID | Risk / assumption | Early signal or basis | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| A-1 | Pinned pppd transmits an MRU in 1493..1499. | Owner review; upstream producer not yet read in this split. | Read that version's MRU writer and inspect the live packet; unvalidated. |
| R-1 | This edit removes the unequal-value output proof. | F-6 also changes the policy scenario. | Coordinate with `plan/release/validation/spec-home-cpe-16f-negotiated-mru-output-proof.md`; retain both proofs using separate attempts if needed. |

## Scope and Next Action

F-1 and F-2 runtime cap fixes remain with their specs under `plan/release/fixes/`; this spec owns the missing live Nak proof.
Peer-image dependency 16h aligns the lab, not production behavior. Complete peer-source research, wiring names, changed-wire census and template checklists before `ready`.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
