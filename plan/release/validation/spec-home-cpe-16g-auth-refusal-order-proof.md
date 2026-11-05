# Spec: Home CPE 16g authentication refusal order proof

| Field | Value |
|-------|-------|
| Status | done |
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
The original split did not rerun race or interop tests. Central Verification below records the subsequent executed proof.

## Required Reading

- [x] `docs/labs/l2tp-interop.md`, CHAP-only scenario and pinned peer's termination behavior.
  → Constraint: preserve the single-attempt trace, matched Nak/TermAck and independent backend refusal evidence.
- [x] `docs/guide/l2tp.md` and `docs/guide/pppoe.md`, authentication policy.
  → Constraint: required authentication closes before LCP Opened; supported CHAP sessions remain usable.
- [x] `docs/architecture/testing/interop.md`, discrimination requirements.
  → Constraint: a refusal proof needs an actual refusal and a useful permitted-method control.

## Current Behavior

These rows describe the pre-fix checker reviewed when this issue was split.
The implementation and closure audit below record its replacement verdict.

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
- Closure provenance correction: `internal/le/interoplab/lab.go`, new `image_provenance_test.go`, and the owned paragraph in `docs/architecture/testing/interop.md` — retain the built reference already consumed by peers in default progress output. No image selection or build lifecycle changes.

## Risks & Assumptions

| ID | Risk / assumption | Early signal or basis | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| R-1 | Searching raw text treats an unrelated log line as a packet. | Valid refusal fails after inserting diagnostic prose. | Match decoded bracketed packets and scope one attempt. |
| R-2 | Stronger checker silently loses backend/cleanup assertions. | New tests cover only a trace substring. | Preserve `checkLNSCHAPOnly` barriers and permitted-method traffic control. |
| A-1 | The pinned peer trace format supports unambiguous directions. | Existing `lnsTracePacket` prefixes. | Confirmed: actual pppd 2.5.5 logs place `rcvd`/`sent` at the start of packet records. The real request2/Ack2 fault is accepted by the old checker and rejected by the fixed checker. |

## Scope and Next Action

Printed repair-command fixes from `spec-home-cpe-16c-auth-policy-repair-commands`
landed in `18ea5e830a`; executable editor examples are retained in
`docs/guide/l2tp.md` and `docs/guide/pppoe.md`.
Original checker implementation and central verification are complete. Formal closure checks
the committed checker, its registered live entry point, and actual normal
and deliberately faulty peer traces. The generic provenance correction is
committed; fresh discrimination using identical image references and normal
restoration have passed. Native review is recorded and checked; closure commits remain.
16h remains the owner of image alignment.

## Implementation Handoff (2026-11-04)

Historical author handoff; the later Central Verification supersedes its
then-unrun test and discrimination instructions.

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
A fixed-checker run with that faulty wire behavior then failed for precisely
"LCP could open". These historical runs did not retain immutable image IDs;
they establish observed behavior, not equality of consumed images. Exact
identity is established only by the fresh paired runs recorded under Generic
Provenance Correction. No claim is made that the deliberately intercepted Ack
actually entered Opened; the proof is the forbidden wire order.

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

## Deliverables Checklist

| Deliverable | Verification | Result |
|-------------|--------------|--------|
| Reject the false-positive refusal trace | Read `lnsCHAPOnlyEvidence` and all three new tests in `lns_auth_order_test.go`; inspect forced-red/restored-green output. | Done: all three injected positions fail with the guard removed; the restored checker rejects them for `LCP could open`. |
| Preserve complete live refusal evidence | Read `checkLNSCHAPOnly`, `lnsNoCredentialRequest` and `scenarioCheckerMap`; inspect actual live results. | Done: process exit, empty daemon sessions, refusal cause, no backend request and kernel cleanup remain required before success. |
| Preserve supported subscribers | Inspect `checkLNSMRU` and the actual `l2tp-lns-mru-xl2tpd` run. | Done: CHAP Challenge/Response/Success, RADIUS request, measured link, useful traffic and cleanup pass. |
| Document packet direction | Read the CHAP-only scenario paragraph and its `lnsCHAPOnlyEvidence` source anchor in `docs/labs/l2tp-interop.md`. | Done in `3cc99259b0`: peer `sent` Ack is forbidden before termination; Ze's independent `rcvd` Ack is permitted. |
| Retain the consumed built image identity beside each live result | Check generic suite progress against actual peer image arguments; repeat discrimination and restore with retained IDs. | Done: eager/selected regression has RED/restored race GREEN; old/current checkers report identical Ze/LAC/RADIUS references with opposite intended verdicts; normal full-build refusal passes with its emitted IDs. |

