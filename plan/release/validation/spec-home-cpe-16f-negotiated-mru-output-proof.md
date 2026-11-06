# Spec: Home CPE 16f negotiated MRU output proof

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | Equal configured and negotiated MRUs let a config-copy defect pass the live session-output assertion. |
| RFC gaps | - |
| Depends | Aligned peer images in `3cc99259b0`; `docs/labs/l2tp-interop.md`, Peer-source basis |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-06 |

## Task

Own finding F-6 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Prove that live session `mru` comes from negotiated peer state using unequal configured and negotiated values.
The implemented proof distinguishes a configuration copy from negotiated output. The unchanged normal producer is correct for this case; a rebuilt semantic break of that producer fails the live proof.

## Required Reading

- [x] `docs/guide/l2tp.md`, Read commands and PPP negotiation.
  → Constraint: effective config, negotiated peer MRU and installed transmit MTU have distinct meanings.
- [x] `docs/labs/l2tp-interop.md`, policy output assertions.
  → Constraint: correlate the daemon's session identifier with the actual wire exchange and working subscriber.
- [x] `docs/architecture/testing/interop.md`, forced-red evidence rules.
  → Constraint: changing session output to a configured constant must fail this proof.

## Current Behavior (before implementation)

| Source checked | Behavior |
|----------------|----------|
| `internal/le/interoplab/l2tp/lns_policy.go`, `checkLNSPolicyOutput` | Requires effective configured MRU1492, fetches one session and queries its detail. |
| Same file, `lnsNegotiatedSession` | Requires session MRU1492 and CHAP-MD5. |
| `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/ze.conf` and `ppp-options` | Both configured local MRU and peer requested MRU are 1492. |
| `internal/le/interoplab/l2tp/lns_policy_test.go`, `TestLNSMeasuredOutput` | Rejects some wrong outputs but uses the same1492 positive value as config. |

## Existing Mechanisms

| Mechanism | Where | Direction |
|-----------|-------|-----------|
| Daemon command path | `lnsPolicyCommand` and `lnsPolicySessions` in `lns_policy.go` | Retain the real registered command path and specific session lookup. |
| Peer trace and traffic | `checkLNSMRU` in `lns_policy.go` | Correlate with unequal values instead of creating a second command harness. |

## Data Flow

Local config and peer options → LCP exchange → session event/snapshot → `show l2tp sessions` and `show l2tp session id` → typed live checker.
During design, read the actual output producer and define expectations separately from effective transmit MTU after 16a/16b.

## Acceptance Criteria

| ID | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| AC-1 | Local MRU1492; peer negotiates a different supported MRU, for example1400. | Effective config remains1492; session list/detail report the peer's actual MRU from the correlated wire exchange. | The smaller-MRU subscriber authenticates and carries traffic within its size limit. |
| AC-2 | Session output is replaced by the configured1492 value. | Checker fails despite otherwise successful negotiation and traffic. | The same session with the actual negotiated value passes. |
| AC-3 | Negotiation lifecycle and authentication are observed. | Preserve null negotiated fields before LCP Opened/after LCP down and agreed `auth-method` while open. | Explicit negotiated no-auth remains distinct from null where policy permits it. |
| AC-4 | A local cap differs from the peer's requested receive limit. | Evidence states which value each output/interface field represents and checks the installed MTU separately. | A valid capped session remains admitted; output does not disguise policy as negotiation. |

## 🧪 TDD Test Plan

| Planned test / carrier | Proof |
|------------------------|-------|
| `TestLNSMeasuredOutputUnequalMRUs` in `lns_policy_test.go` | Correct unequal values pass; a copied config value fails. |
| `TestLNSPolicyCheckersReachDaemon` | Preserve specific live-session lookup and real command traversal. |
| Live `l2tp-lns-mru-xl2tpd` | Correlated peer trace, unequal values, daemon state, interface measurement and traffic. |

Record forced red with the actual snapshot producer returning configured MRU in a rebuilt artifact, then restored green. Obtain independent `ze-test-judge` approval for protected test changes.

