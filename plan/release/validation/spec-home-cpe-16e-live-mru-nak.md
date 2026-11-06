# Spec: Home CPE 16e live MRU Nak proof

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | The LNS MRU interop scenario does not exercise its promised oversized-peer Configure-Nak. |
| RFC gaps | - |
| Depends | Aligned peer images in `3cc99259b0`; `docs/labs/l2tp-interop.md`, Peer-source basis |
| Phase | Source review and live Nak fault red complete, normal restoration blocked by Docker lifecycle |
| Handoff | blocked |
| Updated | 2026-11-06 |

## Task

Own finding F-5 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Make `l2tp-lns-mru-xl2tpd` prove a peer request greater than 1492 is Nakked to 1492 on the wire, then converges and carries traffic.
The direct semantic red is observed. Fresh normal restoration and cleanup
are blocked by Docker lifecycle failure before the scenario starts.
The independent closure agent reads source and central observations without executing code.

## Required Reading

- [x] `docs/labs/l2tp-interop.md`, policy scenarios and peer-source basis.
  → Constraint: retain one-attempt packet evidence and matching packet identifiers.
- [x] `docs/architecture/testing/interop.md`, interop discrimination rules; RFC 1661 Sections 5.3 and 6.1 in repository authoritative text.
  → Constraint: a scenario that omits the oversized request cannot prove the Nak behavior.
- [x] Pinned pppd 2.5.5 `pppd/lcp.c`, `lcp_addci` and `lcp_nakci`.
  → Constraint: choose an explicit MRU pppd actually transmits, rather than assuming an option at its default of 1500.

## Behavior Before Implementation

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

## Acceptance Criteria

| ID | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| AC-1 | Ze MRU1492; pinned pppd explicitly requests a verified value in 1493..1499. | Peer trace contains that request, Ze Nak1492 with its identifier, and the resulting retry/Ack sequence. | A peer that accepts 1492 continues because negotiation has converged. |
| AC-2 | The sequence in AC-1 completes. | CHAP succeeds, the PPP interface has the expected MTU, and bidirectional data traffic succeeds. | Supported subscriber traffic remains usable; dropping all packets fails the test. |
| AC-3 | Trace lacks the oversized request, Nak, matching identifier or convergence. | Checker fails with the missing evidence named. | The complete valid trace still passes. |
| AC-4 | Peer explicitly requests 1500 in shared PPP unit coverage. | Existing explicit1500 boundary remains covered despite pppd's default-option omission. | Live and unit carriers prove their separate reachable inputs. |

## 🧪 TDD Test Plan

| Planned test / carrier | Proof |
|------------------------|-------|
| `TestLNSMRUEvidenceNakSequence` in `lns_mru_nak_test.go` | Valid sequence plus missing Nak, wrong identifier and absent-convergence negative fixtures. |
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
| A-1 | Pinned pppd transmits an MRU in 1493..1499. | `lcp_addci` emits 1493 because it differs from DEFMRU1500. | Confirmed by the real1493 request in the final native run. |
| R-1 | Convergence makes local and peer MRUs equal. | The Nak scenario cannot alone discriminate a copied session value. | Separate `l2tp-lns-negotiated-mru-xl2tpd` uses peer1400 with configured1492. F-6 retains that proof. |

## Scope and Next Action

This spec owns the live Nak proof. F-1 and F-2 cap fixes and the peer-image
alignment have separate completed reviews. Restore the native Docker
lifecycle, observe the fresh normal run and cleanup, then resume closure.

## Implementation Handoff (2026-11-04)

