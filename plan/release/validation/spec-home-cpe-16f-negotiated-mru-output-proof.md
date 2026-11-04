# Spec: Home CPE 16f negotiated MRU output proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, tooling, docs |
| Priority | P2 |
| Release reason | Equal configured and negotiated MRUs let a config-copy defect pass the live session-output assertion. |
| RFC gaps | - |
| Depends | Peer-image alignment: `plan/release/validation/spec-home-cpe-16h-subscriber-radius-pppd.md` |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

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
| R-1 | F-5 convergence makes local/peer values equal again. | `plan/release/validation/spec-home-cpe-16e-live-mru-nak.md` touches the same scenario. | Keep separate attempts or scenarios so each proof remains discriminating. |
| R-2 | Raw peer MRU is renamed implicitly as effective MTU. | 16a/16b introduce transmit capping. | Read snapshot/event producers and document each field before changing expected output. |

## Scope and Next Action

Runtime cap ownership remains with 16a/16b under `plan/release/fixes/`. This spec owns output evidence and any output defect its discrimination proves.
Complete output-producer research, lifecycle carriers, wiring names, source census and template checklists before `ready`; 16h supplies aligned peer images.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