## Files to Modify

- `internal/le/interoplab/l2tp/lns_policy.go` and `lns_policy_test.go` — unequal-value assertions and negative fixtures.
- `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/` — distinct peer/local input or a separate registered attempt that preserves F-5.
- `docs/labs/l2tp-interop.md` — state what live session output proves; verify the `docs/guide/l2tp.md` field description and corresponding wiki text against its producer.

## Risks & Assumptions

| ID | Risk / assumption | Early signal or basis | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| A-1 | Peer MRU1400 is a suitable distinct value. | It is less than1492 and more than IPv6's1280. | Confirmed: the pinned pppd emits explicit MRU1400; the normal live trace acknowledges it, CHAP succeeds and traffic crosses the session. |
| R-1 | F-5 convergence makes local/peer values equal again. | `spec-home-cpe-16e-live-mru-nak` owns the separate1493-to1492 Nak scenario in `docs/labs/l2tp-interop.md`. | Keep separate attempts or scenarios so each proof remains discriminating. |
| R-2 | Raw peer MRU is renamed implicitly as effective MTU. | 16a/16b introduce transmit capping. | Read snapshot/event producers and document each field before changing expected output. |

## Scope

Runtime cap ownership remains with 16a/16b under `plan/release/fixes/`. This spec owns output evidence and any output defect its discrimination proves.
The separate negotiated-MRU carrier preserves the oversized-Nak and omitted-option carriers. Peer-image alignment landed in `3cc99259b0`; the durable peer-source account is `docs/labs/l2tp-interop.md`, Peer-source basis.

## Implementation Handoff (2026-11-04)

The owner authorized this agent to author changes and the root session to
centralize review, formatting, builds and execution. This agent ran no code,
compiler, formatter, linter, test, daemon, Docker command or commit.

| Surface | Source-backed decision / authored change |
|---------|------------------------------------------|
| Output producers | `reactor_kernel.go` retains the negotiated MRU in `handleLCPUp` and clears it in `handleLCPDown`; `snapshot.go` publishes that field; `cmd/l2tp.go` reports null when it is zero. These producers remain unchanged. |
| Peer behavior | Pinned pppd 2.5.5 `lcp_addci` emits MRU when it differs from the default 1500, so the new peer requests 1400 explicitly. No upstream runtime result is assumed. |
| Independent carrier | New registered `l2tp-lns-negotiated-mru-xl2tpd` uses configured local1492 and peer1400. It preserves the separate oversized-Nak and omitted-option scenarios owned by child16e. |
| Wire evidence | `lnsDistinctMRUEvidence` requires both directional request/Ack pairs with matching identifiers and CHAP success after the agreements. |
| Measured values | `checkLNSNegotiatedMRU` measures Ze's interface MTU1400, checks useful traffic and backend reachability, then requires aggregate and detail session MRU1400 while effective config remains1492. |
| Shared machinery | Reuse child16e's parameterized daemon helpers and factor the existing MTU parser into `lnsMeasuredLinkMTU`; preserve the existing1492 wrapper and its test expectations. |

| Authored test | Outcome for root to verify |
|---------------|----------------------------|
| `TestLNSDistinctMRUEvidence` | Correct unequal directions pass; equal/corrupt/unacknowledged inputs, wrong local policy, missing credentials, PAP and redial fail. |
| `TestLNSMeasuredOutputUnequalMRUs` | Registered full checker reaches effective config, aggregate/detail session commands and Ze interface; either session view copying config1492 fails. |
| `TestLNSDistinctMRUScenarioPreparation` | Discovery, kernel preflight, checker, independent RADIUS image and peer ordering are wired. |
| Existing `TestSessionSnapshotNegotiatedPPP` | Preserve actual event-consumer and lifecycle clearing coverage, including negotiated `none`. |
| Existing `TestSessionJSONMRUAndAuthMethod` | Preserve detail/list JSON null and no-auth semantics. |