## Security Review Checklist

| Concern | Source-aware review | Result |
|---------|---------------------|--------|
| False acceptance from diagnostics, wrong direction or another attempt | New loop in `lnsCHAPOnlyEvidence` requires a trimmed packet-record prefix after the one `Connect:` record; only matching previously received request IDs are rejected. | Clear; diagnostic, pre-connection, wrong-direction and unrelated-ID controls pass. Malformed candidate IDs/brackets return errors. |
| Unbounded state, overflow or panic | Identifier parsing uses `strconv.ParseUint` with 8-bit width; the new table is `[256]bool`; traversal is bounded by the existing trace length and termination index. | Clear; byte indexing cannot exceed the table. No new panic, recursion, worker, queue or cache. |
| Cleanup disguising acceptance | `checkLNSCHAPOnly` reads exit, packet evidence, daemon session removal and backend logs before `stopLACDaemon`. | Clear; registered-checker regression rejects the early Ack before its cleanup runs. |
| Policy weakening or credential disclosure | The change is in development checker code, with no product authentication/configuration writer changed. Existing live refusal and backend guards remain. | Clear; no new shell argument, path, socket, secret, crypto or authorization surface. Existing lab credentials remain isolated fixtures. |
| Incorrect artifact attribution | `Suite.prepareImages` now emits the same successful-build `result.Reference` stored for peer creation. NoBuild, pull and failed-build branches are untouched. | Clear: eager/selected regression checks actual container arguments and progress before the checker returns. Only locally built references are narrated; tags on other branches are not presented as build identities. |

## Documentation Update Checklist

| Category | Update? | Evidence |
|----------|---------|----------|
| Feature declaration / comparison | No | `features/l2tpv2-bng.md` and `features/pppoe-access.md` describe existing subscriber capabilities. This checker change adds no capability, supported role or feature-level claim. |
| User guides / wiki | No additional edit | `docs/guide/l2tp.md`, Authentication, `docs/guide/pppoe.md`, Subscriber authentication, and wiki `l2tp.md` still describe required-auth refusal before Opened. Existing policy/examples landed in `18ea5e830a` and wiki `312639d`; the proof fix changes no operator action. |
| Configuration / CLI / API / RPC | No | The committed hunk changes only `lnsCHAPOnlyEvidence` and adds tests. No schema, handler, response type or command grammar changes. |
| Plugin SDK / registration rules | No | No plugin, event type, SDK field or registration contract changes. Existing `scenarioCheckerMap` wiring is preserved. |
| Wire format / RFC summaries | No | No packet producer, protocol decision or RFC-tagged unit changes in this issue. The checker consumes existing pppd packet records; it claims no new RFC requirement coverage. |
| Test infrastructure / lab procedure | Yes | `docs/labs/l2tp-interop.md`, `l2tp-lns-chap-only-xl2tpd`, changed in `3cc99259b0`; source anchor names `lnsCHAPOnlyEvidence`. The provenance correction adds a paragraph in `docs/architecture/testing/interop.md`, Prove a scenario discriminates, anchored to `Suite.prepareImages`; its owned hunk excludes foreign scenario/PeerImage prose. |
| Architecture / runtime dependency / doctor | Owned paragraph only | `0bef13eda7` adds the built-reference progress contract to the existing interop procedure. The exact six-line paragraph was reviewed against `Suite.prepareImages`; foreign shared-page prose was excluded from that commit and this review. No additional runtime dependency, doctor check or structural boundary is introduced. |
| Route metadata / metrics / discovery | No | No metadata, counter, action, test framework or selector is added; this extends an existing registered scenario. `rg` finds the changed-file anchors in the owning lab page. |

