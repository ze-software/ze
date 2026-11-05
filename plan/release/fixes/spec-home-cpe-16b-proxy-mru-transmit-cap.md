# Spec: Enforce the transmit cap for accepted proxy LCP

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol, docs |
| Priority | P1 |
| Release reason | Accepted proxy LCP can install MTU 1500 or 9000 despite a 1492 session cap. |
| RFC gaps | - |
| Depends | Shared transmit cap landed in `18ea5e830a`; its proof refresh landed in `1a32d7f93d`. Child 16a closed in `074c1265f7` and `b21c1ebcc2`. See `docs/guide/l2tp.md`, PPP negotiation and proxy policy, for the directional MRU contract. |
| Phase | 2/2 — independent review and native gate complete; A/B closure commits remain |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-2 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction. Prove that accepted proxy LCP uses the shared transmit cap even when Last-Received carries peer MRU 1500 or 9000. Preserve valid proxy admission and current authentication checks.

Child 16a owns the shared cap implementation. If it already fixes this path, this spec still owes the proxy regression evidence and independent judgment for protected test changes. Do not add a peer-MRU proxy rejection as the cap fix.

## Executive Summary

Accepted proxy LCP now preserves the peer's receive capacity while the shared
PPP interface setup applies the admitted transmit cap. Peer MRUs 1500, 9000
and omission remain useful sessions at MTU 1492; a smaller peer limit remains
binding. Independent review covers admission, real authentication/NCP,
restart and reload isolation, kernel traffic proof, and the shared accounting
fixture. The native review gate is recorded and clean. The main thread owns
both closure commits.
Risks R-1 and R-2 are controlled by positive service proofs and preserved
authentication assertions. Broader unrelated tooling and published-site reds
are disclosed below and do not establish a global-green result.

## Required Reading

- [x] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`.
  → Constraint: preserve the shared transport-independent PPP setup path.
- [x] `docs/guide/l2tp.md`, PPP negotiation/proxy policy and session output.
  → Constraint: distinguish Last-Sent receive policy, Last-Received peer receive MRU, and effective transmit MTU.
- [x] RFC 2661 Section 4.4.5 and RFC 1661 Section 6.1 from authoritative repository texts.
  → Constraint: verify the original proxy requirements before changing protected expectations or RFC annotations.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/ppp/proxy.go`, `EvaluateProxyLCP` | Parses Last-Sent local MRU and Last-Received peer MRU separately. |
| `internal/component/l2tp/ppp/auth.go`, `pppSession.acceptProxyPolicy` | Checks local receive MRU and allowed authentication method; a larger peer receive MRU is not an admission refusal. |
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.run` / `transmitMRU` / `afterLCPOpen` | Keeps the accepted peer MRU in negotiated state; installs the smaller of that value and the admitted session cap after authentication and NCP. |
| `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go`, `TestProxyLCPPolicy` | The renamed row "larger peer MRU preserves proxy admission" retains every admission/authentication assertion. `TestProxyTransmitMRUCap` separately observes the post-NCP capped interface. |

Preserve authentication policy, admissible proxy sessions, rejection/renegotiation for an invalid local policy, and factual negotiated-state reporting.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Proxy parser/admission | `EvaluateProxyLCP`, `acceptProxyPolicy` | Separates directional MRUs and checks admitted authentication. | Preserve; no new peer-MRU admission guard. |
| Shared MTU installation | `pppSession.afterLCPOpen` | Installs interface MTU after normal or proxy negotiation. | Use child 16a's cap and prove the proxy path reaches it. |
| Driver policy test | `TestProxyLCPPolicy` | Covers admitted methods and fallback to renegotiation. | Strengthen with observed installed MTU while retaining policy assertions. |

## Data Flow

LAC proxy AVPs → `EvaluateProxyLCP` → `acceptProxyPolicy` → negotiated peer state / LCP-up event → authentication and NCP → shared MTU setter → subscriber traffic. Keep admission, reporting and transmit enforcement separate.

## Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | Valid local1492 proxy policy, allowed auth, peer MRU 1500 or 9000 | Admit the proxy session, finish auth/NCP and install MTU 1492; valid-sized data traffic succeeds. | Larger peer receive capacity is safe to admit because local transmit policy still caps sends. |
| AC-2 | Same policy, peer MRU 1400 | Admit and install MTU 1400; negotiation reporting retains the actual peer value. | The smaller peer limit remains binding. |
| AC-3 | Proxy peer MRU omitted | Apply peer default 1500 for negotiated-state meaning and install MTU 1492. | Valid omission remains interoperable within the local cap. |
| AC-4 | Disallowed auth or local MRU outside admitted local policy | Preserve existing renegotiation/refusal behavior and never authenticate using a disallowed method. | Existing permitted-method proxy cases continue to pass because the change concerns transmit size. |
| AC-5 | Accepted proxy session restarts LCP or global policy reloads | Re-establishment uses the admitted session cap; new sessions use accepted new policy. | Existing sessions retain their admitted policy and factual negotiation reporting. |

## Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestProxyTransmitMRUCap` | Driver with accepted proxy input and observed backend setter | Values 1400, 1492, 1500, 9000 and omitted MRU reach post-NCP MTU installation. |
| `TestProxyLCPPolicy` strengthened | Existing policy matrix | Keep admission/auth/negotiated-event assertions and add capped effective MTU coverage. |
| Accepted-proxy live case | Existing L2TP lab; exact scenario selected during design | Observe accepted proxy path, installed interface MTU and useful subscriber traffic; uncapped artifact must fail. |

