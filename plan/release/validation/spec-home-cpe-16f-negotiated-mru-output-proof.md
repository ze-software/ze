# Spec: Home CPE 16f negotiated MRU output proof

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | Equal configured and negotiated MRUs let a config-copy defect pass the live session-output assertion. |
| RFC gaps | - |
| Depends | Aligned peer images in `3cc99259b0`; `docs/labs/l2tp-interop.md`, Peer-source basis |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-6 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which records the original owner direction.
Prove that live session `mru` comes from negotiated peer state using unequal configured and negotiated values.
This skeleton does not claim the output producer is defective; the existing live evidence cannot distinguish that defect.
Earlier race and interop results remain historical review evidence; this split reran neither.

## Required Reading

- [ ] `docs/guide/l2tp.md`, Read commands and PPP negotiation.
  → Constraint: effective config, negotiated peer MRU and installed transmit MTU have distinct meanings.
- [ ] `docs/labs/l2tp-interop.md`, policy output assertions.
  → Constraint: correlate the daemon's session identifier with the actual wire exchange and working subscriber.
- [ ] `docs/architecture/testing/interop.md`, forced-red evidence rules.
  → Constraint: changing session output to a configured constant must fail this proof.

## Current Behavior

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

## Draft Acceptance Criteria

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
| A-1 | Peer MRU1400 is a suitable distinct value. | It is less than1492 and more than IPv6's1280; peer producer not yet checked. | Read pinned pppd source and validate actual negotiation; unvalidated. |
| R-1 | F-5 convergence makes local/peer values equal again. | `spec-home-cpe-16e-live-mru-nak` owns the separate1493-to1492 Nak scenario in `docs/labs/l2tp-interop.md`. | Keep separate attempts or scenarios so each proof remains discriminating. |
| R-2 | Raw peer MRU is renamed implicitly as effective MTU. | 16a/16b introduce transmit capping. | Read snapshot/event producers and document each field before changing expected output. |

## Scope and Next Action

Runtime cap ownership remains with 16a/16b under `plan/release/fixes/`. This spec owns output evidence and any output defect its discrimination proves.
Complete output-producer research, lifecycle carriers, wiring names, source census and template checklists before `ready`; 16h supplies aligned peer images.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|

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
complete; this spec stays open for the separate closure review.
