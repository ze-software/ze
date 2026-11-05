# Spec: Validate PPP MRU against enabled network protocol floors

| Field | Value |
|-------|-------|
| Status | done |
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

- [x] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`.
  → Constraint: validate through existing YANG/config/session mechanisms, not a second config path.
- [x] `docs/guide/l2tp.md`, configuration, NCP and reload; `ai/patterns/config-option.md` and `ai/rules/config.md`.
  → Constraint: keep schema and runtime extraction consistent, including default-enabled NCPs.
- [x] RFC 1661 Section 6.1, RFC 1332 Section 2.1, RFC 791 Section 3.2, RFC 5072 Section 2 and RFC 8200 Section 5 in authoritative repository texts.
  → Constraint: confirm direction and exact floor obligations before selecting schema ranges or conditional validation.
- [x] `plan/post-release/features/spec-l2tp-ipv6-subscriber.md`, owner ruling of 2026-08-30.
  → Constraint: the first release's subscriber service is IPv4-only. The 1280 positive proof here covers the PPP driver and NCP boundary, not shipped IPv6 traffic.

## Current Behavior

This table preserves the defective baseline reviewed before `18ea5e830a`.
The implementation audit below describes the corrected behavior.

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

## Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | IPv4 service enabled; configured MRU 64..67 | Reject the incompatible configuration before starting a session; error names value, floor and executable repair. | Supported IPv4-only sizes remain permitted because IPv6 is not enabled. |
| AC-2 | IPv6CP absent/default or explicitly true; configured MRU 1279 | Reject the incompatible configuration consistently across entry points. | MRU 1280, 1492 and 1500 remain permitted for useful IPv6 service. |
| AC-3 | IPv6CP false and IPv4 enabled; MRU 68 or 1279 | Accept valid configuration and establish working IPv4 service with installed MTU respecting both local and peer limits. | Sub-1280 IPv4-only service remains supported because no IPv6 service is promised. |
| AC-4 | Valid IPv6 configuration at 1280; incompatible smaller negotiated peer receive limit | Withhold IPv6 negotiation, allocation and publication below 1280; keep independently useful IPv4, or fail an IPv6-only driver session explicitly. | The driver positive control completes IPv6CP and publishes its negotiated identifier at 1280 with a test address provider. Shipped IPv6 subscriber traffic belongs to the existing post-release IPv6 spec under the owner's IPv4-only ruling. |
| AC-5 | Rejected reload changes MRU/NCP combination | Active config and established sessions retain the previous valid policy. | An accepted later reload applies its valid policy to new sessions only. |

## Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestMRUProtocolFloors` | Schema and runtime config extraction | Matrix 64, 67, 68, 1279, 1280, 1492, 1500 with IPv6 default/on/off; test out-of-range input too. |
| `TestMRUFloorReloadIsolation` | Candidate validation and reload | Invalid candidate cannot mutate current config or session policy. |
| Driver NCP and native IPv4 cases | `TestProxyMRUIPv4FloorRemainsUsable`, `TestSmallPeerMRURetainsIPv4`, `TestIPv6MinimumMRUCompletesNCP`; native `ppp-code-reject-xl2tpd` and cap carriers | Corrected 68-octet driver service completes permitted authentication, IPCP and Echo; a 128-octet driver keeps IPv4 and refuses IPv6CP. The IPv6 driver completes NCP at 1280. The real pppd carrier at peer MRU128 retains IPv4 ping; native cap carriers retain measured MTU1492 and traffic. Full subscriber IPv6 traffic is the owner's post-release feature, not a result claimed here. |

The independent boundary judgments are retained in `18ea5e830a` as one
`Weakening-approved:` trailer per changed protected test. The coordinator's
scoped race, functional and native interop results below supersede the
original unrun author handoff. The later proxy-decoder correction adds a Driver
regression without altering existing assertions or RFC tags.

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
| A-1 | Broken as a schema-only assumption: the scalar range alone does not enforce the effective NCP combination. | `ze-l2tp-conf.yang` declares 68..1500; `RegisterTreeValidator` wires `validateMRUPolicy` into original-tree loading. | The existing registered validator supplies the conditional check; extraction, startup, reload and direct driver admission repeat it at their boundaries. |

## Research Resolved