Independent judgment approved the protected row-name change; `18ea5e830a`
retains its exact `Weakening-approved: ppp.TestProxyLCPPolicy` trailer.
The diff changes only the row name, retaining its inputs, admission result,
method and negotiated-event assertions. The added five-input test proves
effective MTU after real PAP/IPCP; the native carrier proves kernel CHAP/IPCP
and traffic. Central race and live results supersede the original unrun split.

## Files to Modify and Documentation

- `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` and shared driver fixtures: observe installed MTU without losing authentication assertions.
- `internal/component/l2tp/ppp/session_run.go`: only a proxy-path bypass or sibling cap consumer not covered by child 16a; do not duplicate its setter logic.
- L2TP proxy lab fixtures/checkers: live regression proof.
- `docs/guide/l2tp.md`, proxy policy and MRU explanations, corresponding wiki page and affected source anchors: preserve admission while explaining the effective transmit cap.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | The fix rejects oversized peer MRU rather than capping transmit MTU. | No useful admitted session remains in AC-1. | Require positive admitted-proxy and traffic evidence. |
| R-2 | A test change drops authentication or negotiation evidence. | Judge compares old and new assertions. | Independent judgment is mandatory; keep the policy matrix. |
| A-1 | Confirmed: child 16a's shared cap covers accepted proxy sessions. | `run` calls `afterLCPOpen`; driver tests and native kernel measurements observe the capped setter after auth/NCP. | No proxy-specific rejection or second setter is needed. |

## Research Disposition

Research and implementation are complete. RFC 2661's Last-Sent and
Last-Received AVPs retain their directional meaning. RFC 1661 Section 6.1
permits sending less than the advertised receive capacity, so preserving
admission while limiting transmission follows both the source text and the
owner direction in the umbrella. The exact live carrier and approval route
are verified below; dependency closure alone is not credited as F-2 proof.

## Implementation Handoff (2026-11-04)

The owner requires this agent to author changes without compiling or running
code. All execution below is owed by the main thread. No test result is claimed.

| AC | Authored carrier | Observation |
|----|------------------|-------------|
| AC-1, AC-2, AC-3 | `internal/component/l2tp/ppp/proxy_transmit_cap_test.go`, `TestProxyTransmitMRUCap` | Accepted peer MRUs 1400, 1492, 1500, 9000 and omitted option reach actual permitted PAP, IPCP and post-NCP MTU/AdminUp writes; LCP-up retains the factual peer value. A correlated Echo-Reply provides a positive protocol control. |
| AC-1, AC-3 | `test/draft/l2tp/proxy-mru-transmit-cap.ci` and `internal/test/fixture/tunnel_fixture_l2tp_proxy_mru.go` | Reuse the existing SCCRQ/ICRQ/ICCN builder and CHAP/IPCP peer, measure the real kernel interface MTU and return a full-size IPv4 ICMP Echo-Reply for peer MRU 1500, 9000 and omission. StopCCN must remove each PPP interface. |
| AC-4 | Existing `TestProxyLCPPolicy` | All admission/auth/renegotiation assertions remain unchanged. A row-name-only patch is pending independent judgment. |
| AC-5 | `TestProxyTransmitCapAfterLCPRestart` | An accepted jumbo proxy session restarts normal LCP with omitted peer MRU, reauthenticates and renegotiates IPCP, cleans the previous address and installs 1492 again. Existing reactor reload-isolation proof must also run centrally. |