Root must format the new files, run tagged race tests, run the new live
scenario, and record forced red with the actual snapshot producer returning
configured MRU in a rebuilt artifact followed by restored green. No old test
expectation was changed by this agent. The corresponding wiki update and live
verification remain root-owned. Claims in this handoff describe authored
assertions, not tests that have passed.

## Central Verification (2026-11-05)

The coordinator ran the new and existing checker tests under race, including
both copied-value negatives and existing null/down/no-auth lifecycle cases.
The separate registered `l2tp-lns-negotiated-mru-xl2tpd` scenario passed with
configured MRU 1492, correlated wire/session MRU 1400, actual Ze MTU 1400,
both command views, RADIUS credentials and useful subscriber traffic.

For semantic discrimination the coordinator rebuilt the actual daemon with
one hand-written change in `l2tpReactor.handleLCPUp`: publish `r.params.MRU`
instead of the event's negotiated value. Wire negotiation, the driver, MTU
and traffic remained independent of that copied field. The live carrier
failed after those stages with aggregate MRU 1492 versus the peer's 1400.
This is an actual publication-path defect, rather than injected console text
or a producer halted before it could publish state. Unit negatives separately
prove that copying the value into either command view is rejected.

Runtime/carriers are committed in `18ea5e830a` and `3cc99259b0`; matching
wiki documentation is `312639d`. Evidence:
`job-cpe16-final-l2tp-live-e6b8eb67.log`,
`job-cpe16-postreview-runtime-race-f3e7163f.log` and
`job-cpe16-copied-output-red-1d421026.log`, under
`tmp/session/2026-11-04-222204/scratch/`. The issue agent authored the test
and private fault artifact without execution. The final rebuilt normal
artifact passed both output views again with configured 1492 and negotiated
1400, alongside MTU and traffic assertions, in
`job-cpe16-final-artifacts-restored-718e9ad2.log`. Central verification is
complete; the independent closure review below is also complete.

## Deliverables Checklist

| Deliverable | Result | Evidence |
|-------------|--------|----------|
| Unequal live output proof | Done | `lns_negotiated_mru.go`, `checkLNSNegotiatedMRU`: wire1400, Ze MTU1400, traffic, RADIUS and both session views; normal live pass. |
| Configuration-copy discrimination | Done | `l2tpReactor.handleLCPUp` semantic break publishes configured1492; native checker fails with aggregate1492 versus1400. Restored normal artifact passes. |
| Lifecycle and authentication preservation | Done | `TestSessionSnapshotNegotiatedPPP` and `TestSessionJSONMRUAndAuthMethod` pass with unknown/down, PAP, CHAP and explicit `none` assertions intact. |
| Discoverable carrier and documentation | Done | `l2tp.go`, `scenarioCheckerMap`, `prepareScenario`, `imageBuilds`; new scenario directory; lab page in `3cc99259b0`, guide in `18ea5e830a`, wiki in `312639d`. |

## Changed-Wire Census and Wiring

The implementation changes proof inputs and consumers, not the runtime wire or output contract. Source searches for `NegotiatedMRU`, `negotiatedMRU`, `sessionJSON`, `lnsNegotiatedSession`, and the scenario stem cover Go producers/consumers and the independent pppd/xl2tpd configuration carriers.

| Producer or consumer | Location and symbol | Review result |
|----------------------|---------------------|---------------|
| Normal and proxy negotiation event | `ppp/session_run.go`, `pppSession.applyTransition`, `pppSession.run` | Event retains raw peer MRU independently of the transmit cap. Unchanged by this proof. |
| Reactor lifetime | `reactor_kernel.go`, `handleLCPUp`, `handleLCPDown` | Stores actual event MRU; clears it on down under the reactor lock. |
| Snapshot and JSON | `snapshot.go`, `sessionSnapshot`; `cmd/l2tp.go`, `sessionJSON` | Copies actual session state; zero becomes null in both command views, while negotiated `none` remains distinct. |
| Independent wire writer/reader | New scenario `ppp-options`, `xl2tpd.conf`, `ze.conf`; `lnsDistinctMRUEvidence` | Peer1400 and Ze1492 are separate; both directional requests and matching Ack identifiers precede CHAP success. Exactly one attempt is required. |
| Command and interface consumers | `lns_policy.go`, `checkLNSPolicyOutputForMRU`, `lnsNegotiatedSessionMRU`, `lnsMeasuredLinkMTU` | Typed config remains1492, either session view must be1400, measured Ze interface must be1400. Invalid, null or unavailable state fails. |
| Entry point | `l2tp.go`, `RunAt`, `plansAt`, `scenarioCheckerMap`, `prepareScenario`, `imageBuilds` | Native deployment selector discovers the directory, runs kernel preflight and starts independent RADIUS, Ze and real LAC. `TestLNSDistinctMRUScenarioPreparation` checks registration. |

