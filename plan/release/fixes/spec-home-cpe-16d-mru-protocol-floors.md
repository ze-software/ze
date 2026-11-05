# Spec: Validate PPP MRU against enabled network protocol floors

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, config, docs |
| Priority | P1 |
| Release reason | Accepted MRU settings conflict with enabled IPv4 or IPv6 service and can exceed the peer's agreed limit after installation. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/1 |
| Handoff | verify |
| Updated | 2026-11-05 |

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
- `docs/research/l2tpv2-ze-integration.md`, IPv6 subscriber admission and service-size guard.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | A universal 1280 floor removes valid IPv4-only service. | AC-3 becomes rejected. | Make floor policy match effective NCP enablement. |
| R-2 | Validation occurs before explicit NCP settings are resolved. | IPv6-off input differs between entry points. | Validate the effective combination, with default/on/off tests. |
| A-1 | Schema supports the required cross-leaf/default constraint. | Read existing YANG conditional-validation patterns and exercise them. | Unvalidated; use the established validator mechanism where native constraints are insufficient. |

## Research Remaining Before Ready

Confirm exact RFC floor semantics, disabled-both-NCP behavior, peer negotiation policy and every config/provider producer. Complete full template, named wiring/live cases and integration/documentation checklists. No new RFC summary or full conformance claim is authorized by this skeleton.

## Implementation Handoff (2026-11-04)

The owner authorized implementation with compilation and execution centralized
in the root session. This agent authored the changes below and ran no code,
compiler, formatter, linter, test, daemon, Docker command or commit.

| Surface | Decision / authored change |
|---------|----------------------------|
| Config entry points | Native MRU range is 68..1500; registered original-tree validation, runtime extraction, provider reload and direct subsystem startup require at least 1280 when IPv6CP is enabled, including its default. |
| Shared driver admission | Validate the resolved session cap before creating a session; reject and release transferred descriptors for unsupported limits. Both-disabled NCP configuration still requires the 68-octet interface floor. |
| Negotiated peer limit | Normal IPv4 peer MRUs smaller than 68 are Nakked to 68. Proxy local MRU must meet the configured service floor. A smaller peer receive MRU can keep IPv4 usable, but IPv6CP, address allocation and address publication are withheld. IPv6-only service fails explicitly. |
| Restart | The IPv6 size refusal does not mutate admitted `disableIPv6CP`, so a later LCP lifetime can regain IPv6 at supported MRUs. |
| Authoritative basis | RFC 791 Section 3.2 requires forwarding 68 octets without further fragmentation. RFC 5072 Section 2 requires PPP IPv6 Information fields to meet the IPv6 link minimum; RFC 8200 Section 5 states 1280. RFC 1661 Section 6.1 keeps receive capacity at 1500 even after a smaller request. |

| Authored test | Outcome to verify centrally |
|---------------|-----------------------------|
| `TestMRUProtocolFloors` | Matrix 64/67/68/1279/1280/1492/1500/1501 through original-tree loading, raw-tree extraction and provider extraction with IPv6 default/on/off and both NCPs off. |
| `TestMRUFloorReloadIsolation` | Invalid IPv6 MRU preserves active policy; an explicit IPv4-only minimum remains accepted. |
| `TestSubsystemStartRejectsMRUFloor` | Direct Parameters cannot bypass floor validation. |
| `TestDriverRejectsUnsupportedNetworkMRU` | Direct StartSession cannot bypass limits or leak transferred files. |
| `TestPeerMRUBelowIPv4FloorIsNaked` | Values 64/67 Nak to 68, while a minimum supported IPv4 peer is Acked. |
| `TestPeerNakCannotLowerLocalMRUBelowIPv4Floor` | Correlated peer Naks suggesting 64/65/67 preserve the local MRU1492; a supported suggestion of 68 produces a replacement request at 68. |
| `TestIPv6MRUServiceGate` | Small directional MRUs cannot allocate or publish IPv6; unusable IPv6-only sessions fail. |
| `TestSmallPeerMRURetainsIPv4` | A 128-octet peer completes IPCP and installs MTU128; subsequent IPv6CP remains refused. |
| `TestIPv6MinimumMRUCompletesNCP` | A 1280-octet peer completes IPv6CP, publishes its negotiated IPv6 identifier and installs MTU1280. |
| Draft `lns-mru-protocol-floors.ci` | Operator config validation rejects incompatible sizes and accepts explicit IPv4-only/IPv6 minimum controls. |