| Shared mechanism | Disposition |
|------------------|-------------|
| F-1's `pppSession.afterLCPOpen` cap | Consumed without another proxy-specific guard or setter. |
| `tunnelL2TPRadiusAccountingPeer` | Its existing handshake was extracted to `tunnelL2TPOpenProxySession`; the accounting caller keeps identical 1500/1500 inputs, CHAP/IPCP behavior and accounting assertions. |
| L2TP PPP data decoding | Extracted once as `tunnelL2TPPPPData` for the accounting and ICMP carriers; `TestTunnelL2TPPPPData` covers the existing LCP form, IPv4, compressed Protocol field and optional-header bounds. |

### Central Verification Owed

| Run | Required evidence |
|-----|-------------------|
| Format the two new Go test files and the new/changed fixture Go files. | Only this task's files are formatted. |
| Tagged PPP race tests matching `TestProxyTransmitMRUCap`, `TestProxyTransmitCapAfterLCPRestart` and `TestProxyLCPPolicy`. | Complete proxy auth/IPCP, capped backend calls and all preservation rows. |
| Tagged fixture test `TestTunnelL2TPPPPData`. | Shared decoder preserves valid accounting and data carriers and rejects malformed bounds. |
| Named-build L2TP functional draft selector `proxy-mru-transmit-cap`. | All three peer inputs pass real kernel MTU and ICMP assertions on Ze's kernel; a rebuilt uncapped setter fails the MTU assertion. |
| Draft stress proof, then promotion and scoped live run. | Main thread follows `ze-test`; this agent does not promote the unrun draft. |
| Existing `radius-acct-wire` carrier. | Shared handshake/decoder extraction preserves the real accounting proof. |
| Documentation check and wiki sync. | The guide's proxy paragraph distinguishes admitted peer capacity from effective transmit MTU. |

The fixture uses a local CHAP credential and the established native LAC wire
fixture; it is kernel functional proof, not independent xl2tpd interop. The
existing xl2tpd LAC does not generate proxy AVPs, so its normal-LCP scenarios
cannot stand in for this accepted-proxy carrier. Scoped external-peer interop
from F-1 remains required alongside this functional proof.

### Native Docker Carrier Handoff (2026-11-05)

The main thread attempted the functional draft on the development host and
reported a missing `NET_ADMIN` capability. It also established that the QEMU
runner boots an Alpine guest, which the owner prohibited. Neither route is
claimed as live evidence. The existing native Docker L2TP suite now carries
the same registered fixture inside its Ze container, on the available Ze
Docker-host kernel. This reuses the suite's staging, readiness, capability,
device, evidence and cleanup mechanisms rather than creating another wire
handshake or runtime harness.

| Authored change | Purpose |
|-----------------|---------|
| `internal/le/interoplab/l2tp/proxy_mru.go` | Run the existing fixture through `LabOperations.Exec` for peer MRU 1500, 9000 and omission. Require measured MTU, full-size ICMP, CHAP, IPCP and StopCCN cleanup evidence; refuse execution errors and nonzero status. |
| `internal/le/interoplab/l2tp/proxy_mru_test.go` | Reject incomplete or mismatched evidence, nonzero status and unavailable helpers; verify all three exact fixture invocations, one Ze peer and declared static helper staging. |
| `internal/le/interoplab/l2tp/l2tp.go`, `.dockerignore`, `.gitignore`, `test/interop-l2tp/Dockerfile.ze` | Register the scenario and stage a separate static `linuxle` helper from the current checkout. Preserve the daemon's existing `ze_core` personality and every other scenario. |
| `test/interop-l2tp/scenarios/l2tp-proxy-mru-transmit-cap/ze.conf` | Admit the fixture's CHAP credential with local MRU 1492 and the real kernel IPv4 pool. |
| `docs/labs/l2tp-interop.md` | Document the native fixture's evidence, helper staging, exact selector and distinction from the independent xl2tpd scenarios. |

Central execution remains owed. Format the new checker and checker-test files
and the shared L2TP registration file. Run the L2TP lab package tests, including
`TestProxyMRUFixtureEvidence`, `TestProxyMRUCheckerExecutesRegisteredFixture`,
`TestProxyMRUScenarioStagesFixtureHelper` and
`TestZeL2TPListenerReachesTheLabBridge`; run the interoplab staging tests
`TestDockerIgnoreAdmitsEveryStagedLabBinary` and
`TestEveryLabStagesTheZeTheKernelCheckRuns`. The exact live selector is:

The exact command is `ZE_L2TP_INTEROP_SCENARIO=l2tp-proxy-mru-transmit-cap ./le --name cpe16 test deployment docker-l2tp-ppp`.

The first run must build the new staged helper and Ze image; do not set
`NO_BUILD`. Retain all three fixture transcripts and suite cleanup evidence.
A separately rebuilt artifact with the old uncapped setter must fail the
actual MTU observation, then the restored implementation must pass again.
The unrun `.ci` remains a draft; this agent has not promoted it, compiled,
formatted or executed code, run Docker, or committed changes.

The shared-handshake regression now has a native carrier too:
`l2tp-radius-acct-wire-fixture` in the same suite. Its separate configuration
preserves the original accounting fixture's 1500/1500 proxy inputs, RADIUS
mock, NAS template, pool, legal 60-second interval and CQM Lost-Carrier Stop.
`radius_fixture.go` invokes BOTH original registered fixtures unchanged,
through detached exec and bounded exit-status polling, sharing a private
`radius-rx.log` directory. The checker requires both fixtures' evidence and
observes the PPP link's deletion. Ordinary exec has a 30-second producer
bound, so it cannot carry the original 60-second accounting wait.

Central verification additionally owes `TestRadiusAccountingFixtureEvidence`,
`TestRadiusAccountingFixtureChecker` and
`TestRadiusAccountingFixtureUsesOriginalRegistrations`, plus:

The exact command is `ZE_L2TP_INTEROP_SCENARIO=l2tp-radius-acct-wire-fixture ./le --name cpe16 test deployment docker-l2tp-ppp`.

The original `radius-acct-wire.ci`, original registered fixtures and their
assertions are retained. The proxy `.ci` draft has an authored native
equivalent; root must choose its disposition after native proof following
`ze-test`. Do not promote an unrun draft or silently claim it passed.

## Mistake Log

| Date | Cause | Correction | Verification |
|------|-------|------------|--------------|
| 2026-11-05 | Independent review found that redirecting directly to `status` could expose an empty file between creation and writing, causing a concurrent poll to report a false failure. | Write the captured worker status to private `status.tmp`, then rename it to `status` after the mock has been stopped and joined. Add a publication-order contract assertion. | Authored without execution; centralized accounting-carrier verification is owed. |
| 2026-11-04 | The restarted-session Echo helper reused the proxy lifetime's Magic-Number although its replacement Configure-Request omitted that option. `handleLCPPacket` correctly recorded peer magic zero and discarded the stale value under RFC 1661 Section 6.4. | Supply the negotiated peer magic to the helper: the proxy's value initially and zero after the empty replacement request. Keep the existing Echo Identifier, length, payload assertions and timeout. | Root's centralized race run reported the timeout; the correction is authored without execution and its centralized rerun is owed. |

## Central Verification (2026-11-05)

This result supersedes the author's unrun handoffs above. The coordinator ran
the full PPP race suite, including proxy MRUs1400/1492/1500/9000/omission,
successful permitted authentication and NCP, raw negotiated-state assertions,
installed MTU and restarted-session Echo. The corrected Echo case passed
without removing any Identifier, payload, length or timeout assertion.

The native `l2tp-proxy-mru-transmit-cap` carrier passed peer MRUs1500,9000
and omission on Ze's kernel. Each case completed CHAP/IPCP, measured actual
MTU1492, received a1492-octet ICMP round trip and removed its interface. A
rebuilt daemon ignoring the admitted cap failed on actual MTU1500; rebuilding
the correct daemon made all three cases pass again. These are semantic
observations of the installed interface, rather than producer reachability.

The original accounting regression also passed through both unchanged
fixture registrations, including decoded Start/Interim/Stop attributes,
Lost-Carrier cause2 and interface removal. The atomic status-publication fix
and its ordering assertion passed the central checker-package tests.

The coordinator independently approved deletion of the ignored proxy `.ci`
draft after verifying that the native carrier invokes the same fixture with
the same three inputs and retains every MTU, traffic and cleanup assertion.
Its local NET_ADMIN skip is not counted green; the verified native carrier
is the maintained replacement. No published test was removed.