No shell or Python parser was replaced. Existing daemon command registrations, REST authentication, output keys and PPP runtime dependencies remain unchanged. The new scenario uses the existing lab dependency/preflight path.

## Security Review Checklist

| Concern | Result | Source evidence |
|---------|--------|-----------------|
| Injection, traversal and authorization | Clear | `lnsPolicyCommand` JSON-quotes the command and passes separate wget arguments to the existing loopback token-guarded endpoint. The selected detail identifier is parsed as uint16, not peer-supplied shell text. New files are fixed lab configuration. |
| Missing/untrusted state, bounds and overflow | Clear | `lnsPolicyCommand`, `lnsNegotiatedSessionMRU`, `lnsMeasuredLinkMTU`, `lnsTracePacket` reject unread/null/malformed data, wrong counts, missing fields and wrong values; identifiers parse with an 8-bit limit. No new wire parser or peer-sized allocation enters the daemon. |
| Concurrency and lifetime | Clear | `handleLCPUp`, `handleLCPDown`, `Snapshot` retain their lock contracts. New checker calls are sequential and context/timeout bounded; no worker, cache or concurrent owner is introduced. |
| Secrets, cryptography and privilege | Clear | New carrier uses existing isolated lab credentials, RADIUS and NET_ADMIN/PPP setup. It changes no authentication policy, cipher, product permission or production logging. |
| Resource exhaustion, cleanup and information exposure | Clear | New checker scans the finite captured trace, requires one connection, returns errors, then checks existing `waitL2TPClean` teardown. Retained lab evidence uses the existing diagnostic path. No security fix was needed, so no new execution is owed. |

## Documentation Update Checklist

| Category | Update? | Source-aware evidence |
|----------|---------|-----------------------|
| Feature declaration and comparison | No | `features/l2tpv2-bng.md` retains the same capability/scope; this proof adds no supported protocol or maturity claim. `checkLNSNegotiatedMRU` proves existing measured output. |
| User guide | Yes | `18ea5e830a` updates `docs/guide/l2tp.md`, Read commands: raw peer MRU, local cap, null lifetime and `none`; verified against `handleLCPUp/Down`, `sessionSnapshot`, `sessionJSON` and `afterLCPOpen`. Wiki companion is `312639d`. |
| Config syntax | No | New `ze.conf` uses existing MRU/auth/API leaves. No YANG or parser change belongs to this proof. |
| CLI reference | No | `cmd/l2tp.go`, `sessionJSON`, `handleSessions`, `handleSession` retain the commands and keys; the existing guide description is accurate. |
| API/RPC and plugin guide/SDK rules | No | `lnsPolicyCommand` consumes the existing REST envelope; no RPC type, event contract or SDK registration changes. |
| Wire format | No | `pppSession.applyTransition` and `run` still produce the same LCP/event values; the test selects a different existing valid MRU. |
| RFC compliance | No new row/tag | RFC1661 Section6.1 says: "This Configuration Option may be sent to inform the peer that the implementation can receive larger packets, or to request that the peer send smaller packets." This proof checks existing operational output, adds no RFC-tagged unit and claims no new whole-requirement coverage. Existing requirement records remain separately checked. |
| Test infrastructure and agent discovery | Yes | `3cc99259b0` adds the new scenario account and source anchors in `docs/labs/l2tp-interop.md`. Existing `ai/INDEX.md` L2TP lab entry reaches it; no new command or test runner is introduced. |
| Architecture, route metadata and counters | No | No component boundary, metadata key, counter or telemetry label changes. `lnsMeasuredLinkMTU` factors the existing parser while retaining the1492 wrapper. |