RFC 791 Section 3.2 states: "Every internet module must be able to forward a
datagram of 68 octets without further fragmentation." RFC 5072 Section 2
states: "PPP links supporting IPv6 MUST allow the information field to be at
least as large as the minimum link MTU size required for IPv6 [2]." RFC 8200
Section 5 gives the IPv6 link minimum as 1280 octets. RFC 1661 Section 6.1
retains full 1500-octet receive capacity after a smaller request.

Both-disabled NCP input still needs the 68-octet interface floor. An undersized
proxy peer renegotiates; the interface setter never raises its agreed size.
The owner already assigned full IPv6 subscriber traffic to
`plan/post-release/features/spec-l2tp-ipv6-subscriber.md`. No new RFC summary,
feature promotion or full IPv6 service claim is made here.

## Implementation Handoff (2026-11-04)

This historical author handoff is retained for provenance. Central Verification
below records the subsequent observed runs and supersedes its unrun obligations.

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
the channel. The author did not execute the correction; the later central rerun
below passed.

An independent review then found a second minimum-size producer: accepted
proxy LCP admitted a nonzero peer MRU smaller than 68. The root ran the new
driver-level regression before the fix and observed invalid shortcut LCP-up
events for peer MRUs 1, 64 and 67; the MRU68 positive control passed. Proxy
admission now sends these undersized limits through fresh negotiation while
preserving omitted, 1500 and 9000 peer values. The interface setup also refuses
an effective transmit MRU smaller than 68 before authentication/NCP side effects
and installs the exact bounded size rather than raising it to an IP floor.
The tests retain permitted PAP, successful IPCP, MTU68 and correlated Echo
after a corrected peer request; the later central rerun below passed.

## Central Verification (2026-11-05)

This result supersedes the author's unrun handoff above. The coordinator
observed proxy peer MRUs 1, 64 and 67 incorrectly publish LCP-Up before the
lower-bound fix; the MRU68 positive control passed. The same driver tests
now pass with fresh negotiation, successful permitted PAP/IPCP, exact MTU68
and Echo. The independent sink refusal and the LCP/NCP unsent-Ack handler
regressions also pass. Independent review confirmed the source fixes.

Closure then found that explicit zero and malformed final proxy MRU widths could
acquire the absent option's default. The coordinator observed seven failing bad
cases and five passing controls before the decoder correction. Afterward all
12 cases of `TestProxyMRUOptionEncodingDriver` pass under tagged race, and the
complete L2TP package tree passes under race. Bad proxy inputs must now negotiate
fresh LCP before any interface installation, then complete permitted PAP, IPCP
and correlated Echo with a corrected 68-octet peer.

The rebuilt native `l2tp-proxy-mru-transmit-cap` carrier also passes. Peer1500,
peer9000 and an omitted peer MRU each complete CHAP/IPCP, install measured
kernel MTU1492, carry a1492-octet Information-field ICMP round trip and remove
the interface after StopCCN. The fixture argument0 means omission; it does not
prove the explicit-zero rejection covered by the separate Driver regression.

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
Additional decoder evidence is
`job-cpe16d-proxy-mru-encoding-red-fbf3616c.log`,
`job-cpe16d-proxy-mru-encoding-green-fbf3616c.log`,
`job-cpe16d-proxy-mru-runtime-race-e9d0980c.log` and
`job-cpe16d-proxy-native-after-decode-fix-5bad2c84.log` in the same scratch
directory. The matching wiki update is `312639d`. Final native proof freshness,
committed compilation and the machine review checks are recorded below.

The changed `run` producer also required a fresh RFC1661-5.8-2 negative record.
The coordinator's native `job-cpe16d-keepalive-semantic-observed-52117517.log`
shows the unchanged named test green, then a sticky-Opened semantic mutant red
at its actual wire assertion: EchoRequest `0xc021/9` arrived where the test
requires a ConfigureRequest `0xc021/1` after renegotiation. Native recording
merged that observation into the current producer's discrimination record.
No panic or failed setup is credited as this negative proof.

The final duplicate-MRU matrix first failed all eight cases against the
first-match decoder, then all20 cases across both additive matrices passed
under race. The final PPP package race passed, and the rebuilt native proxy
carrier again passed omission,1500 and9000 with measured1492 traffic and cleanup.
These results are in `job-cpe16d-proxy-duplicate-red-ce86d0e7.log`,
`job-cpe16d-proxy-mru-all-cases-green-3392f282.log`,
`job-cpe16d-proxy-final-ppp-race-fdd309de.log` and
`job-cpe16d-proxy-final-native-5bad2c84.log`. The runtime, new tests and native
keepalive record landed together in `7e2fb4953f`. Native
`job-cpe16d-runtime-committed-build-9f891c3d.log` compiles that committed tree
successfully in all five flavors: distro, appliance, setup, host and installer.