## Implementation Summary

### Executive Summary

The live refusal checker now rejects the actual forbidden acknowledgement
order while normal refusal and permitted CHAP traffic remain proven.
The generic suite now emits the existing built image reference; eager and
selected regression cases pass under race. Fresh old/current discrimination
uses identical retained Ze/LAC/RADIUS references, and a normal full rebuild
passes refusal again with its own IDs. Native review passes; closure commits remain.
R-1 is controlled by packet-prefix, direction and attempt matching; R-2 by
the unchanged complete live carrier and its passing positive controls.
Wider RFC/documentation reds remain disclosed under Central Checks and
Limitations.

### What Was Implemented

`lnsCHAPOnlyEvidence` records Ze request IDs from decoded pppd records in
the checked attempt. A peer Ack matching any such request before the selected
Terminate-Request now fails the refusal proof. `TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck`
and `TestLNSCHAPOnlyEvidenceConfAckDirection` distinguish forbidden packets
from permitted controls. `TestLNSCHAPOnlyCheckerRejectsEarlyConfAck` traverses
the existing registered live checker and checks that cleanup cannot create
the refusal. The implementation is committed in `3cc99259b0`.

### Bugs Found/Fixed

The old checker accepted a real peer Ack before required-auth termination
when the faulty daemon withheld its independent Ack and later closed.
The fixed consumer rejects that actual wire order. This demonstrates the
checker defect; it does not demonstrate an authentication bypass or assert
that the deliberately interrupted connection entered Opened.

### Documentation Updates

The source-anchored lab paragraph names both Ack directions. Existing user
guide and wiki policy descriptions remain accurate. The earlier central
`./le doc check all` run found published catalog drift and unrelated pages.
The coordinator repaired its two stale
main-checkout pages in `d5efcf8bd3` and regenerated the index. Wider site and
wiki catalog work remains separate from this issue's complete lab paragraph.
The latest post-provenance documentation check reports only 142 published
gh-pages catalog disagreements; source anchors, YANG validation, usage/help,
rule freshness and discovery checks pass. No global-green result is claimed.

### Deviations from Plan

The registered-checker functional carrier is a Go test plus the existing
native live scenario, rather than a new `.ci`. All planned named tests exist.
No protected existing expectation was changed by this issue; its new tests
add stricter negative assertions. The independent test-judge approval in
`3cc99259b0` covers the separate MRU peer-input change, not these new tests.
No new protocol implementation or RFC proof record is authored at closure.
Closure also repairs missing provenance output in the generic suite. The
correction adds a two-line progress emission and one additive test file;
image identity selection and peer creation remain unchanged.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The earlier refusal proof checked only Nak, termination and absent protocol progress. | A real peer Ack can precede termination while every old assertion still passes. | Forced-red regression tests and actual-wire old-checker pass/fixed-checker rejection. | The consumer now excludes the correlated early Ack; the live positive/refusal controls remain. |
| approach | Historical default-progress live logs omitted built image identities. | `Suite.prepareImages` in `internal/le/interoplab/lab.go` already resolves a successful build to its immutable `Reference`, but published no progress record for it. | Closure compared actual logs with the image-provenance requirement in `docs/architecture/testing/interop.md`, Prove a scenario discriminates. | `0bef13eda7` repairs the producer; regression RED/restored race GREEN and full owning-package race PASS are observed. Fresh identical-image old/current discrimination and normal full-build restoration retain their exact references; round 2 is clean. |

### Provenance finding diagnosis