The source-anchor search over `docs/` for `lns_policy.go`, `lns_negotiated_mru.go`, `lns_negotiated_mru_test.go` and `l2tp.go` identifies the lab page plus existing guide/architecture claims. The lab's affected claims match the reviewed producers. Native index check `dc77857d` reports365 design docs,4206 code paths and772 packages, all references valid. The complete doc check still reports142 foreign gh-pages drifts; its stale-discovery warning was repaired by index update `f2aba6ca`. This is scoped documentation validity, not a claim that every global check is green.

## Implementation Summary

### Executive Summary

The unequal-value output proof is implemented and tested through the live daemon. Independent source review found no blocking defect. The closure claim covers F-6 only.

Risks and observations: R-1 remains controlled by separate Nak, omitted-option and smaller-peer scenarios. R-2 remains controlled by separate raw negotiated MRU, effective config and measured interface assertions. Historical full argv/image identities are unavailable; the evidence limits below preserve that distinction. Foreign native gate failures remain disclosed, and F-5 still owes its own normal restoration proof.

### What Was Implemented

`3cc99259b0` adds `checkLNSNegotiatedMRU`, its registered scenario and additive negative fixtures. It keeps configured1492, peer1400 and installed1400 as separate observations. Shared daemon and MTU helpers retain their original1492 callers. Existing product output producers and lifecycle tests are preserved.

### Bugs Found/Fixed

The equal-value proof was blind to a configuration copy. The unequal-value carrier closes that gap. No new product output defect was found: the deliberately broken rebuilt producer fails and the restored producer passes.

### Documentation Updates

The guide, lab and wiki updates are the committed updates listed above. Their claims name the actual producer and distinguish raw negotiation from transmit policy. Closure adds no public support claim.

### Deviations from Plan

The new tests live in `lns_negotiated_mru_test.go` beside their checker rather than enlarging `lns_policy_test.go`. A separate registered scenario preserves F-5's independently discriminating Nak/omission inputs. The existing lifecycle tests needed no changes. No protected test assertion was lowered by F-6.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Equal configured/session1492 could pass a configuration-copy defect. | Distinct1492/1400 inputs make the producer observable. | `TestLNSMeasuredOutputUnequalMRUs` and the actual rebuilt `handleLCPUp` mutation. | Retain the separate carrier and both command-view negatives; no additional lesson record is needed. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Own F-6 and prove negotiated output using unequal inputs | Done | `lns_negotiated_mru.go`, `checkLNSNegotiatedMRU` | Real daemon, independent peer and measured interface, with producer mutation discrimination. |
| Preserve lifecycle and agreed authentication output | Done | `snapshot_test.go`, `TestSessionSnapshotNegotiatedPPP`; `cmd/l2tp_test.go`, `TestSessionJSONMRUAndAuthMethod` | Existing tests and normal source remain intact. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Normal native selected scenario; `TestLNSMeasuredOutputUnequalMRUs` | Config1492; one live authenticated session, both views1400 and correlated peer1400. |
| AC-2 | Done | Actual `handleLCPUp` semantic mutation and restored native pass | Aggregate1492 fails after independent wire/MTU/traffic checks; unit cases reject either view separately. |
| AC-3 | Done | Existing snapshot/JSON tests pass in observed race run | Null before Opened/down and explicit `none` are preserved; live open case is CHAP-MD5. |
| AC-4 | Done | `lnsMeasuredLinkMTU`, normal trace and command output | Ze's own MTU1400 is separately measured; it is not substituted for raw peer MRU or config. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLNSMeasuredOutputUnequalMRUs` | Done | `lns_negotiated_mru_test.go` | Positive and independent list/detail/interface copy negatives pass. |
| `TestLNSPolicyCheckersReachDaemon` | Done | `lns_policy_test.go` | Existing daemon traversal assertions retained and pass. |
| Live `l2tp-lns-mru-xl2tpd` | Changed | Separate `l2tp-lns-negotiated-mru-xl2tpd` | F-5 retains the Nak carrier. F-6 uses the new live carrier and its observed red/green pair. |
| Additive wire and registration tests | Done | `TestLNSDistinctMRUEvidence`, `TestLNSDistinctMRUScenarioPreparation` | Wrong identifiers, missing Ack/policy/CHAP, PAP, redial and missing registration are rejected. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `lns_policy.go` and `lns_policy_test.go` | Done | Parameterized helpers preserve old callers; new dedicated checker/tests carry the unequal case. |
| Scenario directory | Changed | New `l2tp-lns-negotiated-mru-xl2tpd/{ze.conf,ppp-options,xl2tpd.conf}` instead of replacing F-5 input. |
| Guide, lab and wiki | Done | `18ea5e830a`, `3cc99259b0`, `312639d`; producing functions checked. |