## Executive Summary

The scalar range and effective IPv6CP combination are enforced before session
admission. Normal peers smaller than 68 are Nakked; supported smaller peers
retain IPv4 without gaining unusable IPv6 service. The independent review
found a proxy-decoding ambiguity: explicit zero and some malformed MRU widths
were treated like omission. The corrected decoder resolves only absence to1500,
requires a two-octet value and preserves zero for policy rejection. All12 new
real-driver cases, every reached L2TP package under race and the rebuilt native
proxy carrier pass. The final malformed-option census found that a first valid
MRU could hide a duplicate MRU in the first-match decoder. The corrected helper
scans each final stream and rejects every second MRU, including an identical
one. Round4 independently clears that source delta and eight additive Driver
cases. All20 additive cases, final PPP race and rebuilt native proxy interop
pass. All five committed build flavors pass, and the native review check
confirms the complete reviewed code population is CLEAN with matching hashes.

Risks R-1 and R-2 are resolved by conditional validation after defaults and by
positive IPv4 controls. Full subscriber IPv6 remains the owner's post-release
extension. The native 1280 result here is a driver/NCP proof with a test address
provider. Wider RFC, tooling and published-site checks retain unrelated reds.

## Deliverables Checklist

| Deliverable | Verification | Result |
|-------------|--------------|--------|
| Native scalar range and operator help | Read `ze-l2tp-conf.yang` MRU type, default, help and description. | Done: 68..1500, default1500; IPv6 minimum named. |
| Effective conditional floor at every config entry | Read registered validator, both extractors, startup and reload; inspect `TestMRUProtocolFloors`. | Done: default/on IPv6 requires1280; explicit-off and both-off permit68..1279. |
| Direct driver admission and descriptor ownership | Read `Driver.spawnSession`, `StartSession.closeFiles` and admission regression. | Done: invalid limits close transferred files, emit rejection and create no session. |
| Small-peer negotiation without an upward clamp | Read normal and proxy admission, strict proxy decoder, MTU sink and tiny-proxy tests. | Done:1/64/67 and explicit0 renegotiate; malformed final MRU widths renegotiate; corrected68, omission and9000 controls pass. |
| Negotiated IPv6 size safety and useful IPv4 | Read `ipv6MRUSupported`, NCP request/publication/service consumers and driver tests. | Done: the shared directional predicate gates IPv6; invalid proxy values cannot acquire the omitted option's1500 default. |
| Reload and protected assertions retained | Read reload isolation tests and `18ea5e830a` approval trailers. | Done: invalid candidate leaves live policy; earlier admitted sessions retain their policy. |
| Functional and native IPv4 evidence | Read promoted carrier and central named results; native pppd MRU128 and cap carriers. | Done for existing paths; no full IPv6 traffic claimed. |
| Documentation and local RFC rationale | Compare guides, design and feature paragraph with producers. | Feature description corrected; nine-file comment correction reviewed independently. Native final checks remain the coordinator's. |

## Security Review Checklist

| Concern | Source check | Result |
|---------|--------------|--------|
| Invalid config or direct-session input bypasses floor | `parseMRU`, `validateMRUPolicy`, `ExtractParameters`, `extractFromProvider`, `Subsystem.Start`, `Driver.spawnSession`. | Invalid resolved caps are refused; missing values resolve before validation. |
| Invalid proxy peer is mistaken for an absent value | `decodeProxyMRU`, `EvaluateProxyLCP`, `acceptProxyPolicy`, `pppSession.run`. | Only absence receives1500; explicit peer0 fails the floor. The final decoder scans every option and rejects a second MRU, including an identical one. Stored and published peer MRU agree. |
| Small peer gains IPv6 allocation or publication | `runNCPPhase`, `onNCPOpened`, `afterLCPOpenIPv6Service`, `supportsProtocol`. | Same directional predicate gates allocation, state and service; independently useful IPv4 remains available. |
| Rejected admission leaks transferred files | `spawnSession` rejection occurs before registration; `closeFiles` closes both ownership fields. | Driver regression checks rejection, invalidated descriptors and empty session map. |
| Peer causes a panic or unbounded allocation | Floor comparisons use bounded numeric fields; no new peer-driven loop, allocation or panic. | The new registration panic is an init-time duplicate-registration failure, unreachable from a peer. |
| Config errors expose secrets or inject commands | Numeric limits use fixed formatting and fixed repair commands. | No credential is included and no command is executed from an MRU value. |
| Refused reload corrupts active policy | Extraction completes before `s.params` and reactor defaults change. | Invalid1279/default-IPv6 candidate leaves1492/dual-NCP policy intact; valid68/IPv6-off applies to future sessions. |