The symptom is an actual-wire verdict without a retained built image ID.
The producer is `Suite.prepareImages`: its successful-build branch stores
`result.Reference` for peer creation but omits it from normal progress output.
The owning layer is the shared native interop suite. A [workaround] would
inspect a current cache tag and attribute that identity to the historical run;
that cannot establish what the old container consumed. The [source] fix emits
the existing immutable reference through suite progress, with eager and
selected preparation tests that compare the logged identity to peer arguments.
Fresh fault/restore runs establish their own provenance and do not reconstruct
the historical IDs.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Reject acknowledgement before authentication refusal | Done | `lns_policy.go`, `lnsCHAPOnlyEvidence` | Actual early Ack is rejected for the intended reason. |
| Preserve authentication policy and complete refusal proof | Done | `lns_policy.go`, `checkLNSCHAPOnly`, `checkLNSMRU` | Normal refusal and supported CHAP traffic pass; no product policy change. |
| Retain consumed build identity for live discrimination | Done | `lab.go`, `Suite.prepareImages`, `0bef13eda7` | Fresh structured old/current runs use identical full references and produce opposite intended verdicts; normal full rebuild emits its references and passes. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLNSCHAPOnlyEvidence`, registered valid-refusal control; live `l2tp-lns-chap-only-xl2tpd` | Correlated Nak/termination, no backend request, natural peer exit and cleanup retained. |
| AC-2 | Done | `TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck`, `TestLNSCHAPOnlyCheckerRejectsEarlyConfAck`; real early-Ack fault | Old checker accepts the fault, current checker rejects it; normal rebuilt artifact passes. |
| AC-3 | Done | `TestLNSCHAPOnlyEvidenceConfAckDirection` plus existing refusal negatives | Valid direction, unrelated ID, prose, pre-connection and post-termination controls pass. Malformed packets and repeated attempts fail. |
| AC-4 | Done | Live `l2tp-lns-mru-xl2tpd`, `checkLNSMRU` | Actual CHAP success and useful traffic are required and passed. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Three new named regression tests | Done | `lns_auth_order_test.go` | Forced-red/current-green outputs observed; all three PASS under race. |
| Existing `TestLNSCHAPOnlyEvidence` | Done | `lns_policy_test.go` | Existing assertions retained; race PASS. |
| Live refusal and CHAP-positive controls | Done | `test/interop-l2tp/scenarios/l2tp-lns-chap-only-xl2tpd/`, `l2tp-lns-mru-xl2tpd/` | Native selected runs pass; final restored refusal passes again. |
| Parallel PPPoE refusal | Done | `test/interop-pppoe/scenarios/pppoe-chap-only-pap-client/` | Native selected run passes; `TestCHAPOnlyRefusalEvidence` race PASS. |
| Generic suite provenance regression | Done | `image_provenance_test.go`, `TestSuiteNarratesTheBuiltImageConsumedByPeers` | Both eager/selected cases have observed RED without the emitter and restored race GREEN. Full owning package race PASS. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/le/interoplab/l2tp/lns_policy.go` | Done | Refusal-order guard in `3cc99259b0`; existing assertions preserved. |
| `internal/le/interoplab/l2tp/lns_auth_order_test.go` | Done | All planned test functions exist with exact names. |
| `docs/labs/l2tp-interop.md` | Done | Source-anchored direction paragraph in `3cc99259b0`; wiki policy remains accurate. |
| `internal/le/interoplab/lab.go` | Done | Two-line successful-build progress emission in `0bef13eda7`; selection and ownership unchanged. |
| `internal/le/interoplab/image_provenance_test.go` | Done | Additive complete-suite regression in `0bef13eda7`. |
| `docs/architecture/testing/interop.md`, owned paragraph | Done | Six-line paragraph and source anchor in `0bef13eda7`; foreign shared-page prose excluded. |

### Audit Summary

18 items Done, including the generic source/test/doc correction required by
round 1 and the fresh live provenance. None Partial or Skipped; no item is
abandoned. Native artifact checking passes; the two closure commits remain.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Reject a peer Ack before required-auth termination | Unit and actual-wire discrimination | Without the new guard, all three injected early-Ack cases and the registered-checker regression fail. On real pppd traffic, old checker accepts request2/Ack2 before TermReq; fixed checker rejects it with `LCP could open`. |
| Preserve complete authentication refusal | Live interop | `l2tp-lns-chap-only-xl2tpd` passes after restoring the normal daemon image; `checkLNSCHAPOnly` requires exit, session removal, refusal log, no RADIUS request and kernel cleanup before success. |
| Preserve permitted CHAP subscribers | Live interop positive control | `l2tp-lns-mru-xl2tpd` passes actual CHAP-MD5, independent backend request, ping, measured link and daemon session output. Later restored omission/distinct-MRU controls pass CHAP and traffic too. |
| Retain the artifact identity beside discrimination outcomes | Generic regression and live provenance | Eager/selected complete-suite regression has observed RED/restored race GREEN. Fresh old/current structured reports contain identical full Ze/LAC/RADIUS references and intended opposite verdicts; normal full-build restoration passes with emitted references. Exact IDs appear in Generic Provenance Correction. |