### Audit Summary

13 items: 11 Done, 2 Changed as documented, 0 Partial, 0 Skipped. Both Changed rows preserve the required proof in a dedicated carrier rather than reduce scope.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Live `mru` derives from peer negotiation | Native interop | Restored normal selected scenario passes with wire1400, config1492, both session views1400 and Ze MTU1400; RADIUS and traffic are prerequisites. |
| A copied config value cannot pass | Producer mutation plus checker tests | Actual `handleLCPUp` mutation to configured1492 fails `aggregate session negotiation: daemon negotiated peer MRU is 1492, want client's 1400`; independent unit negatives cover detail-only and aggregate-only copies. |
| Existing lifecycle/authentication remains visible | Named race tests | `TestSessionSnapshotNegotiatedPPP` and `TestSessionJSONMRUAndAuthMethod` PASS, including unknown/down and negotiated `none`; live CHAP passes. |

## Evidence Provenance and Limits

Canonical reproduction is the registered scenario selected with `ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-negotiated-mru-xl2tpd` through `./le --name cpe16 test deployment docker-l2tp-ppp`, as documented in the lab page. This is a reproduction recipe, not a claim that the original full argv is retained. The coordinator observed the native runs; the independent closure reader checked their output and the current producing source without executing them again.

Historical observations: `f3e7163f` contains PASS for all three new tests, daemon traversal and both existing lifecycle tests; the exact test argv is not retained, so no invented command or new run is claimed. `1d421026` records the rebuilt configuration-copy producer's semantic failure. `718e9ad2` records the rebuilt normal selected scenario's pass with both views1400, config1492 and actual interface1400. Historical image IDs and complete invocation arguments are not retained. Session logs are inspection material, not retained standalone proof; the canonical source, named tests, carrier and committed validation account bound these claims. The original log names remain in Central Verification above for provenance.

The coordinator's final native RFC check `722fbda6` reports 76 foreign findings and no stale record among the 54 owned discrimination records and three owned audits. Native RFC update `8119b55c` passes. This F-6 change adds no tagged claim. Repo check's nine unused findings and weakening check's ten findings are foreign; the original three tooling-package test reds were fixed in `f16fe6136b`. Site generation still fails on the foreign registry-to-`os/signal` external-import witness. F-5's later Nak-only fault experiment and normal-restoration proof belong to `spec-home-cpe-16e-live-mru-nak`; they do not erase the earlier observed F-6 semantic red/normal green pair. No whole-tree, all-scenario or full-RFC success is claimed.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None in F-6 scope | All four acceptance criteria are satisfied; sibling F-5 restoration is not this output proof. | None |

## Review Gate