| Surface | Authoring result | Central verification owed |
|---------|------------------|---------------------------|
| Nak producer proof | Read pinned pppd 2.5.5 `lcp_addci` and `lcp_nakci`: default 1500 omits MRU; explicit 1493 is emitted and a 1492 Nak is accepted. | Capture that exact request/Nak/retry/Ack from the real pinned peer. |
| Registered MRU checker | `lnsMRUEvidence` now requires the oversized request, matching Nak, retry and Ack before CHAP; retains local request/Ack and existing positive controls. | Run `TestLNSMRUEvidenceNakSequence`, prior checker tests, and the live `l2tp-lns-mru-xl2tpd` scenario after the judged fixture migration. |
| Prior trace proof | `TestLNSLocalMRUProofSurvivesNakStrengthening` preserves the complete old trace as a local-request/authentication proof, while refusing to count it as a Nak proof. | Independent judgment of the exact fixture migration in session scratch, then root applies it. |
| Omitted-MRU carrier for F-1 | Added `l2tp-lns-omitted-mru-xl2tpd`, the typed `checkLNSOmittedMRU`, Ze-side MTU measurement and raw peer MRU 1500 check. | Run new checker/preparation tests and the real omitted-option scenario with useful traffic; F-1 remains open until this succeeds. |
| Documentation | Updated the lab MRU subsection and added omitted-option evidence requirements. | Root doc/anchor check and corresponding wiki edits. |

The fixture migration and existing scenario's peer MRU 1492 to 1493 change are
proposed as an exact patch for independent judgment, not applied by the author.
All Go, image, daemon and test execution remains with the main thread at the
owner's request. No compilation, formatting, tests, Docker command or commit
ran in this authoring agent. All runtime outcomes remain unverified.

## Historical Verification

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
| 1 | Original local-policy/CHAP trace and1493 peer fixture migration | Independent coordinator judgment | Approved before applying; original complete local-policy/auth trace remains a separate positive control and the new trace adds actual Nak correlation. | Native approval for `l2tp.TestLNSMRUEvidence`; fixture Test-judge rationale in `3cc99259b0`. |
| 2 | Unit/checker race and actual pppd wire exchange | Main coordinator | Pass: peer1493, Ze Nak1492, matching1492 retry/Ack, CHAP/IPCP and useful traffic. | `job-cpe16-postreview-runtime-race-f3e7163f.log`, `job-cpe16-final-l2tp-live-e6b8eb67.log`. |

## Central Verification (2026-11-05)

The coordinator observed the actual pppd1493 Configure-Request, its correlated
Ze Nak1492, and the replacement1492 request/Ack before successful CHAP/IPCP
and traffic. Literal1500 negotiation remains covered by the existing driver
tests: pppd omits its default1500 option, which is why the wire carrier uses
1493. The separate omitted-option carrier also passed and measured Ze MTU1492.

The full L2TP checker race run passed the original negative cases, the new
correlation cases and `TestLNSLocalMRUProofSurvivesNakStrengthening`, which
retains the original local-policy/CHAP proof. This issue agent ran no code;
all execution and independent test judgment were centralized.

The historical race log retains the test verdicts, but not its exact argv.
Its passing named tests are reported as historical observations. No unchanged
test was rerun to reconstruct that command.

Implementation and carriers are committed as `3cc99259b0`. The two named
logs are under `tmp/session/2026-11-04-222204/scratch/`. Earlier NOT RUN
handoffs are superseded by these results. The spec stays open for closure.

## Central Fault Discrimination (2026-11-06)

The coordinator rebuilt the actual daemon with a private Go overlay disabling
only `negotiatePeerOption`'s upper-MRU guard. The tracked source is unchanged.
The unchanged length/floor checks, FSM, authentication and transmit cap remain
active. Static comparison verified exactly one guard replacement.

The actual command was `./le --name cpe16 job run label cpe16e-nak-disabled-native quiet command env -u NO_BUILD GOFLAGS=-overlay=/home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-04-232740/scratch/cpe16e-f5-nak-disabled-overlay.json ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-mru-xl2tpd SESSION_TIMEOUT=300 ./le --name cpe16 test deployment docker-l2tp-ppp`.
Native job `1f721660` reports the missing MRU1492 Nak. Its actual trace shows
peer request0x1/MRU1493 and Ze Ack0x1/MRU1493, followed by CHAP success and
IPCP addresses. The checker fails at `lnsMRUEvidence` before MTU, traffic or
session-output checks. This is a direct wire semantic red, not a cap-setter
mutation, an assertion change, a panic or a failure to establish PPP.