The changed PPP-wire census is empty: the refusal fix changes a logfile
consumer, not a PPP writer. The existing shell-free `cat` invocation in
`readLNSPolicyTrace` reads pppd's logfile; `lnsTracePacket` decodes records;
`lnsCHAPOnlyEvidence` decides the refusal; `checkLNSCHAPOnly` calls it;
`scenarioCheckerMap` exposes it through the native selected scenario.
The unchanged PPPoE consumer `check_auth_allow.go`, `chapOnlyRefusalEvidence`,
already excludes the peer Ack from actual captured frames. Existing scenario
`ppp-options` sets `nopersist`, and `xl2tpd.conf` disables redial. No Python,
shell or third-party reader consumes a changed representation.
The generic provenance correction adds one progress-record form through
`Suite.prepareImages` and `Suite.progress`; the default sink is stderr.
Existing phase/verdict lines and the structured `SuiteReport.Images` reference
fields retain their formats. The new regression reads the added record and
existing complete-suite progress tests still pass.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every F-7 deliverable is implemented and centrally tested. Image alignment and other MRU/RFC work retain their existing sibling ownership. | Not applicable |

## Review Gate

Round 1 scope from the closure brief: the complete F-7 checker hunk and new
regression file in `3cc99259b0`, current packet/cleanup/registration consumers,
actual unit and live discrimination outputs, lab/guide/wiki claims and the
spec's ACs. Lenses: logic+wiring, security+edge cases, completeness+Go style.
The checker review found no source defect. The same round found the generic
suite's missing retained build identity, an ISSUE under the required interop
proof workflow. Round 2 reviewed only the stable `Suite.prepareImages` emitter,
the full new `TestSuiteNarratesTheBuiltImageConsumedByPeers`, the eager/selected
image and peer consumers they reach, and the exact owned documentation patch.
All three lenses found zero BLOCKER and zero ISSUE in that correction. The
observed regression RED and restored race GREEN corroborate its intended
behavior. Fresh live provenance and normal restoration are directly observed.
Closure-record corrections do not add a round.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-16g-auth-refusal-order-proof-3a525723-d369-4547-9124-fe35463dc0f5.md`; native artifact read, CLEAN, two rounds, seven reviewed files. |
| `./le spec review check` | PASS: coordinator observed `4 code files, clean, hashes match` for the final seven-file population. |
| Rounds | 2; round 1 found missing generic image provenance, round 2 reviewed its source correction and consumers. |
| Reviewer lenses used | Logic/wiring; security/edge cases; completeness/Go style. |
| Source review | Round 2 final source review: 0 BLOCKER, 0 ISSUE. Fresh identical-image discrimination and normal full-build restoration observed; native artifact/check complete. |

The style pass covered `lns_policy.go` and `lns_auth_order_test.go` against
`docs/contributing/ze-go-style.md`. No new panic or peer-controlled index can
escape the byte identifier bound. Loop bounds are the finite input trace;
request state is a fixed table local to one invocation. No lifecycle resource,
worker, wrapper API or exported domain invariant is added. Raw trace input is
validated before a successful verdict; parse errors propagate. The fixture
adapter changes only the returned trace and delegates every other operation
to the existing complete checker lab. The round-2 style pass covers `lab.go`
and `image_provenance_test.go`: one build-time `textbuf.Buffer`, existing
progress-writer semantics, no new lifecycle/worker/cache, no panic and no
image-reference mutation. Test loops have fixed populations and the test
observes the same reference in actual container arguments before success.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Default suite progress omitted the immutable built image reference required beside live discrimination results. | `internal/le/interoplab/lab.go`, `Suite.prepareImages` | `0bef13eda7` emits the existing build reference; additive regression has RED/restored race GREEN and full-package race PASS. Round 2 independently clean; fresh identical-image comparison and normal restoration complete. |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/l2tp/lns_auth_order_test.go` | Yes | Full file read: three exact planned test functions and `lnsAuthOrderLab.Exec`. |
| `internal/le/interoplab/image_provenance_test.go` | Yes | Full committed file read: additive eager/selected complete-suite regression and exact consumed-reference/progress checks. |
| Both L2TP policy scenario directories and parallel PPPoE scenario | Yes | Source and native selected-run output inspected; all scenario names resolve through existing checker registration. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Complete valid refusal still accepted | Read `checkLNSCHAPOnly` and normal final restored log: empty sessions, ordered Nak/TermReq/TermAck and ready backend without requests; scenario PASS. |
| AC-2 | Early Ack causes intended rejection | Read guard and actual old/fixed logs: matching request2/Ack2 before TermReq; old PASS, fixed FAIL `LCP could open`; restored unit race PASS. |
| AC-3 | Input distinctions retained | Read both fixture tables and race output: direction/ID/prose/pre-connection/post-termination controls PASS; malformed candidates return errors; existing wrong-ID/order/redial negatives PASS. |
| AC-4 | Supported CHAP and traffic remain useful | Read `checkLNSMRU` and native positive log: CHAP success, measured MTU1492, actual daemon user/address/auth and scenario PASS after ping/backend assertions. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Selected `l2tp-lns-chap-only-xl2tpd` native plan | Existing live scenario plus `TestLNSCHAPOnlyCheckerRejectsEarlyConfAck` | `scenarioCheckerMap` registers `checkLNSCHAPOnly`; complete carrier calls the changed consumer before cleanup. Actual live fault reaches the expected rejection. |
| Selected `l2tp-lns-mru-xl2tpd` / `pppoe-chap-only-pap-client` | Existing native live scenarios | Actual positive and parallel refusal runs PASS; no new `.ci` is required for an existing native checker. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Actual pppd logfile has start-of-record `rcvd [LCP ConfReq id=0x2 ...]` then `sent [LCP ConfAck id=0x2 ...]` before `rcvd [LCP TermReq ...]`. Fixed consumer rejects this real sequence. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Peer Ack direction and permitted Ze Ack | `lnsCHAPOnlyEvidence` and exact direction tests | Owning lab paragraph's source anchor resolves and its claim matches the consumer. |
| Natural exit, backend and cleanup barriers | `checkLNSCHAPOnly`, `lnsNoCredentialRequest` | Both owning lab anchors and Design/Related headers match current source. |
| Operator auth behavior and wiki | Required-auth sections in both guides and wiki `l2tp.md` | No product policy writer changed; existing refusal-before-Opened wording remains accurate. |
| Other categories | `rg` changed-file source-anchor census; owned checker and generic progress hunks | No config/API/SDK/wire/dependency change. Existing feature descriptions remain applicable; the owned progress paragraph describes the added narration. |
| Generic built-reference progress | `Suite.prepareImages`, `Suite.progress`; committed owned paragraph in `0bef13eda7` | Correct: local successful builds emit their resolved reference before peer start; pull/NoBuild do not emit a build identity. No whole-page review of foreign prose is claimed. |
| Latest documentation and citation checks | `job-cpe16-doc-check-provenance-final-baf5a445.log`; coordinator's citation/anchor check | Source anchors: 4200 code paths/772 packages, all valid. All 443 YANG commands validate; usage/help contracts pass; 34 rules round-trip identically; 365 design docs and 3037 digest anchors resolve. Citation/anchor check PASS. Remaining 142 failures are published gh-pages catalog drift only. |

