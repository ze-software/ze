# Spec: Home CPE 16g authentication refusal order proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | The L2TP live refusal checker accepts an LCP Configure-Ack before the required authentication refusal. |
| RFC gaps | - |
| Depends | Peer-image alignment: `plan/release/validation/spec-home-cpe-16h-subscriber-radius-pppd.md` |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

## Task

Own finding F-7 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Make the L2TP live checker reject a Configure-Ack before Terminate-Request in the disallowed-auth attempt.
Preserve the working authentication policy; this is a proof defect, not evidence of a runtime auth bypass.
Earlier race and interop results remain historical review evidence; this split reran neither.

## Required Reading

- [ ] `docs/labs/l2tp-interop.md`, CHAP-only scenario and pinned peer's termination behavior.
  → Constraint: preserve the single-attempt trace, matched Nak/TermAck and independent backend refusal evidence.
- [ ] `docs/guide/l2tp.md` and `docs/guide/pppoe.md`, authentication policy.
  → Constraint: required authentication closes before LCP Opened; supported CHAP sessions remain usable.
- [ ] `docs/architecture/testing/interop.md`, discrimination requirements.
  → Constraint: a refusal proof needs an actual refusal and a useful permitted-method control.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/le/interoplab/l2tp/lns_policy.go`, `lnsCHAPOnlyEvidence` | Orders CHAP request, PAP Nak, TermReq and TermAck; forbids CHAP/IPCP/PAP progress, but does not check ConfAck. |
| Same file, `checkLNSCHAPOnly` | Waits for peer exit and removed session, verifies Ze refusal logs and no RADIUS credential request before cleanup. |
| `internal/le/interoplab/l2tp/lns_policy_test.go`, `TestLNSCHAPOnlyEvidence` | Tests wrong IDs, order, protocol progress and redial, without a ConfAck-before-TermReq fixture. |

## Existing Mechanisms

| Mechanism | Where | Direction |
|-----------|-------|-----------|
| Ordered decoded packet parser | `lnsTracePacket` in `lns_policy.go` | Extend with direction-aware Configure-Ack refusal checks. |
| Complete live refusal carrier | `checkLNSCHAPOnly` in `lns_policy.go` | Preserve independent backend and teardown barriers. |
| Permitted authentication control | `checkLNSMRU` in `lns_policy.go` | Retain a CHAP-authenticated, traffic-carrying session alongside the negative case. |

## Data Flow

CHAP-only Ze request → PAP-only peer Nak → Ze TermReq → peer TermAck/exit → session removal and backend evidence → checker verdict.
Inspect packet direction and connection attempt boundaries; diagnostic text must not count as a packet.

## Draft Acceptance Criteria

| ID | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| AC-1 | Valid single-attempt disallowed-PAP trace. | Checker accepts the correlated refusal with no ConfAck before TermReq, no authentication/NCP progress and no backend request. | Required-auth refusal remains a valid test result because the suggested method is not allowed. |
| AC-2 | Insert Ze's Configure-Ack before TermReq into the otherwise valid refusal trace. | Checker rejects it even if teardown follows and CHAP/IPCP never appear. | The original valid refusal trace still passes. |
| AC-3 | Traces contain wrong-direction packets, unrelated IDs, repeated attempts or textual mentions of ConfAck. | Verdict depends on actual packets for the checked attempt and preserves existing negative coverage. | Diagnostic prose alone does not manufacture forbidden traffic. |
| AC-4 | A subscriber supports permitted CHAP-MD5. | Authentication succeeds and useful traffic crosses the established session. | Valid subscribers survive because the test strengthens proof rather than changing policy. |

## 🧪 TDD Test Plan

| Planned test / carrier | Proof |
|------------------------|-------|
| `TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck` in `lns_policy_test.go` | Inserted false-positive fixture fails before the checker fix, then is rejected. |
| Existing `TestLNSCHAPOnlyEvidence` | All existing positive and negative refusal fixtures retain coverage. |
| Live `l2tp-lns-chap-only-xl2tpd` plus CHAP-positive `l2tp-lns-mru-xl2tpd` | Actual pre-Opened refusal and supported-method session traffic. |
| Existing `pppoe-chap-only-pap-client` | Preserve the parallel PPPoE refusal-order proof and rerun it alongside the two L2TP policy scenarios. |

Show that the previous checker accepts the new false-positive fixture. Record forced-red/restored-green live discrimination using the governing interop workflow, and obtain independent `ze-test-judge` approval for protected test edits.

## Files to Modify

- `internal/le/interoplab/l2tp/lns_policy.go` and `lns_policy_test.go` — packet-order verdict and regression fixtures.
- `docs/labs/l2tp-interop.md` — precise refusal-order evidence; update matching wiki coverage without claiming a new policy restriction.

## Risks & Assumptions

| ID | Risk / assumption | Early signal or basis | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| R-1 | Searching raw text treats an unrelated log line as a packet. | Valid refusal fails after inserting diagnostic prose. | Match decoded bracketed packets and scope one attempt. |
| R-2 | Stronger checker silently loses backend/cleanup assertions. | New tests cover only a trace substring. | Preserve `checkLNSCHAPOnly` barriers and permitted-method traffic control. |
| A-1 | The pinned peer trace format supports unambiguous directions. | Existing `lnsTracePacket` prefixes. | Read the pinned log producer and inspect the live trace after16h; unvalidated. |

## Scope and Next Action

Printed repair-command fixes remain with `plan/release/fixes/spec-home-cpe-16c-auth-policy-repair-commands.md`.
Complete peer-source checks, functional wiring names and template checklists before `ready`; 16h is a lab-image dependency only.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