## Documentation Update Checklist

| # | Category | Update needed | Source-aware disposition |
|---|----------|---------------|--------------------------|
| 1 | Feature declaration | Yes | `features/l2tpv2-bng.md` Description now states68..1500 and default-enabled IPv6's1280 floor. It names actual IPv4 interop and the driver-only IPv6 boundary. Scope remains partial and Level experimental; broader review dates are not re-attested. |
| 2 | User guide | Yes, already landed | `docs/guide/l2tp.md`, MRU/NCP and reload paragraphs, in `18ea5e830a`; `validateMRUPolicy`, both extractors and `spawnSession` match them. `docs/guide/pppoe.md` correctly retains its fixed1492 cap and full1500 receive capacity. Wiki equivalents landed in `312639d`. |
| 3 | Config syntax | No additional page | No grammar changed. The existing MRU scalar range and NCP boolean meanings are documented in the L2TP guide. `docs/guide/configuration.md` anchors authentication timing, which this fix does not change. |
| 4 | CLI reference | No | No command is added or renamed. Existing `handleConfig` still reports effective policy; only accepted MRU combinations change. |
| 5 | API/RPC | No | No request, response, event type or wire method changes. Driver admission rejects an unsupported existing field value. |
| 6 | Plugin guide | No | The shared address handler registration and process boundary are unchanged. The static pool's IPv6 decline is explicit in the feature paragraph. |
| 7 | Wire format | No new format | MRU remains a two-octet Information-field size in the existing LCP option. The change constrains supported values and preserves the actual agreed value. |
| 8 | SDK rules | No | No SDK or registration contract changes. |
| 9 | RFC compliance | Yes, rationale repaired | Nine source files carry exact RFC791/5072 quotes or caller references in `426b892b43`. Its comment-sensitive RFC2661 audit was independently rejudged and native-stamped. The changed run producer's RFC1661-5.8-2 negative claim was natively re-recorded from an observed sticky-Opened semantic mutant failing the exact wire assertion. Final native check has76 foreign findings, adding only an unrelated RFC2545 unit-changed record to the75 baseline; no owned floor/keepalive finding appears. No generated RFC page is hand-edited. |
| 10 | Test infrastructure | No new mechanism | `test/l2tp/lns-mru-protocol-floors.ci` uses the existing config-validation carrier. Native labs use their existing fixtures and checker registrations; no new runtime dependency. |
| 11 | Comparison | No | This fix neither promotes the declared feature level nor introduces full subscriber IPv6. |
| 12 | Architecture | Yes, already landed | `docs/research/l2tpv2-ze-integration.md`, schema and IPv6-size paragraphs, already describes the registered validator and directional service predicate. |
| 13 | Route metadata | No | No route fields or propagation rules change. A refused IPv6 size emits no assigned-address event. |
| 14 | Metrics | No | Existing WARN and session events expose refusal. No metric name or label changes. |
| 15 | Agent discovery | No | No command, feature family, package boundary or native tool is added. |
| 16 | Related source-anchored designs | Assessed | `docs/architecture/l2tp/subscriber-session-model.md` keeps acknowledged publication and lifetime withdrawal. `docs/architecture/l2tp/bng-1-radius-attributes.md` keeps teardown causes/accounting. `docs/architecture/l2tp/bng-5-pppoe.md` adds no conflicting MRU policy. `docs/research/l2tpv2-implementation-guide.md` Section18 describes raw proxy option AVPs and optional renegotiation; strict final MRU decoding preserves that design. The full IPv6 feature remains in its existing post-release spec. |
| 17 | Manual reference | No change needed | `docs/config-reference.md` Subscriber Authentication anchors the changed YANG file's existing `auth-method` leaf. Its CHAP-MD5 default and explicit no-auth opt-in examples match that leaf and are unaffected by the MRU floor. The page makes no MRU range, default or IPv6 minimum claim; the owned MRU documentation is in the L2TP guide. |

## Implementation Summary

### What Was Implemented