Runtime: `18ea5e830a`; live carriers/helper changes: `3cc99259b0`. Evidence:
`job-cpe16-live-proxy-final-18c364fb.log`,
`job-cpe16-live-proxy-cap-disabled-abc3730b.log`,
`job-cpe16-live-proxy-restored-18c364fb.log`,
`job-cpe16-postreview-runtime-race-f3e7163f.log` and
`job-cpe16-final-l2tp-live-e6b8eb67.log`, under
`tmp/session/2026-11-04-222204/scratch/`. Closure review remains separate.

## Deliverables Checklist

| Deliverable | Verification method and result |
|-------------|--------------------------------|
| Accepted large and omitted peer MRU uses the shared cap | `run` retains raw peer MRU; `transmitMRU` and `afterLCPOpen` install the effective minimum. Five driver inputs pass in central race run `f3e7163f`. |
| Smaller peer receive limit remains binding | `TestProxyTransmitMRUCap/smaller_peer` observes raw 1400 and installed MTU 1400 after PAP/IPCP. |
| Admission and authentication remain strict | `TestProxyLCPPolicy` retains every old input and assertion; unknown/disallowed/missing-required authentication renegotiates. |
| Restart and reload isolation | `TestProxyTransmitCapAfterLCPRestart` and `TestReactorPPPPolicyReloadIsolation` pass in `f3e7163f`; old address is removed and the second admitted policy is separate. |
| Actual kernel MTU, useful traffic and cleanup | `checkProxyMRU` runs all three fixture inputs; `18c364fb` passes and `abc3730b` fails on measured MTU 1500 with the cap disabled. |
| Shared accounting behavior preserved | Both original registrations run in `l2tp-radius-acct-wire-fixture`; `e6b8eb67` passes its decoded Start/Interim/Stop and interface-removal assertions. |
| Independent protected-test judgment | The committed approval in `18ea5e830a` matches the name-only `TestProxyLCPPolicy` diff. No protected test is edited during closure. |
| Durable operator and lab documentation | `docs/guide/l2tp.md` and `docs/labs/l2tp-interop.md` match admission, cap, native carrier and independent xl2tpd proof. |

## Security Review Checklist

| Check | Source-aware result |
|-------|---------------------|
| Untrusted proxy input | `EvaluateProxyLCP` validates all three option streams; `acceptProxyPolicy` rejects unknown/disallowed authentication and invalid local policy before publishing proxy Opened. |
| Oversized or omitted peer receive MRU | `transmitMRU` bounds raw peer/default MRU by the admitted cap; `afterLCPOpen` programs that value without reducing receive capacity. |
| Authentication before forwarding | `afterLCPOpen` requires `runAuthPhase` and `runNCPPhase`; the driver proof forbids network allocation before credential acceptance. |
| Bounds and allocation | The fixture parses peer MRU into 16 bits; PPP-data decoding checks optional lengths/offsets before slicing; its ICMP size is bounded by the test's effective MTU. No new product allocation is introduced. |
| Injection, path traversal and privileges | Native fixture argv is fixed apart from validated numeric MRU/port; the accounting worker uses constant commands and a private container directory. Existing lab grants remain scoped to its Ze container. |
| Lifecycle, races and denial of service | Session cap is captured at admission. Restart waits for network withdrawal; fixture polls and worker deadlines are bounded. Worker status is atomically renamed only after its mock is joined. |
| Cryptography and credential disclosure | Existing CHAP/RADIUS test secrets stay test fixtures; no production cryptographic choice, authorization boundary or secret logging is added. |
| Runtime dependency and doctor coverage | Product PPP device/module/socket dependencies are unchanged. The lab uses its existing preflight, static binary staging and container readiness; no new operator-side runtime dependency is added. |

## Documentation Update Checklist