### Central Checks and Limitations

| Check | Observed result and scope |
|-------|---------------------------|
| Full scoped race run | `job-cpe16-postreview-runtime-race-f3e7163f.log`: all L2TP runtime packages and both checker packages PASS, including all new/existing refusal tests and `TestCHAPOnlyRefusalEvidence`. No own failure or skip is hidden. |
| Fixture discrimination | `job-cpe16-refusal-old-checker-red-c37f7460.log`: three early-Ack subcases and registered checker FAIL because the old consumer accepts them. Direction controls PASS. Restored `job-cpe16-refusal-checker-restored-5ffdc697.log`: all three tests PASS under race. |
| Historical actual-wire observations | `job-cpe16-live-early-ack-old-checker-8708cdb4.log`: deliberately faulty behavior, old checker, real request2/Ack2 before TermReq, 1 passed. `job-cpe16-live-early-ack-fixed-checker-a3f0496b.log`: that faulty wire behavior, current checker, 0 passed/1 failed for `LCP could open`. Those runs omitted immutable IDs; equal consumed images are established only by fresh `eeb1ae95`/`267f5fe1` structured reports below. No synthetic log text or claim of actual Opened. |
| Restored live controls | `job-cpe16-final-artifacts-restored-718e9ad2.log`: normal required-auth refusal, omitted MRU and distinct negotiated MRU all PASS after normal image rebuild. Earlier `job-cpe16-final-l2tp-live-e6b8eb67.log` includes permitted CHAP/Nak traffic PASS; parallel PPPoE refusal PASS in `job-cpe16-live-pppoe-refusal-final-b6032ddb.log`. |
| Broader prechecks | Central repo check has nine foreign unused exports; weakened check has ten foreign findings and no F-7 test. These are disclosed unrelated reds, not a claim of global health. |
| Documentation | Latest post-provenance `./le doc check all` reports 142 published gh-pages disagreements only: one CLI JSON catalog plus 47 mode differences across HTML/Markdown CLI indexes and llms metadata. Main/wiki repairs, source anchors, YANG validation, usage/help, rule freshness, discovery and digest checks pass. Wider publication remains centrally tracked; no global documentation-green claim. |
| RFC freshness | Neither changed F-7 unit carries an RFC requirement tag or owns a summary/record. Corpus check has wider 16a/client/BGP findings; no global RFC-green result is claimed and closure authors no records. |
| Generic provenance regression and package | Both eager/selected cases fail without the progress emitter and pass after restoration under race. Complete owning `internal/le/interoplab` package race PASS in `job-cpe16-interop-package-provenance-green-afff16bc.log`. |
| Committed source compilation | `job-cpe16-provenance-committed-compile-9f891c3d.log`: HEAD `0bef13eda781` has `compiles: OK (every flavor of the committed tree compiles)`, covering distro, appliance, setup, host and installer. This is compilation evidence, not a whole-tree test claim. |
| Final native review and citations | Final artifact is CLEAN over seven files and two rounds; coordinator's native check PASSes for all four code files with matching hashes. Citation/anchor check PASSes. No new source changes occurred after round 2. |