The existing schema, registered original-tree validator and effective runtime
extractors enforce68..1500 and1280 for enabled IPv6CP. Startup and direct driver
admission cannot bypass that policy. Normal unsupported peer limits renegotiate;
the interface sink refuses rather than raising them. The negotiated directional
predicate withholds IPv6 while retaining independently useful IPv4.

### Bugs Found/Fixed

The earlier implementation fixed post-NCP IPv6CP refusal classification and
proxy peer1/64/67 admission. Closure found missing local RFC rationale and an
additional explicit-zero/malformed-width proxy decoding bypass. The comment
correction landed in `426b892b43`; all five committed build flavors passed.
The runtime decoder correction and additive12-case Driver regression are
independently reviewed CLEAN. The coordinator observed seven failing bad cases
before the fix and all12 passing under race afterward; final native checks remain
with the coordinator.

### Documentation Updates

The L2TP/PPPoE guides in `18ea5e830a`, wiki `312639d` and integration design
describe the current floors, receive capacity and admission/reload behavior.
Closure corrects the owned feature paragraph and explicitly separates driver
IPv6CP proof from the post-release subscriber IPv6 path. Existing feature anchors
for `handleTick`, `answerRefusedSCCRQ`, `stopCCNSlot`, `nasPortIDFormat` and
`bngRegister` were source-checked; no full BNG review is claimed.

The coordinator's latest documentation check reports142 published gh-pages
catalog/mode disagreements. Its YANG443, usage/help, source4206/772, rules34
and digest3037 stages pass. The discovery-index warning was repaired by native
`job-cpe16d-final-doc-index-update-f2aba6ca.log`, which regenerated365 design
documents and4206 source paths across772 packages. Targeted native
`job-cpe16d-final-doc-index-check-dc77857d.log` then passes all365 design docs
and4206 code paths across772 packages, with every reference valid.
The final site build stops in foreign
`externalSource.witness` work, at registry-to-`os/signal` import identity.
The post-comment RFC check is byte-identical to its75 foreign baseline after
the independently rejudged RFC2661 audit. The final post-runtime check adds only
one foreign RFC2545 unit-changed record, for76 findings; owned floor and keepalive
proofs have no freshness finding. `job-cpe16d-final-doc-check-baf5a445.log`
reports all source anchors valid, including the feature description.

### Deviations from Plan