| # | Category | Applies? | Source-aware disposition |
|---|----------|----------|--------------------------|
| 1 | Feature declaration | No for F-2 | `features/l2tpv2-bng.md` retains its partial/experimental declaration; this fix corrects existing PPP service without promoting its level. Its stale MRU floor description belongs to F-4. |
| 2 | Config syntax | No | Existing `l2tp mru` and authentication grammar remain unchanged by the proxy-cap path. |
| 3 | CLI reference | No | No handler, command grammar or output field is added; raw negotiated MRU remains factual. |
| 4 | API/RPC | No | `EventLCPUp` and session output keep the existing raw peer MRU. |
| 5 | Plugin guide | No | Existing local/RADIUS authentication and pool registrations are reused. |
| 6 | User guide | Yes | `18ea5e830a` updates `docs/guide/l2tp.md`, proxy policy and transmit-MTU paragraphs, anchored to `acceptProxyPolicy`, `afterLCPOpen` and `transmitMRU`; wiki `312639d` carries the same contract. |
| 7 | Wire format | No | AVPs 26/27/28 and LCP option encodings are unchanged; the cap changes transmitted size, not encoding. |
| 8 | Plugin SDK | No | No public SDK or process-protocol type changes. |
| 9 | RFC behavior/proof | Yes | Shared proof repair `1a32d7f93d` refreshes 53 records and three audits without changing claims. Native check `722fbda6` leaves 75 foreign findings; native update `8119b55c` completed. Derived public outputs are not committed. |
| 10 | Test infrastructure | Yes | `3cc99259b0` updates `docs/labs/l2tp-interop.md`, native proxy and original accounting carrier sections, with fixture/checker/staging anchors. Existing shared interop architecture remains applicable. |
| 11 | Comparison | No | The existing L2TP/PPP capability and feature level are unchanged. |
| 12 | Architecture | No structural change | `ppp` stays transport-independent; one shared setter applies the cap. `docs/architecture/l2tp/subscriber-session-model.md` keeps its authentication-before-publication contract. |
| 13 | Route metadata | No | No route metadata key is added or changed. |
| 14 | Prometheus metrics | No | No metric declaration or update path changes for F-2. |
| 15 | Registry/inventory | No operator registry change | The native lab checker and compiled test fixture register beside their existing peers; no operator command/plugin/event inventory changes. |
| 16 | Source anchors and design pages | Yes, assessed | The guide and lab anchors match current producers. `docs/research/l2tpv2-implementation-guide.md`, proxy AVP model; `docs/research/l2tpv2-ze-integration.md`, PPP/kernel division; `docs/architecture/testing/interop.md`, existing suite/staging; and the subscriber-session model remain accurate for this change. `docs/guide/pppoe.md` already describes the shared1492 transmit cap and1500 receive capacity. `docs/architecture/l2tp/bng-1-radius-attributes.md` describes unchanged teardown causes and original accounting assertions, both preserved by the companion carrier. Neither related page needs a new F-2 edit. |
| 17 | Examples | Yes, checked | Both native scenario configs retain accepted grammar and explicit CHAP; the proxy case uses MRU1492/IPv6CP off, while accounting keeps its original default1500 policy and legal60-second interval. |

The latest central `./le doc check all` result is `job-cpe16-doc-check-closure-final-baf5a445.log`:
142 published gh-pages catalog/mode disagreements remain. YANG443, usage/help,
source4200/772, rules34, discovery365 and digest3037 checks pass. Site generation
`bba3847b` reaches the foreign inventory import-witness refusal for
`internal/component/plugin/registry` to `os/signal`. These results are not a
global-green claim and are not rerun to renew unchanged evidence.

## Implementation Summary

### What Was Implemented

The shared `pppSession.transmitMRU` bounds transmit MTU after accepted proxy
negotiation without overwriting raw peer state. Driver tests add complete
PAP/IPCP and restarted-session coverage. The native kernel fixture adds real
CHAP/IPCP, measured MTU, full-sized ICMP and StopCCN cleanup for 1500, 9000
and omitted peer MRU. Shared handshake/decoder extraction retains both
original accounting fixture registrations and assertions.

### Bugs Found/Fixed

F-2's oversized installed MTU is fixed in `18ea5e830a`. The existing Mistake
Log records the restarted Echo Magic-Number and atomic status-publication
corrections; their central race/live evidence passes. This independent closure
found no additional product defect.

### Documentation Updates

The guide's admitted-peer/effective-MTU distinction and the lab's two native
carrier descriptions landed with their implementations. The checklist above
records affected anchors and the actual central documentation result.

### Deviations from Plan