### Retained Command and Observed Output

| Central native carrier | Directly observed result |
|------------------------|--------------------------|
| Selected `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-chap-only-xl2tpd` through `./le --name cpe16 test deployment docker-l2tp-ppp` | Old checker accepts the real faulty ordering; fixed checker rejects it with `LCP could open`; final normal rebuild passes. |
| Selected `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-mru-xl2tpd` through the same native command | PASS: CHAP and traffic plus the independently checked backend, link and session evidence. |
| Selected `ZE_PPPOE_INTEROP_SCENARIO=pppoe-chap-only-pap-client` through `./le --name cpe16 test deployment docker-pppoe-accel` | PASS: complete parallel refusal proof. |

Retained evidence is the canonical checker, named tests, scenario inputs and
source-anchored lab procedure committed in `3cc99259b0`, plus the directly
observed outcomes recorded here in commit A. The session logs under
`tmp/session/2026-11-04-222204/scratch/` are temporary debugging aids. This
independent closure agent performed static reads and prose edits only; all
compilation, execution, native gates and commits belong to the coordinator.

### Generic Provenance Correction (2026-11-05)

The independent correction is committed in `0bef13eda7`. Its complete
production change is two lines in `Suite.prepareImages`, which emit the
successful local build's logical name and existing `result.Reference` through
`Suite.progress`. Eager and selected preparation both reach this producer
before peer creation. Image selection, ownership, cleanup and pull/NoBuild
behavior remain unchanged.

