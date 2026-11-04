# Spec: Enforce the transmit cap for accepted proxy LCP

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, docs |
| Priority | P1 |
| Release reason | Accepted proxy LCP can install MTU 1500 or 9000 despite a 1492 session cap. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-home-cpe-16a-omitted-mru-transmit-cap.md` (shared transmit-cap implementation) |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

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