| Native peer | Consumed fault-run image reference |
|-------------|------------------------------------|
| Ze | `sha256:fc49e4b5d3e053f46c1449d7bf913bd7110a4e732ec6ae1c45ff51982c0da2b3` |
| LAC | `sha256:0aec4aa34b1a6933aeb5118d5c578f24ca91b4bf8e87352e7cb49bb1b053eb0f` |
| RADIUS | `sha256:38b54923cc1c77a6d79916b08ef599dea15bcf2f72b665c9116c3cf7b0e3b8f3` |

The forced run also reports three peer-container removal timeouts. That is
a cleanup failure, separate from the actual semantic failure. A fresh normal
rebuild and complete cleanup remain blocked. The trace and build IDs are in
`job-cpe16e-nak-disabled-native-1f721660.log` under the central scratch directory.

## Blocker

On 2026-11-06, the normal restoration job `c2b9a595` exited1 in native
L2TP preflight: its Docker one-shot exceeded the2-minute limit. It reports
0 passed and0 failed scenarios. It reached neither image rebuilding nor
the Nak scenario, so it proves no normal restoration or protocol failure.
The normal command removed both GOFLAGS and NO_BUILD:
`./le --name cpe16 job run label cpe16e-nak-restored-native quiet command env -u GOFLAGS -u NO_BUILD ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-mru-xl2tpd SESSION_TIMEOUT=300 ./le --name cpe16 test deployment docker-l2tp-ppp`.

The blocked native path is `Docker.RunOneShot`, `forceRemoveContainer` and
`awaitRemoval` in `internal/le/interoplab`. The coordinator's inspection
found the preflight one-shot already exited0 at00:10:09 with PID0, but still
removing. At00:12:50, all three owned fault-run peer containers also had
PID0 and remained removing. No source bug in those lifecycle functions
has been verified. Native cleanup reports its failures honestly.

The source overlay is private and tracked production source is normal.
That does not establish which artifact a later native run consumes. Resume
only after the lifecycle can create and remove the required owned resources.
Rebuild normal images through the native harness, retain their immutable
IDs, observe1493/Nak1492/retry1492/Ack, useful PPP traffic and complete
cleanup. Then obtain the native review artifact and close this same spec.
No global cleanup or daemon restart is authorized by this handback.

## Executive Summary

F-5 remains blocked: fresh normal restoration and cleanup did not run after
the observed direct Nak fault red. The latest failure is Docker preflight,
not PPP negotiation. This spec remains open.

The registered live checker now requires pppd1493, Ze Nak1492, the1492 retry
and its matching Ack before CHAP. The earlier normal native session passed
IPCP, measured MTU1492, RADIUS, ICMP request/reply and PPP/L2TP cleanup.
Implementation is in `3cc99259b0`. Production Nak policy remains in
`pppSession.negPolicy` and `negotiatePeerOption`.

Risks and observations: pppd omits its default1500 option, so the live carrier
uses1493. Explicit1500 remains a separate unit input. Peer1400 publication
is proved by a separate scenario. No whole-tree green or general RFC
conformance claim follows from these scoped results.

## Deliverables Checklist

| Deliverable | Done | Source and evidence |
|-------------|------|---------------------|
| Real oversized peer request, matching Nak and convergence | Yes | `lnsMRUEvidence`, real `l2tp-lns-mru-xl2tpd` trace in `job-cpe16-final-l2tp-live-e6b8eb67.log`. |
| Working subscriber service after convergence | Yes | `checkLNSMRU`: CHAP, client MTU, ping, RADIUS, live daemon answers and `waitL2TPClean`. |
| Missing evidence and incorrect correlation fail | Yes | `TestLNSMRUEvidenceNakSequence`, `TestLNSMRUEvidence`, checker race run. |
| Prior positive trace and explicit1500 remain proved | Yes | `TestLNSLocalMRUProofSurvivesNakStrengthening`; `TestConfiguredMRUNakInformationFloor`. |
| Accurate operator/lab documentation | Yes | `docs/labs/l2tp-interop.md` in `3cc99259b0`, matching wiki `312639d`. |
| Fresh normal restoration and cleanup after direct Nak fault | No | `1f721660` proves the missing-Nak red. Restoration `c2b9a595` stops in Docker preflight before any scenario. |