The scalar YANG range is unconditional; registered tree validation supplies the
effective cross-leaf/default rule. The original draft's full IPv6 traffic wording
exceeded the owner's existing IPv4-only release ruling. Its corrected positive
control is1280 driver/NCP negotiation; full subscriber IPv6 remains with the
existing post-release feature, without a maturity promotion.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 expected the scalar schema to enforce the effective NCP combination. | The registered original-tree validator supplies that conditional check after defaults. | Read scalar range, `register.go` and `validateMRUPolicy`; inspect all entry-point matrix assertions. | Retain the established validator and repeat validation at direct runtime boundaries. |
| assumption | The draft promised live subscriber IPv6 at1280. | The owner assigned subscriber IPv6 to post-release; the shipped pool declines it. | Read the2026-08-30 ruling, `poolPlugin.handle` and the driver's test provider. | Label the positive control driver/NCP-only and name the existing IPv6 feature owner. |
| approach | Feature prose retained64..1500 and pending policy evidence. | Current floors and completed IPv4 native evidence differ. | Read the feature Description against schema, validators and maintained checkers. | Correct that paragraph and its anchors; keep broader maturity unchanged. |
| approach | New floor guards and wrappers omitted required local RFC quotes/references. | Each enforcing boundary must state its RFC basis. | Round1 source-style/RFC pass and completed floor-caller census. | Independent author added comment-only rationale in nine files; round2 reviewed only that delta. |
| approach | The tiny-proxy regression used0 to mean omission. | An explicit MRU option carrying0 reached the same decoder value and bypassed the floor; malformed widths could also look absent. | Read `EvaluateProxyLCP` and the driver helper's option construction; central Driver run reproduced seven failures. | Resolve absence explicitly, require two-octet final MRU values and reject every below-floor value. The additive12-case matrix passes under race; the final duplicate census then found the separate row below. |
| approach | The first decoder correction still used first-match lookup. | A valid first MRU hid illegal duplicates, even identical ones; RFC1661 Section6 permits no repeated MRU. | Final malformed-option census and eight observed real-Driver reds. | Scan every final option and reject the second MRU; round4 independently reviewed that helper and eight additive cases. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Compatible admitted MRU/NCP combinations | Done | `ValidateNetworkMRU`, registered validator, extractors, startup and driver | Default/on/off and both-off matrix checked. |
| Never raise an unsupported agreed peer limit | Done | `decodeProxyMRU`, `EvaluateProxyLCP`, `acceptProxyPolicy`, `afterLCPOpen` | Existing1/64/67 and explicit0 renegotiate. Invalid final widths cannot become absent; every duplicate final MRU triggers fresh negotiation. Both additive matrices pass. |
| Preserve valid service and reload isolation | Done | Config carrier, provider/reactor tests and NCP driver controls | IPv4 retains its smaller MTU;1280 positive proves only driver IPv6CP. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestMRUProtocolFloors`, promoted floor carrier | Original-tree, raw tree, provider and CLI refuse incompatible IPv4 values. |
| AC-2 | Done | Matrix default/on1279 rejection;1280/1492/1500 acceptance | Explicit NCP values resolve before conditional validation. |
| AC-3 | Done | CLI positive controls; corrected68 proxy driver;128-octet IPv4 driver/native carrier | Authentication/NCP, exact sink value and useful protocol/IPv4 controls are retained. |
| AC-4 | Done | `TestIPv6MRUServiceGate`, `TestSmallPeerMRURetainsIPv4`, `TestIPv6MinimumMRUCompletesNCP`, `TestProxyMRUOptionEncodingDriver` | Allocation/publication/service guards agree; explicit0 no longer becomes default1500. Full shipped IPv6 is not claimed. |
| AC-5 | Done | `TestMRUFloorReloadIsolation`, protected provider and reactor reload tests | Refused candidate retains old settings; valid candidate changes future admission only. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Config matrix and startup/reload | Done | `internal/component/l2tp/mru_protocol_floor_test.go` | Central race results pass; exact assertions read. |
| Driver admission, normal Nak and NCP limits | Done | `ppp/mru_protocol_floor_test.go` | Direct rejected caps release ownership; good IPv4/IPv6-engine controls retained. |
| Tiny proxy, strict final MRU decoding and sink refusal | Done | `ppp/proxy_mru_floor_driver_test.go`, `ppp/proxy_mru_encoding_driver_test.go` | Existing1/64/67 red/fixed evidence and68 positive verified. Initial12-case matrix reproduces seven bad cases; duplicate matrix reproduces all eight. All20 now pass under race, retaining the original controls. |
| Protected boundary migrations | Done | `config_policy_test.go`, `reactor_ppp_linux_test.go`, `auth_policy_driver_test.go` | Exact independent approvals remain in `18ea5e830a`; authentication and isolation assertions retained. |
| Promoted operator configuration carrier | Done | `test/l2tp/lns-mru-protocol-floors.ci` |80/80 under load; registered named carrier passes. |
| Existing policy functional companion | Done | `test/l2tp/lns-mru-auth-allow-config.ci` |80/80 under load; order, readback and malformed-policy assertions preserved. |
| Native IPv4 data/control carrier | Done | `ppp-code-reject-xl2tpd` and maintained cap scenarios | Real pppd MRU128 retains ping; cap cases retain measured1492 and cleanup. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| YANG, config parser and registration | Done | Scalar range/help, registered conditional validator and default resolution are wired. |
| Startup, provider reload and session admission | Done | Each validates before changing policy or creating a session. |
| Shared PPP negotiation and service predicate | Done | Strict proxy decoding separates absence, explicit zero and invalid widths; policy and the exact MTU sink enforce supported sizes. |
| Boundary, provider, reactor and driver tests | Done | Existing protected expectations were independently judged; additive12-case decoder proof retains valid controls and weakens no test. |
| Functional carriers | Done | Both promoted config proofs retain named registered execution. |
| Guides, design, wiki and feature declaration | Done | Current owned prose is source-checked and all4206 native source anchors resolve; derived public pages are never hand-edited. |

### Audit Summary

All21 grouped audit rows are Done and independently source-reviewed. No item is
skipped. Both additive matrices have observed red/green, final scoped/native
checks pass, owned proof records remain fresh and all five committed flavors
compile. The machine review check confirms CLEAN and matching code hashes.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refuse MRU settings incompatible with enabled protocols | Operator command and entry-point matrix | Promoted `lns-mru-protocol-floors` passes; its real `ze config validate` commands reject67 and default/on1279, accept IPv4-only68/1279 and IPv6-config1280. The same matrix covers original tree and provider extraction. |
| Retain supported smaller IPv4 service without raising peer limits | Driver plus native IPv4 | Corrected68 proxy driver completes PAP/IPCP/Echo with MTU68; the128-octet driver retains IPv4 and rejects IPv6CP. Native pppd MRU128 carries ping. Explicit-zero/malformed final proxy values now renegotiate before installation; omission and9000 controls retain the1492 cap. |
| Bound the IPv6 engine before allocation, publication or service | Driver NCP and state boundary | Unsupported directional limits fail IPv6-only setup with no IP request or assigned event;1280 completes IPv6CP and publishes its negotiated identifier. This uses a test provider and does not prove shipped subscriber IPv6 traffic. |
| Preserve live policy on rejected reload | Provider and reactor entry points | Refused1279/default-enabled candidate leaves1492 policy. Valid68/IPv6-off candidate applies to new admission while the earlier StartSession keeps1492 and its original authentication list. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Full shipped IPv6 subscriber traffic and prefix delegation. | Existing2026-08-30 owner ruling makes the first release IPv4-only. The1280 test proves the engine boundary only. | `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-16d-mru-protocol-floors-3a525723-d369-4547-9124-fe35463dc0f5.md`; native record in `job-cpe16d-formal-review-26caa91e.log`:46 reviewed paths, verdict CLEAN,4 rounds. |
| `./le spec review check` | PASS: native `review_gate: OK (26 code files, clean, hashes match)` in the same26caa91e log. Citation-anchor advisory assessed the sole additional page, `docs/config-reference.md`, in the Manual reference row above. |
| Rounds | 4. Round1: whole floor change, all entry points, protected migrations, tests and documents. Round2: nine-file RFC comment correction and reached references; CLEAN. Round3: strict proxy decoder/policy/session correction, additive12-case Driver regression and reached run-loop/ticker consumers; the final census found illegal duplicate MRUs still first-match accepted, withdrawing the earlier CLEAN. Round4: only the full-scan duplicate guard, eight additive Driver cases and reached decoder/error/normal-LCP/auth/NCP callers; CLEAN,0 BLOCKER and0 ISSUE. Both additive matrices, final scoped race and rebuilt native proxy carrier pass; owned proof records are fresh. |
| Reviewer lenses used | Logic/wiring/spec; security and negative boundaries; Go style, performance and documentation. All lenses ran inline. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | New floor enforcement/helper callers lacked local RFC quotes and section references. | Nine floor-source files, starting at `mru_policy.go` `ipv6MRUSupported` and `config_policy.go` wrappers | `426b892b43`: independent comment-only correction reviewed CLEAN in round2; comment-sensitive audit independently rejudged/native-stamped; baseline RFC findings unchanged and all five committed flavors compile. |
| 2 | BLOCKER | Explicit peer MRU0 was decoded as absent and could shortcut LCP with default1500. Some malformed MRU option lengths also became absent. | `proxy.go` `EvaluateProxyLCP` / `lookupMRUOption`, `auth.go` `acceptProxyPolicy`, `session_run.go` `run` | `7e2fb4953f`: strict `decodeProxyMRU` and unconditional below68 policy rejection; obsolete run-loop fallback removed. Additive real-driver matrix reproduces seven bad cases before the fix, then passes all12 cases under race. The duplicate boundary is resolved by finding3 below. |
| 3 | ISSUE | A valid first MRU hid an illegal duplicate MRU, including a second zero or malformed-width value. | `proxy.go` `decodeProxyMRU` / `lookupOption` | `7e2fb4953f`: full final-option scan rejects every second MRU with the exact RFC1661 Section6 quote above the guard. All eight new Driver cases failed before the fix, including identical valid duplicates. Both matrices now pass all20 cases under race. Round4 CLEAN. |

Documentation NOTEs were corrected in this pass: the feature description and
draft's overbroad IPv6 traffic claim. They earn no additional source-review round.

### Fixes applied

The missing RFC rationale belonged to protocol source comments: new helpers,
guards and wrappers omitted their local quotation or caller section reference.
The [workaround] of waiving that rule would leave the enforcing basis invisible.
The [source] correction added exact RFC quotations and caller references without
changing executable behavior, then independently refreshed the comment-sensitive
audit fingerprint.

The zero/width failure belonged to proxy decoding. `EvaluateProxyLCP` reused a
lookup contract that collapsed absent and malformed options, while `run` mapped
zero to1500. The [workaround] of rejecting all proxy LCP would discard valid
admission without resolving the input meaning. The [source] correction resolves
only absence to1500, validates the final field width, preserves zero for the floor
policy and stores/publishes that same decoded peer value.

The duplicate failure also belongs to `decodeProxyMRU`: first-match lookup hides
the rest of the final MRU options. The [workaround] of testing only one instance
leaves invalid input admitted. The selected [source] correction rejects a
second final MRU while retaining valid omission and converged-history controls.
All eight new Driver cases failed before the fix; both matrices now pass all20
cases under race with their valid admission and useful-service controls retained.

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| Schema and runtime source | Yes | Read current YANG, config/registration, startup/reload, driver and PPP guards named in the audit. |
| Boundary and protected tests | Yes | Read exact config, provider, reactor, driver and NCP test bodies, including their positive controls. |
| Operator and native carriers | Yes | Read promoted floor/auth-policy `.ci` files and `checkPPPCodeReject`/cap checkers. |
| Additive explicit-zero/malformed proxy test | Yes | Read all12 cases of `proxy_mru_encoding_driver_test.go` and actual Driver/pipe helper: invalid input must renegotiate before any MTU call, then complete useful service. |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Configured IPv4 range cannot be smaller than68. | `parseMRU`, native schema and current original-tree/provider/CLI matrix match; central named runs pass. |
| AC-2 | Default/on IPv6 requires1280. | Both extractors resolve defaults before `validateMRUNCP`; promoted CLI carrier and central matrix pass. |
| AC-3 | Explicit IPv6-off retains68/1279 and useful smaller IPv4. | Read the valid CLI rows, corrected68 driver authentication/IPCP/Echo,128-octet driver and native pppd ping proof. |
| AC-4 | Smaller negotiated directions cannot allocate/publish/start IPv6. | Same predicate gates every consumer;1280 driver NCP positive passes. Strict final proxy decoding prevents explicit0 or malformed widths acquiring1500. |
| AC-5 | A bad candidate cannot replace active session policy. | Read `Reload` extraction-before-mutation, rejection assertions and old/new reactor-dispatch snapshots; central race passes. |

### Wiring Verified

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Operator `ze config validate` | `test/l2tp/lns-mru-protocol-floors.ci` | Seven real command invocations with rejected and permitted outcomes; promoted named carrier passes. |
| Established policy and provider readback | `test/l2tp/lns-mru-auth-allow-config.ci` | Existing auth/order/refusal/readback obligations survive the independently approved68/IPv6-off migration. |
| Native subscriber traffic | `ppp-code-reject-xl2tpd` plus cap scenarios | Typed interop plans invoke real pppd and kernel PPP. This is IPv4 evidence, not a full IPv6 carrier. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Broken as schema-only; handled by the planned established validator | The scalar type has no conditional expression; `register.go` registers `validateMRUPolicy` for original-tree loading. Runtime extraction and direct entry points repeat the effective check. |
| R-1 | Resolved | Explicit IPv6-off68/1279 and both-off68 remain accepted; corrected68 driver reaches useful service. |
| R-2 | Resolved | Conditional validation follows effective NCP defaults in both extractors; matrix includes absent/on/off values. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide range, defaults and reload | Schema, `validateMRUPolicy`, `ExtractParameters`, `extractFromProvider`, `spawnSession` | Yes; current L2TP guide and wiki match. PPPoE fixed1492/full1500-receive claim remains correct. |
| Feature paragraph and IPv6 scope | `handleConfig`, maintained IPv4 checkers, test address provider, shipped `poolPlugin.handle`, existing owner ruling | Yes;68/1280 and current scoped proof replace the obsolete text, without promoting full BNG support. |
| Integration and sibling architecture | Directional predicate, NCP publication, reset and existing accounting causes | Yes; no new SDK/event/route/metric or wire-layout claim. Related pages are named in the checklist. |
| Manual reference | `ze-l2tp-conf.yang` auth-method/allow-no-auth leaves and `docs/config-reference.md` Subscriber Authentication | Yes; the existing default and opt-in examples remain accurate. The anchored page contains no MRU-floor claim to change. |
| Native documentation/proof checks | Final doc-check and RFC check; independently rejudged audit and native keepalive semantic record | All4206 source anchors resolve;142 site-only drifts remain. Discovery regeneration and targeted check pass for365 design docs/4206 paths/772 packages. Final RFC76 is the75 foreign baseline plus one unrelated RFC2545 unit-changed record; no owned floor/keepalive record is stale. |
