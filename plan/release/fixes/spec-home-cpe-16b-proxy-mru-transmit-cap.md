# Spec: Enforce the transmit cap for accepted proxy LCP

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, docs |
| Priority | P1 |
| Release reason | Accepted proxy LCP can install MTU 1500 or 9000 despite a 1492 session cap. |
| RFC gaps | - |
| Depends | Shared transmit cap landed in `18ea5e830a`; its 53 owned RFC records and three audit updates now verify. Child 16a still requires fresh independent formal closure. See `docs/guide/l2tp.md`, PPP negotiation and proxy policy, for the directional MRU contract. |
| Phase | 2/2 — central verification complete; closure review remains |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-2 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction. Prove that accepted proxy LCP uses the shared transmit cap even when Last-Received carries peer MRU 1500 or 9000. Preserve valid proxy admission and current authentication checks.

Child 16a owns the shared cap implementation. If it already fixes this path, this spec still owes the proxy regression evidence and independent judgment for protected test changes. Do not add a peer-MRU proxy rejection as the cap fix.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`.
  → Constraint: preserve the shared transport-independent PPP setup path.
- [ ] `docs/guide/l2tp.md`, PPP negotiation/proxy policy and session output.
  → Constraint: distinguish Last-Sent receive policy, Last-Received peer receive MRU, and effective transmit MTU.
- [ ] RFC 2661 Section 4.4.5 and RFC 1661 Section 6.1 from authoritative repository texts.
  → Constraint: verify the original proxy requirements before changing protected expectations or RFC annotations.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/ppp/proxy.go`, `EvaluateProxyLCP` | Parses Last-Sent local MRU and Last-Received peer MRU separately. |
| `internal/component/l2tp/ppp/auth.go`, `pppSession.acceptProxyPolicy` | Checks local receive MRU and allowed authentication method; a larger peer receive MRU is not an admission refusal. |
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.run` / `afterLCPOpen` | Copies accepted proxy peer MRU into negotiated state and installs MTU without the session-cap bound. |
| `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go`, `TestProxyLCPPolicy` | The row "peer MRU is not local cap" admits local1492/peer1500 and checks the negotiated event, not the installed capped interface MTU. |

Preserve authentication policy, admissible proxy sessions, rejection/renegotiation for an invalid local policy, and factual negotiated-state reporting.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Proxy parser/admission | `EvaluateProxyLCP`, `acceptProxyPolicy` | Separates directional MRUs and checks admitted authentication. | Preserve; no new peer-MRU admission guard. |
| Shared MTU installation | `pppSession.afterLCPOpen` | Installs interface MTU after normal or proxy negotiation. | Use child 16a's cap and prove the proxy path reaches it. |
| Driver policy test | `TestProxyLCPPolicy` | Covers admitted methods and fallback to renegotiation. | Strengthen with observed installed MTU while retaining policy assertions. |

## Data Flow

LAC proxy AVPs → `EvaluateProxyLCP` → `acceptProxyPolicy` → negotiated peer state / LCP-up event → authentication and NCP → shared MTU setter → subscriber traffic. Keep admission, reporting and transmit enforcement separate.

## Draft Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | Valid local1492 proxy policy, allowed auth, peer MRU 1500 or 9000 | Admit the proxy session, finish auth/NCP and install MTU 1492; valid-sized data traffic succeeds. | Larger peer receive capacity is safe to admit because local transmit policy still caps sends. |
| AC-2 | Same policy, peer MRU 1400 | Admit and install MTU 1400; negotiation reporting retains the actual peer value. | The smaller peer limit remains binding. |
| AC-3 | Proxy peer MRU omitted | Apply peer default 1500 for negotiated-state meaning and install MTU 1492. | Valid omission remains interoperable within the local cap. |
| AC-4 | Disallowed auth or local MRU outside admitted local policy | Preserve existing renegotiation/refusal behavior and never authenticate using a disallowed method. | Existing permitted-method proxy cases continue to pass because the change concerns transmit size. |
| AC-5 | Accepted proxy session restarts LCP or global policy reloads | Re-establishment uses the admitted session cap; new sessions use accepted new policy. | Existing sessions retain their admitted policy and factual negotiation reporting. |

## 🧪 Draft Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestProxyTransmitMRUCap` | Driver with accepted proxy input and observed backend setter | Values 1400, 1492, 1500, 9000 and omitted MRU reach post-NCP MTU installation. |
| `TestProxyLCPPolicy` strengthened | Existing policy matrix | Keep admission/auth/negotiated-event assertions and add capped effective MTU coverage. |
| Accepted-proxy live case | Existing L2TP lab; exact scenario selected during design | Observe accepted proxy path, installed interface MTU and useful subscriber traffic; uncapped artifact must fail. |

Obtain independent `ze-test-judge` approval before changing protected expectations, including the named row. Admission may remain accepted; the implication that its installed MTU may exceed the local cap must change. Run scoped PPP/L2TP tests under race. No tests ran for this split.

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
| A-1 | Child 16a's shared cap also covers accepted proxy sessions. | Trace actual `run` to setter and observe MTU. | Unvalidated until the proxy driver/live regression proves it; repair a bypass here. |

## Research Remaining Before Ready

Complete the template, exact live scenario, test-judge route, producer/consumer census and documentation checklists. Verify RFC source text and keep its citations exact. The dependency being fixed is not evidence that this issue is closed.

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

```sh
ZE_L2TP_INTEROP_SCENARIO=l2tp-proxy-mru-transmit-cap ./le --name cpe16 test deployment docker-l2tp-ppp
```

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

```sh
ZE_L2TP_INTEROP_SCENARIO=l2tp-radius-acct-wire-fixture ./le --name cpe16 test deployment docker-l2tp-ppp
```

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