## Changed-Wire Census and Wiring

| Producer or consumer | Behavior checked |
|----------------------|------------------|
| pppd 2.5.5 `lcp_addci`, `lcp_nakci`, `lcp_printpkt` | Explicit1493 emitted,1492 suggestion accepted, packet identifiers/options decoded. Default1500 omission remains separate. |
| `pppSession.handleLCPPacket`, `evalLCPRequest`, `negPolicy` in `session_run.go`; `negotiatePeerOption` in `lcp_options.go` | Peer MRU1493 exceeds session1492 and produces Nak1492 through the ordinary Driver path. |
| `scenarioCheckerMap`, `plansAt`, `prepareScenario` in `l2tp.go` | Native deployment selection reaches `checkLNSMRU` and starts Ze, real LAC and independent RADIUS. |
| `lnsMRUEvidence`, `lnsLocalMRUAgreement`, `lnsCHAPSuccess` | Both directional agreements and ordered CHAP remain mandatory. Missing/mismatched packets return errors. |
| `checkLNSMRU`, `pingThroughSession`, `commandOutcome`, `waitL2TPClean` | MTU, live daemon output, nonzero ping exit and cleanup all affect the final verdict. |
| Scenario `ppp-options`, `xl2tpd.conf`, `ze.conf`; Dockerfile | Explicit1493, no persist/redial, cap1492/CHAP, pinned real PPPoL2TP peer. No synthetic peer or shell/Python verdict replaces it. |

RFC1661 Section5.3: "If every instance of the received Configuration Options
is recognizable, but some values are not acceptable, then the implementation
MUST transmit a Configure-Nak." Section6.1: "The default value is 1500 octets."
The session1492 maximum is Ze policy, not a universal RFC maximum.

## Security Review Checklist

| Concern | Result | Source-aware evidence |
|---------|--------|-----------------------|
| Malformed or mismatched peer evidence | Pass | `lnsTracePacket` parses an8-bit identifier and requires a closing bracket/options. `lnsMRUEvidence` checks order and identifiers. |
| Authentication bypass or progress without agreement | Pass | Single attempt, no PAP, both LCP agreements before ordered CHAP. Real RADIUS request and live session are required. |
| Injection, path traversal and authorization | Pass | Fixed trace path and argv. REST command ID is parsed uint16 and JSON-quoted by `lnsPolicyCommand`; existing loopback token endpoint. |
| Bounds, race, allocation and denial of service | Pass | Finite trace scans and existing timed waits. No new goroutine, allocation by wire length, credential algorithm or production resource. Central race scope passes. |
| Privilege, information disclosure and cleanup | Pass | Existing NET_ADMIN/PPP lab access and test-only credentials. No new production privilege or doctor dependency. Successful assertions precede LAC stop and both-peer cleanup. |

## Documentation Update Checklist

| Category | Update | Evidence |
|----------|--------|----------|
| Feature declaration | No | `features/l2tpv2-bng.md` remains experimental/partial. This repairs proof of existing policy without changing scope or level. |
| User guide | No | `docs/guide/l2tp.md`, Show Commands: local policy and actual peer MRU remain distinct and correct. |
| Config syntax and examples | No | `ze.conf` retains accepted1492/CHAP policy. Only the external pppd scenario input changes. |
| CLI reference, API/RPC and plugin SDK | No | Existing commands, REST transport and JSON keys are unchanged. |
| Wire format | No | No encoder, parser or packet format changes in F-5. MRU policy is existing production behavior. |
| RFC compliance | No | No RFC tag or claim changes in F-5. Shared runtime proof refresh and audits are completed by the implementation session. |
| Test infrastructure | Yes | `docs/labs/l2tp-interop.md`, MRU and peer-source sections, `3cc99259b0`; wiki `312639d`. |
| Comparison, architecture, metrics, route metadata and SDK rules | No | No feature scope, registry, telemetry or architecture boundary changes. |
| Agent discovery | No | Existing lab/scenario remains on the same discovery and command surfaces. |