| Field | Value |
|-------|-------|
| Round 1 scope | Whole logical F-6 change in `3cc99259b0`, shared reached helpers/registrations and unchanged negotiation-event/snapshot/JSON/lifecycle producers; current closure prose and umbrella citer. |
| Independent reviewer | Closure phase agent, distinct from the author; all lenses inline and no child agents. |
| Reviewer lenses used | Wiring/logic/guard discrimination; security/edge cases/style; specification/goal/lifecycle/documentation. |
| Source verdict | CLEAN: 0 BLOCKER, 0 ISSUE. No source fix required. |
| Artifact | `tmp/review/home-cpe-16f-negotiated-mru-output-proof-3a525723-d369-4547-9124-fe35463dc0f5.md`; coordinator native run `8c37dcf2` records 19 files, CLEAN, 1 round, reviewer `close_16f-Codex-GPT6`. |
| `./le spec review check` | PASS in `8c37dcf2`: 12 code files match the clean review hashes. Native citation-anchor run `7c7ad749` passes with no advisory. |
| Rounds | 1 |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| None | - | No BLOCKER or ISSUE found. | Whole round 1 scope | No source fix required. |

NOTE: Missing closure checklists and stale skeleton wording were corrected in one prose completion edit; no product defect or extra review round was created.

Provenance NOTE: the native artifact identifies the actual Codex GPT-6 reviewer explicitly. Its automatic model header comes from the historical Claude transcript selected by `model.go`, `CurrentModel`, and does not identify this reviewer. The artifact records this limitation; the unchanged source verdict remains CLEAN in one round.

The style pass covers the F-6 Go additions and reached shared helpers: no peer-reachable panic, new worker, queue, cache or unbounded retry; finite trace scans and context-bound waits; no product hot-path allocation or lifecycle type change. Nil/missing JSON, wrong cardinality, uint16/identifier boundaries, malformed trace, equal values, independent list/detail errors and both configured/negotiated values were checked at their deciding consumers. Existing1492 wrappers and lifecycle assertions remain in place.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| New checker/test and all three scenario files | Yes | Source read of `lns_negotiated_mru.go`, `lns_negotiated_mru_test.go`, `ze.conf`, `ppp-options`, `xl2tpd.conf`; tracked in `3cc99259b0`. |
| Existing lifecycle carriers | Yes | Source read of `snapshot_test.go`, `TestSessionSnapshotNegotiatedPPP`, and `cmd/l2tp_test.go`, `TestSessionJSONMRUAndAuthMethod`. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Unequal config and actual output | Closure source read of `checkLNSNegotiatedMRU`/`lnsNegotiatedSessionMRU`; normal native retained output has1492/1400. |
| AC-2 | Copies fail | Closure read of actual mutation failure1492 versus1400 and independent list/detail negative cases; restored carrier passes. |
| AC-3 | Lifecycle/auth preserved | Closure read of both producing functions and unchanged snapshot/JSON tests; named PASS output in observed race log. |
| AC-4 | MTU separate | Closure read of `lnsMeasuredLinkMTU` call on peerZe and actual native link JSON1400. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Native deployment selector for `l2tp-lns-negotiated-mru-xl2tpd` | Native interop scenario directory; no new .ci | `RunAt` → `plansAt` → registered checker → actual REST config/list/selected-detail commands and peerZe link measurement; preparation test and native pass. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Pinned pppd emits1400; actual request/Ack trace, authenticated session and traffic pass. R-1 mitigated by separate scenarios; R-2 mitigated by independent raw-session/config/interface assertions. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide's negotiated/raw/null distinctions | `handleLCPUp/Down`, `sessionSnapshot`, `sessionJSON`, `afterLCPOpen` | Yes; committed guide and wiki reflect these producers. |
| Lab's independent unequal-value proof | `checkLNSNegotiatedMRU`, `lnsDistinctMRUEvidence`, `TestLNSMeasuredOutputUnequalMRUs` | Yes; committed lab subsection and scenario listing are accurate. |
| All No categories | Source-anchor search and current source census in Documentation Update Checklist | No config/command/wire/SDK/metadata/counter/architecture contract changes introduced by F-6. |
| Native documentation validity | Coordinator index check `dc77857d` | 365 design docs/4206 paths/772 packages valid; 142 foreign site drifts remain disclosed. |