The development host could not run the NET_ADMIN draft, and the available QEMU
guest violated the owner's kernel instruction. The maintained native Docker
carrier invokes the same fixture on Ze's kernel, preserving every planned
MTU, traffic and cleanup assertion. The ignored unpromoted draft was removed
only after independent judgment; no published test was deleted. The original
accounting carrier remains unchanged and its native companion preserves its
240-second bound and decoded-wire assertions.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Cap accepted large peer MRU | Done | `session_run.go` `run`, `transmitMRU`, `afterLCPOpen` | Raw1500/9000 survives; installed MTU1492. |
| Preserve smaller and omitted peer semantics | Done | `TestProxyTransmitMRUCap` | 1400 stays1400; omission reports1500 with MTU1492. |
| Preserve admission/authentication | Done | `auth.go` `acceptProxyPolicy`; `TestProxyLCPPolicy` | Only the row name changed; full method/negotiation assertions remain. |
| Preserve restart and reload isolation | Done | `TestProxyTransmitCapAfterLCPRestart`; `TestReactorPPPPolicyReloadIsolation` | Replacement lifetime keeps its admitted cap; new session policy is independent. |
| Prove useful accepted service | Done | `tunnelL2TPProxyMRUTransmitCap`; `checkProxyMRU` | Real CHAP/IPCP, kernel MTU, ICMP and cleanup; uncapped artifact fails. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Five-input driver test and three-case native proxy carrier | Positive auth/NCP/traffic prevents blanket refusal from passing. |
| AC-2 | Done | Driver smaller-peer1400 case | Raw and installed values remain1400. |
| AC-3 | Done | Driver omission case and native peer0 | Default raw1500, installed1492. |
| AC-4 | Done | Unchanged policy matrix | Disallowed/unknown/missing-required auth and invalid local policy renegotiate. |
| AC-5 | Done | Restart and reactor reload-isolation tests | Address withdrawal, reauth/IPCP and replacement installation are observed. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestProxyTransmitMRUCap` | Done | `proxy_transmit_cap_test.go` | Five peer values reach post-NCP setter and Echo. |
| `TestProxyLCPPolicy` | Done | `rfc1661_proxy_policy_driver_test.go` | Independent approval and all preservation rows retained. |
| Native accepted-proxy case | Done | `proxy_mru.go`; `tunnel_fixture_l2tp_proxy_mru.go` | Measured-state green/red/restored-green evidence. |
| Restart/reload tests | Done | PPP driver and reactor tests above | Central `f3e7163f` passes both. |
| Shared data decoder | Done | `TestTunnelL2TPPPPData` | Accounting form, compressed Protocol and optional-header bounds covered. |
| Original accounting companion | Done | `radius_fixture_test.go`; native accounting scenario | Both registrations, fail-closed evidence and atomic publication are preserved. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| PPP proxy policy and new driver tests | Done | `18ea5e830a`; policy assertions retained. |
| Shared `session_run.go` setter | Done | F-1 implementation reused without another setter. |
| Shared and proxy fixture files | Done | `3cc99259b0`; original accounting inputs stay1500/1500. |
| Proxy checker and regression tests | Done | `3cc99259b0`; exact three fixture calls and rejection controls. |
| Accounting checker and regression tests | Done | `3cc99259b0`; original assertions and lifecycle bound retained. |
| Native registration, staging, image and configs | Done | Existing suite, separate static helper, one Ze namespace. |
| Guide, lab page and wiki | Done | Durable behavior and reproduction entry points match producers. |

### Audit Summary

23 audited rows are Done: five requirements, five ACs, six test groups and seven
file groups. None is Partial or Skipped. The maintained native carrier replaces
only the unpromoted draft, as recorded in Deviations.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Useful accepted large/omitted proxy sessions respect1492 | Actual kernel functional proof and semantic fault | `checkProxyMRU` executes1500/9000/0; restored run `18c364fb` reports measured1492,1492-byte ICMP and interface removal for every case. Cap-disabled `abc3730b` observes actualMTU1500 and fails. |
| Preserve strict policy and factual negotiation | Real driver matrix and independent judgment | `TestProxyLCPPolicy` keeps all old assertions; the added full-NCP test independently checks raw1400/1492/1500/9000/default1500 and effective MTU. |
| Keep restart/reload and shared accounting usable | Real driver plus native companion | `f3e7163f` passes restart/reload isolation; `e6b8eb67` passes original decoded accounting Start/Interim/Stop and interface cleanup. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No F-2 implementation or proof remains. | All five ACs and23 audit rows are complete; the machine gate is clean and root executes A/B. | - |
| Feature declaration's stale MRU floor description | This is the separately assigned F-4 protocol-floor documentation, not a proxy-cap scope reduction. | `plan/release/fixes/spec-home-cpe-16d-mru-protocol-floors.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-16b-proxy-mru-transmit-cap-3a525723-d369-4547-9124-fe35463dc0f5.md`; native `c71b71aa` recorded29 paths, CLEAN. |
| `./le spec review check` | Coordinator observed OK:16 code files, clean, hashes match the recorded artifact. Citation-anchor check exits0; its two advisory related pages are now explicitly assessed in checklist row16. |
| Rounds | 1; final independent source verdict CLEAN, 0 BLOCKER, 0 ISSUE. |
| Reviewer lenses used | Logic/wiring/spec; security/negative paths; style/performance/documentation. All ran inline in this independent closure context. |

### Round 1 scope

The whole logical proxy change: shared admission/negotiated-state/setter path,
five-input and restarted-driver tests, retained policy matrix and approval,
reactor reload isolation, shared handshake/decoder and original accounting
invariants, both native checkers and registration/staging/configuration, and
guide/lab/design claims. No code, test or RFC claim changed during closure.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No product BLOCKER or ISSUE. | Whole round1 scope above | No product fix required. |

Record NOTEs were fixed in one edit: stale dependency/current-behavior and
unvalidated-assumption prose now state the actual outcome; the missing closure
checklists/audit are completed; full-path citers are cleared. The F-4 feature
description is assigned to its existing owning spec. These record repairs do
not earn another source-review round.

The Go style pass checked peer-reachable panics, bounds, names, ownership,
duplication, initialization and return types. Changed fixture loops are bounded
by attempts/context; successful setup transfers its socket to a caller that
closes it, while errors close it locally. No new peer-reachable panic or hot-path
product allocation was found. The shared cap adds no proxy-specific layer.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Driver, fixture and checker files named above | Yes | Current files and their producing functions were read during this pass; native configs and Dockerfile staging are present. |
| Original `test/l2tp/radius-acct-wire.ci` | Yes | Read unchanged fixture registrations, decoded-wire assertions and240-second timeout; native companion preserves them. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Accepted large peer fits cap and remains useful | Actual1500/9000 MTU/traffic/cleanup lines in central `18c364fb`; real1500 cap-disabled failure in `abc3730b`. |
| AC-2 | Smaller peer binds | Source and named `smaller_peer` PASS in `f3e7163f`. |
| AC-3 | Omission preserves default/cap distinction | Named driver omission PASS and native peer0 measured1492/traffic/cleanup. |
| AC-4 | Admission/authentication preserved | Current matrix and exact name-only committed diff read; `f3e7163f` names every passing policy row. |
| AC-5 | Lifetime isolation | Current restart test checks address removal and replacement auth/IPCP; named restart/reload PASS lines read in `f3e7163f`. |

### Wiring Verified (end-to-end)

| Entry Point | Carrier | Verified |
|-------------|---------|----------|
| Native `l2tp-proxy-mru-transmit-cap` selector | Existing L2TP suite and registered fixture | `scenarioCheckerMap` registers `checkProxyMRU`; `prepareScenario` selects one Ze container; the checker calls all three exact fixture argv and requires measured-state evidence. |
| Original accounting fixture registrations | `test/l2tp/radius-acct-wire.ci` and native companion | Both original names are retained; `radiusFixtureWorker` joins its mock before atomic status publication and the checker requires original evidence and link removal. |
| Proxy ICCN to effective MTU | Real driver and kernel fixture | `EvaluateProxyLCP` → `acceptProxyPolicy` → raw negotiated state → auth/NCP → `afterLCPOpen` → `SetMTU(transmitMRU())`. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Actual accepted proxy path reaches the shared setter; driver five-input and kernel three-input proof verify the installed result. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Directional MRU and effective transmit cap | `EvaluateProxyLCP`, `acceptProxyPolicy`, `run`, `transmitMRU`, `afterLCPOpen` | Yes; guide and wiki preserve factual raw negotiation. |
| Native functional proof and original accounting preservation | `tunnelL2TPProxyMRUTransmitCap`, `checkProxyMRU`, `radiusFixtureWorker`; original `.ci` | Yes; lab page keeps this distinct from external xl2tpd interop. |
| Design and related pages | Existing AVP model, PPP/kernel division and subscriber acknowledgment model; `docs/guide/pppoe.md` cap/receive paragraph and `docs/architecture/l2tp/bng-1-radius-attributes.md` teardown/accounting descriptions | Yes; shared sizing, termination causes and original accounting assertions remain accurate. The native citation-anchor check exits0. |
| Shared central documentation result | Native `doc check all`, `job-cpe16-doc-check-closure-final-baf5a445.log` | 142 gh-pages-only disagreements disclosed; anchors/command/model/discovery stages pass. |
