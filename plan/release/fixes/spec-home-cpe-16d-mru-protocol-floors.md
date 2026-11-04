# Spec: Validate PPP MRU against enabled network protocol floors

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, config, docs |
| Priority | P1 |
| Release reason | Accepted MRU settings conflict with enabled IPv4 or IPv6 service and can exceed the peer's agreed limit after installation. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

## Task

Own finding F-4 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction and review. Make accepted MRU settings compatible with enabled IPv4/IPv6 service. Cover default-enabled IPv6CP and explicitly disabled IPv6CP. Never repair an impossible configuration by silently raising the MTU beyond the peer's receive limit.

Child 16a owns shared transmit-cap enforcement. This spec owns protocol-floor validation and its schema/runtime/negotiation boundaries; coordinate their shared files without duplicating cap logic.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`.
  → Constraint: validate through existing YANG/config/session mechanisms, not a second config path.
- [ ] `docs/guide/l2tp.md`, configuration, NCP and reload; `ai/patterns/config-option.md` and `ai/rules/config.md`.
  → Constraint: keep schema and runtime extraction consistent, including default-enabled NCPs.
- [ ] RFC 1661 Section 6.1, RFC 1332, RFC 791 minimum datagram size and RFC 5072 Section 2 in authoritative repository texts.
  → Constraint: confirm direction and exact floor obligations before selecting schema ranges or conditional validation.

## Current Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/config_policy.go`, `parseMRU` | Accepts integers from 64 through 1500, independent of enabled NCPs. |
| `internal/component/l2tp/config.go`, `ExtractParameters` | Initializes `EnableIPCP` and `EnableIPv6CP` true, then applies explicit NCP values. |
| `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `mru` / `ncp/enable-ipv6cp` | MRU range is 64..1500; IPv6CP defaults true. |
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.afterLCPOpen` | Installs at least 68 even when negotiated peer MRU is smaller. |

Preserve valid IPv4-only configurations, useful IPv6 at supported sizes, explicit NCP choices, fail-closed authentication and admission-time reload isolation.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Native leaf validation | `ze-l2tp-conf.yang`, MRU/NCP leaves | Defines operator-facing types, ranges and defaults. | Extend native constraints as far as they express the selected conditional policy. |
| Config extraction | `parseMRU`, `ExtractParameters` | Resolves MRU and NCP enablement. | Apply consistent protocol-aware validation after effective defaults are known. |
| Negotiation and interface setup | Shared PPP driver | Learns directional peer MRU and installs interface MTU. | Refuse or renegotiate unsupported service sizes without silently increasing a peer limit. |

## Data Flow

File/editor/provider input → schema validation → effective MRU and NCP defaults → startup/reload validation → admitted session policy → LCP/NCP negotiation → interface installation. Census every extraction and provider path, including direct driver callers and negotiated peer minima, before design approval.

## Draft Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | IPv4 service enabled; configured MRU 64..67 | Reject the incompatible configuration before starting a session; error names value, floor and executable repair. | Supported IPv4-only sizes remain permitted because IPv6 is not enabled. |
| AC-2 | IPv6CP absent/default or explicitly true; configured MRU 1279 | Reject the incompatible configuration consistently across entry points. | MRU 1280, 1492 and 1500 remain permitted for useful IPv6 service. |
| AC-3 | IPv6CP false and IPv4 enabled; MRU 68 or 1279 | Accept valid configuration and establish working IPv4 service with installed MTU respecting both local and peer limits. | Sub-1280 IPv4-only service remains supported because no IPv6 service is promised. |
| AC-4 | Valid IPv6 configuration at 1280; incompatible smaller negotiated peer receive limit | Do not install an MTU larger than the peer can receive or announce usable IPv6 over an unsupported link; apply the RFC-backed negotiation/service policy selected during research. | A conforming peer establishes working IPv6 at 1280; a peer's larger receive capacity remains allowed. |
| AC-5 | Rejected reload changes MRU/NCP combination | Active config and established sessions retain the previous valid policy. | An accepted later reload applies its valid policy to new sessions only. |

## 🧪 Draft Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestMRUProtocolFloors` | Schema and runtime config extraction | Matrix 64, 67, 68, 1279, 1280, 1492, 1500 with IPv6 default/on/off; test out-of-range input too. |
| `TestMRUFloorReloadIsolation` | Candidate validation and reload | Invalid candidate cannot mutate current config or session policy. |
| Driver and live NCP cases | Exact functional/interop carriers selected during design | Useful IPv4-only and IPv6 traffic at supported floors; incompatible peer size cannot be hidden by the 68 installation floor. |

Independent `ze-test-judge` approval is required before changing protected boundary expectations. Run scoped L2TP/PPP tests under race and prove functional/interop assertions discriminate the defective artifact. No tests ran for this split.

## Files to Modify and Documentation

- `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `config_policy.go`, `config.go` and provider extraction consumers found by the census: consistent effective MRU/NCP validation.
- Shared PPP negotiation/installation code only where necessary to handle a negotiated incompatible peer limit; share child 16a's cap.
- Config boundary/reload tests and applicable driver/functional/interop cases.
- `docs/guide/l2tp.md`, MRU range, NCP defaults, validation errors and reload behavior; corresponding wiki page and affected source anchors updated with implementation.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | A universal 1280 floor removes valid IPv4-only service. | AC-3 becomes rejected. | Make floor policy match effective NCP enablement. |
| R-2 | Validation occurs before explicit NCP settings are resolved. | IPv6-off input differs between entry points. | Validate the effective combination, with default/on/off tests. |
| A-1 | Schema supports the required cross-leaf/default constraint. | Read existing YANG conditional-validation patterns and exercise them. | Unvalidated; use the established validator mechanism where native constraints are insufficient. |

## Research Remaining Before Ready

Confirm exact RFC floor semantics, disabled-both-NCP behavior, peer negotiation policy and every config/provider producer. Complete full template, named wiring/live cases and integration/documentation checklists. No new RFC summary or full conformance claim is authorized by this skeleton.
