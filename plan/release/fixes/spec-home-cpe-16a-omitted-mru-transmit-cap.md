# Spec: Cap transmit MTU when the peer omits MRU

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, config, docs |
| Priority | P1 |
| Release reason | A working subscriber can transmit packets larger than an accepted 1492 session limit. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

## Task

Own finding F-1 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction and review evidence. A peer that omits MRU must establish a useful session whose transmit MTU respects its admitted session cap. L2TP configured to 1492 and PPPoE's fixed 1492 cap must both hold.

This spec owns the shared transmit-cap implementation. F-2's proxy regression and protected test judgment belong to `plan/release/fixes/spec-home-cpe-16b-proxy-mru-transmit-cap.md`; implement one shared cap, not a second proxy-specific policy.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`.
  → Constraint: keep transport-independent PPP setup shared between L2TP and PPPoE.
- [ ] `docs/guide/l2tp.md`, PPP negotiation and reload; `docs/guide/pppoe.md`, MRU ceiling.
  → Constraint: preserve admission-time policy isolation and correct each cap description with its implementation.
- [ ] RFC 1661 Section 6.1, RFC 2516 Section 7 and RFC 1332 in the repository authoritative texts.
  → Constraint: distinguish transmit limits from required receive capacity; verify existing requirement ownership before adding RFC tags.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.afterLCPOpen` | Falls back to 1500 for absent negotiated MRU, then installs the larger of that value and 68. It does not bound interface MTU by the session cap. |
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.handleLCPPacket` | An accepted peer request without MRU records the PPP default of 1500. |
| `internal/component/l2tp/ppp/manager.go`, session construction | Defaults an absent `StartSession.MaxMRU` to 1500 and stores the session cap separately. |

Preserve authentication allow-list enforcement, receive capacity, valid subscriber traffic, and the distinction between negotiated peer MRU and installed transmit MTU.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Interface setup after NCP | `pppSession.afterLCPOpen` | Installs MTU and brings the PPP interface up. | Extend this shared path with the admitted session's transmit cap. |
| Session policy snapshot | `StartSession.MaxMRU` and `pppSession.maxMRU` | Retains the session's admitted policy. | Use existing state rather than reading current global config. |

## Data Flow

Transport config/default → admitted session cap → normal LCP peer MRU or PPP default → post-NCP interface MTU installation → kernel subscriber traffic. Trace restart, reload, userspace transmit helpers and every MTU writer before design approval. A negotiated peer value remains factual even when local policy imposes a smaller transmit MTU.

## Draft Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | L2TP cap 1492; peer omits MRU | Authentication and IPCP complete, installed MTU is 1492, and traffic succeeds without transmitting an Information field larger than 1492. | The session remains admitted because omitted MRU supplies a peer receive default, not permission to ignore local policy. |
| AC-2 | PPPoE peer omits MRU | The session passes useful traffic with installed MTU at most 1492. | Default pppd behavior remains interoperable within the transport cap. |
| AC-3 | Cap 1492; peer requests MRU 1400, 1492 or 1500 | Installed MTU is 1400, 1492 or at most 1492 respectively after valid negotiation; valid-sized traffic succeeds. | Smaller peer limits remain binding; a larger request can converge within the local cap. |
| AC-4 | LCP restarts or configuration reloads during established service | Replacement negotiation uses the admitted session policy; new sessions use the new policy. Required receive capacity and auth behavior remain intact. | Existing sessions retain their admitted policy, as the reload contract requires. |

## 🧪 Draft Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestOmittedMRUTransmitCap` | Shared PPP driver with observed backend MTU | Both transport inputs reach the real post-NCP setter; uncapped code installs 1500 and fails. |
| `TestTransmitCapRestartIsolation` | Driver restart/reload and event consumers | Replacement negotiation cannot bypass the stored cap. |
| Omitted-MRU live cases | L2TP and PPPoE policy scenarios; name exact cases during design | Observe omitted MRU on wire, installed `pppN` MTU, successful data traffic and oversize handling on Ze's kernel. |

Run scoped PPP/L2TP/PPPoE tests under race and prove live assertions fail against a rebuilt defective artifact. Existing interop results are historical; no tests ran for this split.

## Files to Modify and Documentation

- `internal/component/l2tp/ppp/session_run.go`: shared interface transmit-cap enforcement; census sibling transmit/MTU paths before editing.
- Shared PPP driver tests and existing L2TP/PPPoE policy scenarios: omitted-option and smaller-limit regression proof.
- `docs/guide/l2tp.md` and `docs/guide/pppoe.md`: explain effective transmit cap separately from negotiated peer receive MRU; update corresponding wiki pages and affected source anchors with implementation.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | Replacing negotiation state with configured cap misreports what the peer agreed. | Unequal configured/peer values expose copied output. | Keep negotiated state separate; output proof is owned by child 16f. |
| R-2 | A sibling writer or restart restores 1500. | Writer census or restart regression finds another setter. | Extend the shared path and prove every reached setter uses it. |
| A-1 | The shared interface setter reaches both transports. | Trace each transport's admitted `StartSession` and run both live cases. | Unvalidated; an alternate path must join the same enforcement before closure. |

## Research Remaining Before Ready

Complete the full design template, writer/consumer census, exact test and scenario names, RFC mapping, and integration/documentation checklists. Resolve low-MRU behavior with child 16d; its protocol-floor validation is separately owned. No implementation or closure is claimed here.