Protected expectation migrations are proposed separately for independent
judgment; existing tests and the live configuration test remain unchanged
until that verdict. The old policy assertion at MRU64 is replaced with MRU68
and explicit IPv6-off service, retaining authentication and reload assertions;
new tests preserve 64/67 as rejected inputs. Root must format and compile the
new files, apply approved migrations, run scoped race/functional tests, prove
live IPv4/IPv6 behavior and synchronize the corresponding wiki text.

### Central Test Finding and Correction

The root's central PPP race run found that the small-peer session completed
IPCP and installed MTU128, but its post-NCP IPv6CP probe received no reply.
`handleFrame` called the refusal path, while `supportsProtocol` still classified
IPv6 as supported using only the configured enable flag. The refusal helper
therefore silently discarded the packet. The capability verdict now also
checks the negotiated size guard for both IPv6CP and IPv6 data. The regression
delivers the post-NCP probe through a unit-descriptor pipe, matching Linux's
packet route after channel connection, and still requires Protocol-Reject on
the channel. The author did not execute the correction; root rerun is owed.

An independent review then found a second minimum-size producer: accepted
proxy LCP admitted a nonzero peer MRU smaller than 68. The root ran the new
driver-level regression before the fix and observed invalid shortcut LCP-up
events for peer MRUs 1, 64 and 67; the MRU68 positive control passed. Proxy
admission now sends these undersized limits through fresh negotiation while
preserving omitted, 1500 and 9000 peer values. The interface setup also refuses
an effective transmit MRU smaller than 68 before authentication/NCP side effects
and installs the exact bounded size rather than raising it to an IP floor.
The tests retain permitted PAP, successful IPCP, MTU68 and correlated Echo
after a corrected peer request; root must rerun them after the source fix.

## Central Verification (2026-11-05)

This result supersedes the author's unrun handoff above. The coordinator
observed proxy peer MRUs 1, 64 and 67 incorrectly publish LCP-Up before the
lower-bound fix; the MRU68 positive control passed. The same driver tests
now pass with fresh negotiation, successful permitted PAP/IPCP, exact MTU68
and Echo. The independent sink refusal and the LCP/NCP unsent-Ack handler
regressions also pass. Independent review confirmed the source fixes.

The configuration floor carrier and migrated auth-policy carrier each passed
80 of 80 invocations under load, then were promoted. Their registered live
runner passed both tests. The final race run passed all L2TP packages,
including admission, provider, extraction, startup/reload and negotiated
IPv6-service coverage. The runtime is committed in `18ea5e830a`, and every
committed build flavor compiles. PPP statement coverage measured 82.6%; the
new `ValidateNetworkMRU` helper's five statements were all covered.

Evidence: `job-cpe16-handler-and-floor-red-7e7e4fa1.log`,
`job-cpe16-handler-and-floor-fixed-d12ee1a2.log`,
`job-cpe16-promoted-config-proofs-8456fabc.log` and
`job-cpe16-postreview-runtime-race-f3e7163f.log` under the coordinator's
`tmp/session/2026-11-04-222204/scratch/`. Stress captures under
`tmp/stress-repro/` identify each of the 80 successful invocations; the
stress harness's exit1 means no failure reproduced, not a failed carrier.
The matching wiki update is `312639d`. This spec stays open for closure.