The source-anchor census for `lns_policy.go` and `l2tp.go` resolves to the lab
and existing interop architecture pages. Their claims match the producers.
The research guide describes general LCP/MRU behavior and requires no F-5 edit.
The central full doc check reports142 published-site differences and a
discovery warning. Index update `f2aba6ca` and targeted check `dc77857d`
resolve the latter:365 design documents,4206 source anchors,772 packages,
all references valid. The site build remains blocked in foreign inventory
work (`registry` importing `os/signal`). These are disclosed limits, not a
whole-tree documentation pass.

## Implementation Audit

| Requirement / AC | Status | Implementation and direct proof |
|------------------|--------|---------------------------------|
| Task: live oversized request/Nak/convergence | Done | AC-1, `lnsMRUEvidence`; final native peer1493/Nak1492/retry1492/Ack trace. |
| AC-1 | Done | Matching request/Nak IDs and retry/Ack IDs, ordered before CHAP. |
| AC-2 | Historical normal proof complete; fresh restoration blocked | `checkLNSMRU`; `e6b8eb67` passes CHAP/IPCP, client MTU1492, ping, RADIUS and cleanup. `c2b9a595` runs no scenario. |
| AC-3 | Done | `TestLNSMRUEvidenceNakSequence` rejects missing/wrong request, Nak, retry, Ack and early CHAP; prior negatives preserved. |
| AC-4 | Done | `TestConfiguredMRUNakInformationFloor` sends literal1500 and requires Nak1492. |
| Every planned test | Done | Nak test is in `lns_mru_nak_test.go`, prior trace test is additive, existing `TestLNSMRUEvidence` retains its assertions. |
| Every planned file | Done | Checker, fixture tests, scenario options and lab page are committed in `3cc99259b0`. Wiki synchronization is `312639d`. |

## Goal Validation

| Goal | Evidence type | Concrete evidence |
|------|---------------|-------------------|
| Actual wire Nak and useful convergence | Native interop | `job-cpe16-final-l2tp-live-e6b8eb67.log`: request0x1/MRU1493, Nak0x1/MRU1492, retry0x2/MRU1492, Ack0x2, CHAP/IPCP, MTU1492, scenario pass. |
| A missing or unrelated packet cannot pass | Negative and positive unit controls | `TestLNSMRUEvidenceNakSequence` and retained previous local-policy/CHAP trace in the central checker race scope. |
| The default-option limitation does not erase literal1500 coverage | Unit and peer-source inspection | `TestConfiguredMRUNakInformationFloor`; pinned pppd `lcp_addci` default exclusion and `lcp_nakci` acceptance. |
| Direct producer fault must fail, then normal rebuilt service and cleanup must pass | Native semantic discrimination | `1f721660`: guard-disabled image Acks1493 and fails the missing-Nak assertion despite CHAP/IPCP. Fresh normal `c2b9a595` stops in Docker preflight, so this goal remains incomplete. |