`TestSuiteNarratesTheBuiltImageConsumedByPeers` in
`internal/le/interoplab/image_provenance_test.go` executes `Suite.Run` in both
preparation modes. It requires actual peer creation to consume the same full
SHA256 reference present in progress before the checker, plus the final pass
verdict. Both cases FAIL for missing progress output without the emitter in
`job-cpe16-interop-image-provenance-red-41f0f16a.log`; both PASS under race
after restoration in `job-cpe16-interop-image-provenance-green-41f0f16a.log`.
The complete owning package also PASSes under race in
`job-cpe16-interop-package-provenance-green-afff16bc.log`, including the new
regression and existing suite progress tests.

The only reviewed architecture change is the six-line progress paragraph in
`docs/architecture/testing/interop.md` carried by `0bef13eda7`, anchored to
`Suite.prepareImages`. Its working-tree page also contains foreign edits;
the correction used an owned HEAD-based hunk and did not publish those edits.
Final native review population is four Go files, the lab page and both plan
files. The exact committed architecture paragraph is reviewed separately;
no whole-page review of foreign work is claimed.

The fresh native full-build current-checker run
`job-cpe16g-fixed-checker-image-proof-4dca3d97.log` emitted all three built
references below and rejected the actual early Ack for `LCP could open`.
The coordinator then compared old and current host checkers using those exact
available immutable image references, rather than a shared cache tag. The
input-only private image-declaration overlay changes each selected image's
Tag to its full SHA256 reference. With `NO_BUILD=1`, `Suite.prepareImages`
preserves that exact reference and `resolvedPeers` passes it to peer creation.
The structured result records all three exact references for each run.
The old variant alone additionally disables the `lnsCHAPOnlyEvidence` guard;
the current variant leaves that consumer unchanged.

| Comparison using identical images | Directly observed verdict |
|--------------------------|---------------------------|
| `job-cpe16g-old-checker-pinned-latest-proof-eeb1ae95.log` | Actual request2/Ack2 before TermReq, complete original barriers pass; JSON code 0, passed 1, failed 0. |
| `job-cpe16g-current-checker-pinned-latest-proof-267f5fe1.log` | Actual request2/Ack2 before TermReq rejected with `LCP could open`; JSON code 1, passed 0, failed 1. |

Both structured reports record the identical three-image population:

| Logical image | Exact consumed reference |
|---------------|--------------------------|
| Ze | `sha256:c2563e711ae1b472c2ac002dc24e2fcf7179aa40baff745f7910944cfdf191fb` |
| LAC | `sha256:23a5196652fa1f8e8b47f203f09cdb21a273dbff765c62dcc21d5d8eb1c20cf2` |
| RADIUS | `sha256:8decd78ca04f6b2e6d7c945627967d0ffc78b58cf744860f1fac77cbaf6e37bd` |

An earlier fresh old-checker full build also accepted the real faulty order,
but produced a different image population. That build and the later full
build are separate controls, not the identical-image comparison above. A
setup-only Docker-info timeout and a subsequent attempt naming an unavailable
old image are not semantic evidence; the successful paired runs supersede
them. No historical omitted ID is inferred from a current shared tag.
The final normal full-build restoration in
`job-cpe16g-normal-image-provenance-restored-8cbdeacb.log` passes the complete
refusal carrier: one attempt, correlated PAP Nak and TermReq/TermAck, no peer
Ack of Ze's request, no backend request, natural peer exit, removed daemon
session and kernel cleanup. Ze's independent Ack of the peer request appears
and is correctly permitted. The native full build emits these identities:

| Restored image | Exact consumed reference |
|----------------|--------------------------|
| Ze | `sha256:9249679e3b8f1a3af1afa6a48059641c3770787fe6a223a9d2780013829b8925` |
| LAC | `sha256:888be29a76531204e87131f0c36747fd1e80a3802a449e154a3058f8de975a7a` |
| RADIUS | `sha256:d2e91c0271db10000c1578ef16d38bcd629e73a5ee67c2e3a2cda1be8a470c7a` |

The result is `1 passed, 0 failed`. This is restored normal behavior, not a
claim that the faulty daemon entered Opened. The source and canonical tests
remain unchanged after the clean second review.
