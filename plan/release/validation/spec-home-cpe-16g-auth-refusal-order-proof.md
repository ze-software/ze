# Spec: Home CPE 16g authentication refusal order proof

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | The L2TP live refusal checker accepts an LCP Configure-Ack before the required authentication refusal. |
| RFC gaps | - |
| Depends | Peer-image alignment: `plan/release/validation/spec-home-cpe-16h-subscriber-radius-pppd.md` |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-7 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Make the L2TP live checker reject the peer's acknowledgement of a Ze Configure-Request before Terminate-Request in the disallowed-auth attempt.
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
| AC-1 | Valid single-attempt disallowed-PAP trace. | Checker accepts the correlated refusal with no peer Ack of Ze's request before TermReq, no authentication/NCP progress and no backend request. | Required-auth refusal remains a valid test result because the suggested method is not allowed. |
| AC-2 | Insert the peer's Configure-Ack of a Ze request before TermReq into the otherwise valid refusal trace. | Checker rejects it even if teardown follows and CHAP/IPCP never appear. | The original valid refusal trace and Ze's independent Ack of the peer request still pass: Ack-Sent alone does not establish LCP Opened. |
| AC-3 | Traces contain wrong-direction packets, unrelated IDs, repeated attempts or textual mentions of ConfAck. | Verdict depends on actual packets for the checked attempt and preserves existing negative coverage. | Diagnostic prose alone does not manufacture forbidden traffic. |
| AC-4 | A subscriber supports permitted CHAP-MD5. | Authentication succeeds and useful traffic crosses the established session. | Valid subscribers survive because the test strengthens proof rather than changing policy. |

## 🧪 TDD Test Plan

| Planned test / carrier | Proof |
|------------------------|-------|
| `TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck` in `lns_auth_order_test.go` | Inserted false-positive fixture fails before the checker fix, then is rejected. |
| `TestLNSCHAPOnlyEvidenceConfAckDirection` in `lns_auth_order_test.go` | Valid refusal, Ze-originated Ack, unrelated IDs and diagnostic/pre-connection records stay distinct from a peer Ack of Ze's request. |
| `TestLNSCHAPOnlyCheckerRejectsEarlyConfAck` in `lns_auth_order_test.go` | The registered live checker rejects early peer acknowledgement and retains the valid complete refusal path. |
| Existing `TestLNSCHAPOnlyEvidence` | All existing positive and negative refusal fixtures retain coverage. |
| Live `l2tp-lns-chap-only-xl2tpd` plus CHAP-positive `l2tp-lns-mru-xl2tpd` | Actual pre-Opened refusal and supported-method session traffic. |
| Existing `pppoe-chap-only-pap-client` | Preserve the parallel PPPoE refusal-order proof and rerun it alongside the two L2TP policy scenarios. |

Show that the previous checker accepts the new false-positive fixture. Record forced-red/restored-green live discrimination using the governing interop workflow, and obtain independent `ze-test-judge` approval for protected test edits.

## Files to Modify

- `internal/le/interoplab/l2tp/lns_policy.go` and new `lns_auth_order_test.go` — packet-order verdict and regression fixtures; existing `lns_policy_test.go` assertions are preserved.
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

## Implementation Handoff (2026-11-04)

The owner requires all compilation and execution in the root session. This
agent authored source, tests and documentation only; no formatter, compiler,
test, daemon, Docker command or commit ran.

| Surface | Authored change and source basis |
|---------|---------------------------------|
| `lnsCHAPOnlyEvidence` | Track decoded Ze Configure-Request Identifiers within the one actual `Connect:` attempt and reject matching peer Configure-Acks before Ze's Terminate-Request. A fixed 256-entry set bounds the Identifier state. Preserve every existing refusal, backend and cleanup assertion. |
| Direction correction | `pppoe.chapOnlyRefusalEvidence` forbids the client's Ack received by Ze. In pppd's trace that packet is `sent`, while a `rcvd` Ack is Ze acknowledging the client's independent request and cannot alone open LCP. The root confirmed this distinction from the producing code before implementation. |
| Log record scope | Pinned pppd 2.5.5 `utils.c`, `log_write`, writes the formatted packet message directly to its logfile. Require packet direction at the start of the trimmed record; diagnostic mentions and records before `Connect:` do not count as acknowledgements for the checked attempt. |
| New proof carriers | New `lns_auth_order_test.go` contains correlated early-Ack negative traces, preserved valid-direction controls and a registered-checker traversal using the complete live refusal carrier. Existing tests and fixtures are unchanged. |
| Documentation | The CHAP-only scenario paragraph now names the forbidden packet direction and the valid independent Ze Ack. |

Root must format the new test file and run the three new test units plus
existing `TestLNSCHAPOnlyEvidence`, `TestLNSPolicyCheckersReachDaemon`,
`TestCHAPOnlyRefusalEvidence` and the live `l2tp-lns-chap-only-xl2tpd`,
`l2tp-lns-mru-xl2tpd` and `pppoe-chap-only-pap-client` controls. The previous
checker accepts the inserted early-Ack trace by source inspection; its actual
red run and restored green remain unobserved and are owed centrally. Keep
the scenario's backend and useful-traffic controls in the live evidence.

## Central Verification (2026-11-05)

The coordinator ran the three new early-Ack/direction/registered-checker
tests and the existing checker suite under race. All passed. Removing only
the new guard made all three inserted early-Ack cases and the registered
checker regression fail, while valid refusal, wrong direction, unrelated ID,
diagnostic/pre-connection and post-termination controls still passed. The
restored targeted tests passed again.

The normal L2TP refusal, permitted CHAP/Nak traffic and parallel PPPoE
refusal scenarios all passed. For an actual-wire discrimination, a private
hand-written daemon solicited a no-auth Configure-Request Ack before closing,
withheld the independent Ack that would let the peer enter NCP, and closed
on the real validated response. No trace text was fabricated. With the old
checker guard disabled, the real peer's matching Ack of request2 before
Terminate-Request passed every existing backend, no-NCP and cleanup barrier.
The same faulty image, without rebuilding it, then failed under the fixed
checker for precisely "LCP could open". No claim is made that the deliberately
intercepted Ack actually entered Opened; the proof is the forbidden wire order.

Implementation is committed in `3cc99259b0`. Evidence under
`tmp/session/2026-11-04-222204/scratch/`:
`job-cpe16-final-l2tp-live-e6b8eb67.log`,
`job-cpe16-live-pppoe-refusal-final-b6032ddb.log`,
`job-cpe16-refusal-old-checker-red-c37f7460.log`,
`job-cpe16-refusal-checker-restored-5ffdc697.log`,
`job-cpe16-live-early-ack-old-checker-8708cdb4.log` and
`job-cpe16-live-early-ack-fixed-checker-a3f0496b.log`.
All execution was centralized; the issue agent authored without running code.
The final rebuilt normal artifact passed required-auth refusal again, then
the omitted-MRU and distinct-MRU useful-traffic controls passed, in
`job-cpe16-final-artifacts-restored-718e9ad2.log`. Central verification is
complete; this spec stays open for the separate closure review.