## Deviations and Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A default1500 peer request was assumed observable. | pppd omits the MRU option at1500. | Pinned `lcp_addci`, then actual native trace. | Use explicit1493 live, retain explicit1500 unit proof. |
| approach | Skeleton named the new Nak test in `lns_policy_test.go`. | It is in `lns_mru_nak_test.go`. | Independent source review. | Correct the test carrier name without changing its proof. |
| approach | Historical author handoff and absent closure tables left current verification unclear. | Earlier normal scenarios and checker race had passed. | Independent closure read of source and native logs. | Add the evidence and identify historical entries. |
| approach | Earlier live green did not retain a Nak-specific producer red. | Omission/proxy cap mutations do not establish the Nak assertion. | Independent closure evidence review. | Observe direct rebuilt upper-MRU-guard red in `1f721660`; keep this spec blocked until normal restoration and cleanup pass. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Fresh normal restoration and cleanup after the Nak fault | `c2b9a595` stops in Docker preflight before image rebuilding or any scenario. | `plan/release/validation/spec-home-cpe-16e-live-mru-nak.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Not recorded: blocked implementation-evidence handback, no closure prepared. |
| `./le spec review check` | Not run for this blocked handback. |
| Rounds | 1 |
| Reviewer lenses used | Logic/wiring/spec alignment; adversarial correlation/removed behavior; security/edge cases/style. |
| Final independent source result | CLEAN:0 BLOCKER,0 ISSUE. No product fix. Direct semantic red observed; fresh normal restoration and cleanup remain blocked. |

### Findings fixed

| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| NOTE | Stale skeleton statements, test filename and missing closure tables. | This spec. | One closure prose edit. |

All changed Go was read against the style guide, including socket-to-panic,
loop bounds, naming, lifecycle, copies and validation. No new panic, worker,
cache, production resource or data-plane operation is introduced by F-5.
Missing identifiers/options, PAP, extra attempts and absent CHAP fail closed.
The prior complete trace remains a positive local-policy/CHAP control.

## Pre-Commit Verification

### Files Exist (source read)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/l2tp/lns_mru_nak_test.go` | Yes | Read the valid/negative traces and both named test bodies. |
| Existing scenario directory | Yes | Read `ppp-options`, `xl2tpd.conf` and `ze.conf`. |

### AC Verified (source/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Real request and correlated convergence | Read final native `e6b8eb67` trace and `lnsMRUEvidence` source. |
| AC-2 | Authenticated useful session and cleanup | Historical `e6b8eb67` passes. Fresh restoration `c2b9a595` reaches no scenario and does not re-establish this result. |
| AC-3 | Missing or mismatched packets fail | Read all11 Nak-sequence fixtures, retained prior controls and central checker race pass. |
| AC-4 | Literal1500 is still checked | Read `TestConfiguredMRUNakInformationFloor`: explicit1500 request requires Nak1492. |

### Wiring Verified (end-to-end)

| Entry Point | Carrier | Verified |
|-------------|---------|----------|
| `test deployment docker-l2tp-ppp`, selected scenario | Real `l2tp-lns-mru-xl2tpd` directory and `scenarioCheckerMap` | `checkLNSMRU` is registered; source and native run reach it. No new `.ci` is required for this existing deployment carrier. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Pinned pppd emits1493, accepts Nak1492, retries1492 in the actual final peer trace. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Lab's exact Nak sequence and retained service | `lnsMRUEvidence`, `checkLNSMRU`, scenario files and actual native trace. | Yes |
| No guide/config/CLI/API/SDK/wire/architecture/comparison change | Source-anchor search and read of lab, interop architecture, guide and research MRU sections. Existing producer contracts remain unchanged. | Yes |

### Native Checks and Limits

| Check | Observed result |
|-------|-----------------|
| Scoped implementation validation | Historical checker race and normal native run pass. Direct Nak fault `1f721660` fails correctly. Fresh normal `c2b9a595` exits1 before any scenario. |
| Committed flavors | `job-cpe16d` committed compile `9f891c3d`: all five flavors pass after `7e2fb4953f`. |
| RFC proof freshness | Final central check `722fbda6`:76 foreign findings, no owned stale/missing record. Derived RFC update `8119b55c` passes. |
| Repo/weakening prechecks | Central result:9 unused exports and10 unrelated weakening findings. Original3 scoped failures were repaired in `f16fe6136b`. |
| Documentation | Full check `baf5a445` and targeted repair `dc77857d` as disclosed above. |
| Manual and build limits | Real peer sources and owned source anchors were read. Normal fresh Docker rebuilding/cleanup are blocked as above. No full verify or push runs. Foreign site build remains blocked. |
