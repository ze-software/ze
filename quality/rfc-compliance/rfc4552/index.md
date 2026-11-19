# RFC 4552 - Authentication/Confidentiality for OSPFv3

OSPFv3 IPsec authentication. Every requirement listed for RFC 4552, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 27 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 10 | of 27 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 27 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 27 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 6 | 6 of 23 tests linked to requirements, 0 without a fault test and 4 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 6 | of 27 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 27 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 27 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 6 | of 27 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 27 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 27 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 27 | of 38 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 27 mandatory requirements: they add up to 27. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPFv3 IPsec authentication |
| Automatic checks | Included |
| Requirements | 38 |
| Mandatory requirements (MUST level) | 27 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 23 |
| Tests linked to requirements | 23 |
| Recorded audit verdicts | 6 |
| Fault test records | 10 |
| Summary | `rfc/short/rfc4552.md` |
| Requirement table | `rfc/requirements/rfc4552.md` |
| RFC text | `rfc/full/rfc4552.txt` |

## Automatic checks

Included in automatic checks: Authentication/Confidentiality for OSPFv3 (RFC 4552): OSPFv3 IPsec SA/policy installer over kernel XFRM. 4 MET (authentication support, confidentiality via ESP not AH, stream-cipher prohibition, hexadecimal key configuration) + 11 single-polarity positive (transport-mode SA + policies, per-interface SPD selection, source/dest/proto/direction selectors, inbound interface tagging, manual keying, shared inbound/outbound SA, IPsec barrier around all OSPF, protect + bypass SPD rules) + 5 gap (virtual-link IPsec: distinct SA, source/destination LA-bit selection, transit-area SPD rules) + 6 lower-layer (per-packet AH/ESP discard, ESP encapsulation and multi-SA storage performed by Linux XFRM on the state ze installs) + 1 gap (the §6 clause that imports RFC 4305 algorithm keyword levels, with no test of ze's algorithm enums against them)

## Published support status

**Area:** OSPFv3 IPsec authentication

**Test verification:** 4 of 27 MUST proven

**Evidence:** unit 4

**Unmet:** RFC4552-11-4, RFC4552-6-7, RFC4552-9-1, RFC4552-9-2, RFC4552-9-3, RFC4552-9-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 22 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **27** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC4552-3-1`](#rfc4552-3-1), [`RFC4552-4-2`](#rfc4552-4-2), [`RFC4552-6-6`](#rfc4552-6-6), [`RFC4552-7-1`](#rfc4552-7-1), [`RFC4552-12-1`](#rfc4552-12-1)

**Annotated (including scoped evidence) (22):** [`RFC4552-2-2`](#rfc4552-2-2), [`RFC4552-3-2`](#rfc4552-3-2), [`RFC4552-3-4`](#rfc4552-3-4), [`RFC4552-3-5`](#rfc4552-3-5), [`RFC4552-4-3`](#rfc4552-4-3), [`RFC4552-4-4`](#rfc4552-4-4), [`RFC4552-6-1`](#rfc4552-6-1), [`RFC4552-6-2`](#rfc4552-6-2), [`RFC4552-6-3`](#rfc4552-6-3), [`RFC4552-6-4`](#rfc4552-6-4), [`RFC4552-6-5`](#rfc4552-6-5), [`RFC4552-6-7`](#rfc4552-6-7), [`RFC4552-6-9`](#rfc4552-6-9), [`RFC4552-6-11`](#rfc4552-6-11), [`RFC4552-9-1`](#rfc4552-9-1), [`RFC4552-9-2`](#rfc4552-9-2), [`RFC4552-9-3`](#rfc4552-9-3), [`RFC4552-9-4`](#rfc4552-9-4), [`RFC4552-11-1`](#rfc4552-11-1), [`RFC4552-11-2`](#rfc4552-11-2), [`RFC4552-11-3`](#rfc4552-11-3), [`RFC4552-11-4`](#rfc4552-11-4)

## Figures

- **MUST bar**: Ze verified 4, Tests awaiting verification 11, OS testing gap 6, Not built 6
- **Overview bar**: Ze MUST verified 4, Tests awaiting verification 11, OS testing gap 6, Not assessed 11, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 27 |
| MUST rows kept (the MUST bar) | 27 |
| Ze verified | 4 |
| OS verified | 0 |
| Tests awaiting verification | 11 |
| OS testing gap | 6 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 11 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4552.json](../../../data/rfc/rfc4552.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4552-2-2` | All implementations conforming to this specification MUST support transport mode SA to provide required IPsec security to OSPFv3 packets (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L249). **negative:** no negative test. **{single-polarity}:** the installer only ever builds a transport-mode SA (buildIPsecSA Mode=ModeTransport, ipsec_install.go), so there is no tunnel-mode reject path to exercise |
| `RFC4552-3-1` | Implementations conforming to this specification MUST support authentication for OSPFv3 (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L49). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L105) |
| `RFC4552-3-2` | In order to provide authentication to OSPFv3, implementations MUST support ESP (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L138). **negative:** no negative test. **{single-polarity}:** an esp interface always installs an SA with Proto=ProtoESP (ipsecProtoNumber, ipsec_install.go); "supporting ESP" has no reject path of its own |
| `RFC4552-3-4` | o OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM policy check (net/xfrm/xfrm_policy.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex, with the AH or ESP template the interface names), and that installed policy is the state the kernel's check reads: Linux XFRM discards an OSPFv3 packet that arrives on that interface without the required AH or ESP transform. Owed tag: a test that reads that inbound policy back. Ze only samples the resulting counters (readXfrmDropsPlatform, ipsec_drops_linux.go) |
| `RFC4552-3-5` | o OSPFv3 packets that fail the authentication checks MUST be silently discarded. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM AH and ESP input (net/ipv6/ah6.c, net/ipv6/esp6.c), linux:CONFIG_INET6_AH with CONFIG_INET6_ESP; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the integrity transform and its truncation length, and Linux XFRM discards a packet whose ICV does not verify. Ze holds no verifying branch of its own and reads only the XfrmInIntegFailures/XfrmInStateProtoError counters |
| `RFC4552-4-2` | If confidentiality is provided, ESP MUST be used (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L200). **negative:** `unit/verify` [`TestIPsecAHWithEncryptionRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L88) |
| `RFC4552-4-3` | o OSPFv3 packets that are not protected with ESP MUST be silently discarded. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM policy check (net/xfrm/xfrm_policy.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex) with an ESP template on a confidentiality-enabled interface, and that installed policy is the state the kernel's check reads: Linux XFRM discards an OSPFv3 packet that arrives there without the required ESP transform. Owed tag: a test that reads that inbound policy and its ESP template back |
| `RFC4552-4-4` | o OSPFv3 packets that fail the confidentiality checks MUST be silently discarded. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM ESP (net/ipv6/esp6.c), linux:CONFIG_INET6_ESP; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP encryption transform beside the integrity one, and Linux XFRM discards a packet whose decryption or integrity check fails |
| `RFC4552-6-1` | IPsec in transport mode MUST be supported (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L280). **negative:** no negative test. **{single-polarity}:** every installed policy is transport mode (buildIPsecPolicies Mode=ModeTransport, ipsec_install.go); there is no non-transport policy to reject |
| `RFC4552-6-2` | The implementation MUST support multiple SPDs with an SPD selection function that provides an ability to choose a specific SPD based on interface. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L282). **negative:** no negative test. **{single-polarity}:** each interface's policies are scoped by IfIndex (buildIPsecPolicies IfIndex, ipsec_install.go), so the per-interface policy set is the interface-selected SPD; there is no reject path |
| `RFC4552-6-3` | The implementation MUST be able to use source address, destination address, protocol, and direction as selectors in the SPD (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L285). **negative:** no negative test. **{single-polarity}:** every policy carries source, destination, upper-protocol 89, and direction selectors (buildIPsecPolicies, ipsec_install.go); a selector set is emitted, never rejected |
| `RFC4552-6-4` | The implementation MUST be able to tag the inbound packets with the ID of the interface (physical or virtual) via which it arrived. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L288). **negative:** no negative test. **{single-polarity}:** the inbound require-policy is scoped to the arrival ifindex (buildIPsecPolicies SADirIn IfIndex, ipsec_install.go); the kernel does the per-packet tagging, ze only supplies the interface selector |
| `RFC4552-6-5` | Manually configured keys MUST be able to secure the specified traffic (§6) | MUST | 6 | **positive:** `unit/verify` [`TestSAParamsSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L338). **negative:** no negative test. **{single-polarity}:** the SA is keyed from the statically configured SPI+key with no IKE (buildIPsecSA, ipsec_install.go); manual keying has no reject path |
| `RFC4552-6-6` | The implementation MUST NOT allow the user to choose stream ciphers as the encryption algorithm for securing OSPFv3 packets (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L202). **negative:** `unit/verify` [`TestIPsecStreamCipherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L221) |
| `RFC4552-6-7` | Except when in conflict with the above statement, the key words "MUST", "MUST NOT", "REQUIRED", "SHOULD", and "SHOULD NOT" that appear in the [N6] document for algorithms to be supported are to be interpreted as described in [N7] for OSPFv3 support as well. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an interpretation clause importing the RFC 4305 algorithm levels for OSPFv3, with no role the gate can name; ze's OSPFv3 IPsec algorithm enums (internal/plugins/ospf/config.go, ipsecAuthKeyLen and ipsecEncKeyLen) carry the algorithms, and no test checks them against the levels RFC 4305 assigns |
| `RFC4552-6-9` | IP encapsulation of ESP packets MUST be supported (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM ESP (net/ipv6/esp6.c), linux:CONFIG_INET6_ESP; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode ESP SA and asks for no UDP encapsulation, so xfrmStateFromParams writes no Encap, and Linux XFRM carries every ESP packet directly in IP |
| `RFC4552-6-11` | Different SAs for different Differentiated Services Code Points (DSCPs) As per [N3], the IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM SAD (net/xfrm/xfrm_state.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes each state with its own SPI, and Linux XFRM keys the SAD by destination, SPI and protocol, so several SAs carrying one selector set coexist between a given sender and receiver |
| `RFC4552-7-1` | Therefore, the implementations MUST use manually configured keys with the same SA parameters (Security Parameter Index (SPI), keys, etc.) for both inbound and outbound SAs (as shown in Figure 3). (§7) | MUST | 7 | **positive:** `unit/verify` [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L214). **positive:** `unit/verify` [`TestRFC4552EveryStateSharesSPIAndKeys`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_same_keys_test.go#L38). **negative:** `unit/verify` [`TestRFC4552EveryStateSharesSPIAndKeys`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_same_keys_test.go#L41) |
| `RFC4552-9-1` | A different SA than the SA of the underlying interface MUST be provided for virtual links (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv3 virtual-link IPsec is unimplemented; the installer consumes only configured-interface IPsec blocks (setConfig over cfg.Interfaces, ipsec_install.go) and no virtual link ever installs an SA. Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-2` | Hence, routers that implement this specification MUST change the way the source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never enables IPsec on a virtual link, so it does not switch virtual-link source/destination selection when IPsec is on; the endpoints stay RFC 5340 §2.9 routed globals (v6ResolveVirtualEndpointLocked, virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-3` | The first IPv6 address with the "LA-bit" set in the list of prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** virtual-link IPsec is unimplemented, so the RFC 4552 §9 rule of using the first LA-bit intra-area-prefix address as the source is not applied; ze selects the source from RFC 5340 routed globals (v6RouterGlobalAddr, virtuallink_v6.go:42-81). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-4` | The first IPv6 address with the "LA-bit" set in the list of prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the matching destination rule (first LA-bit intra-area-prefix address of the virtual neighbor) is likewise unapplied because virtual-link IPsec is unimplemented (virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-11-1` | The IPsec protection barrier MUST be around the OSPF protocol. (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L291). **positive:** `unit/verify` [`TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_barrier_test.go#L27). **negative:** no negative test. **{single-polarity}:** the out/in/fwd proto-89 policies put the IPsec barrier around all OSPF traffic on the interface (buildIPsecPolicies, ipsec_install.go); the barrier is installed, never rejected |
| `RFC4552-11-2` | The SPD selection function MUST return an SPD with the following rule for all the interfaces that have OSPFv3 authentication/confidentiality disabled. No. source destination protocol action 1 any any OSPF bypass (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecDisabledInterfaceBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L370). **negative:** no negative test. **{single-polarity}:** a disabled interface installs no require-policy (setConfig adds only interfaces with an IPsec block, ipsec_install.go), so its OSPF is bypassed by the kernel default; the bypass has no reject path |
| `RFC4552-11-3` | The SPD selection function MUST return an SPD with the following rules for all the interfaces that have OSPFv3 authentication/confidentiality enabled. No. source destination protocol action 2 fe80::/10 any OSPF protect 3 fe80::/10 any ESP/OSPF or AH/OSPF protect (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L148). **positive:** `unit/verify` [`TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_barrier_test.go#L30). **negative:** no negative test. **{single-polarity}:** an enabled interface installs the out/in/fwd protect (require) policies (installLocked, ipsec_install.go); the protect rules are installed, never rejected. Ze's source selector is ::/0 scoped to the interface and to protocol 89, wider than fe80::/10, so every OSPF packet on the interface is protected, not only link-local-sourced ones. The virtual-link rules 4 and 5 of the same table are the gap row RFC4552-11-4 |
| `RFC4552-11-4` | These rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no virtual-link SPD rules are installed on transit-area interfaces because virtual-link IPsec is unimplemented (ipsec_install.go installs only configured-interface policies). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-12-1` | The implementations MUST allow the administrator to configure the cryptographic and authentication keys in hexadecimal format (§12) | MUST | 12 | **positive:** `unit/verify` [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L43). **negative:** `unit/verify` [`TestIPsecNonHexKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L235) |
| `RFC4552-4-1` | Implementations conforming to this specification SHOULD support confidentiality for OSPFv3 (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-6-8` | The routing module SHOULD be able to configure, modify, and delete IPsec rules on the fly. This is needed mainly for securing virtual links. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-6-10` | For simplicity, UDP encapsulation of ESP packets SHOULD NOT be used (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-8-1` | The user SHOULD be given the choice of sharing the same SA among multiple interfaces or using a unique SA per interface (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10-1` | To maintain the security of a link, the authentication and encryption key values SHOULD be changed periodically (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10.1-1` | The following three-step procedure SHOULD be provided to rekey the routers on a link without dropping OSPFv3 protocol packets or disrupting the adjacency. (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10.3-1` | recommends that the encryption and authentication keys SHOULD be changed at least every 90 days (§10.3) | SHOULD | 10.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-2-1` | Two hosts MAY establish a tunnel mode SA between themselves (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-2-3` | They MAY also support tunnel mode SA to provide required IPsec security to OSPFv3 packets. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-3-3` | In order to provide authentication to OSPFv3, implementations MUST support ESP and MAY support AH. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-11-5` | These rules MAY alternatively be installed on all the interfaces. (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4552-3-4`](#rfc4552-3-4) o OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded. (§3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM policy check (net/xfrm/xfrm_policy.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex, with the AH or ESP template the interface names), and that installed policy is the state the kernel's check reads: Linux XFRM discards an OSPFv3 packet that arrives on that interface without the required AH or ESP transform. Owed tag: a test that reads that inbound policy back. Ze only samples the resulting counters (readXfrmDropsPlatform, ipsec_drops_linux.go) |
| [`RFC4552-3-5`](#rfc4552-3-5) o OSPFv3 packets that fail the authentication checks MUST be silently discarded. (§3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM AH and ESP input (net/ipv6/ah6.c, net/ipv6/esp6.c), linux:CONFIG_INET6_AH with CONFIG_INET6_ESP; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the integrity transform and its truncation length, and Linux XFRM discards a packet whose ICV does not verify. Ze holds no verifying branch of its own and reads only the XfrmInIntegFailures/XfrmInStateProtoError counters |
| [`RFC4552-4-3`](#rfc4552-4-3) o OSPFv3 packets that are not protected with ESP MUST be silently discarded. (§4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM policy check (net/xfrm/xfrm_policy.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex) with an ESP template on a confidentiality-enabled interface, and that installed policy is the state the kernel's check reads: Linux XFRM discards an OSPFv3 packet that arrives there without the required ESP transform. Owed tag: a test that reads that inbound policy and its ESP template back |
| [`RFC4552-4-4`](#rfc4552-4-4) o OSPFv3 packets that fail the confidentiality checks MUST be silently discarded. (§4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM ESP (net/ipv6/esp6.c), linux:CONFIG_INET6_ESP; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs the ESP encryption transform beside the integrity one, and Linux XFRM discards a packet whose decryption or integrity check fails |
| [`RFC4552-6-7`](#rfc4552-6-7) Except when in conflict with the above statement, the key words "MUST", "MUST NOT", "REQUIRED", "SHOULD", and "SHOULD NOT" that appear in the [N6] document for algorithms to be supported are to be interpreted as described in [N7] for OSPFv3 support as well. (§6) | {gap}, no test | an interpretation clause importing the RFC 4305 algorithm levels for OSPFv3, with no role the gate can name; ze's OSPFv3 IPsec algorithm enums (internal/plugins/ospf/config.go, ipsecAuthKeyLen and ipsecEncKeyLen) carry the algorithms, and no test checks them against the levels RFC 4305 assigns |
| [`RFC4552-6-9`](#rfc4552-6-9) IP encapsulation of ESP packets MUST be supported (§6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM ESP (net/ipv6/esp6.c), linux:CONFIG_INET6_ESP; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the transport-mode ESP SA and asks for no UDP encapsulation, so xfrmStateFromParams writes no Encap, and Linux XFRM carries every ESP packet directly in IP |
| [`RFC4552-6-11`](#rfc4552-6-11) Different SAs for different Differentiated Services Code Points (DSCPs) As per [N3], the IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver. (§6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM SAD (net/xfrm/xfrm_state.c, the XFRM core linux:CONFIG_XFRM_USER selects); internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams writes each state with its own SPI, and Linux XFRM keys the SAD by destination, SPI and protocol, so several SAs carrying one selector set coexist between a given sender and receiver |
| [`RFC4552-9-1`](#rfc4552-9-1) A different SA than the SA of the underlying interface MUST be provided for virtual links (§9) | {gap}, no test | OSPFv3 virtual-link IPsec is unimplemented; the installer consumes only configured-interface IPsec blocks (setConfig over cfg.Interfaces, ipsec_install.go) and no virtual link ever installs an SA. Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-2`](#rfc4552-9-2) Hence, routers that implement this specification MUST change the way the source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled. (§9) | {gap}, no test | ze never enables IPsec on a virtual link, so it does not switch virtual-link source/destination selection when IPsec is on; the endpoints stay RFC 5340 §2.9 routed globals (v6ResolveVirtualEndpointLocked, virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-3`](#rfc4552-9-3) The first IPv6 address with the "LA-bit" set in the list of prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link. (§9) | {gap}, no test | virtual-link IPsec is unimplemented, so the RFC 4552 §9 rule of using the first LA-bit intra-area-prefix address as the source is not applied; ze selects the source from RFC 5340 routed globals (v6RouterGlobalAddr, virtuallink_v6.go:42-81). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-4`](#rfc4552-9-4) The first IPv6 address with the "LA-bit" set in the list of prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link. (§9) | {gap}, no test | the matching destination rule (first LA-bit intra-area-prefix address of the virtual neighbor) is likewise unapplied because virtual-link IPsec is unimplemented (virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-11-4`](#rfc4552-11-4) These rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link. (§11) | {gap}, no test | no virtual-link SPD rules are installed on transit-area interfaces because virtual-link IPsec is unimplemented (ipsec_install.go installs only configured-interface policies). Disclosed in docs/features/rfc-status.md RFC 4552 row |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4552-2-2`](#rfc4552-2-2)

All implementations conforming to this specification MUST support transport mode SA to provide required IPsec security to OSPFv3 packets (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L249) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC4552-3-1`](#rfc4552-3-1)

Implementations conforming to this specification MUST support authentication for OSPFv3 (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L105) | unit/verify | not verified |
| positive | [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L49) | unit/verify | not verified |

### [`RFC4552-3-2`](#rfc4552-3-2)

In order to provide authentication to OSPFv3, implementations MUST support ESP (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L138) | unit/verify | mutant, verified |

### [`RFC4552-3-4`](#rfc4552-3-4)

o OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-3-4, so there is no linked test.

### [`RFC4552-3-5`](#rfc4552-3-5)

o OSPFv3 packets that fail the authentication checks MUST be silently discarded. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-3-5, so there is no linked test.

### [`RFC4552-4-2`](#rfc4552-4-2)

If confidentiality is provided, ESP MUST be used (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPsecAHWithEncryptionRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L88) | unit/verify | not verified |
| positive | [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L200) | unit/verify | not verified |

### [`RFC4552-4-3`](#rfc4552-4-3)

o OSPFv3 packets that are not protected with ESP MUST be silently discarded. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-4-3, so there is no linked test.

### [`RFC4552-4-4`](#rfc4552-4-4)

o OSPFv3 packets that fail the confidentiality checks MUST be silently discarded. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-4-4, so there is no linked test.

### [`RFC4552-6-1`](#rfc4552-6-1)

IPsec in transport mode MUST be supported (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L280) | unit/verify | not verified |

### [`RFC4552-6-2`](#rfc4552-6-2)

The implementation MUST support multiple SPDs with an SPD selection function that provides an ability to choose a specific SPD based on interface. (§6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecPoliciesInterfaceScopedWildcard moved. Forbidden: one SPD applied regardless of interface. Ze realises the per-interface SPD as IfIndex-scoped XFRM policies; TestIPsecPoliciesInterfaceScopedWildcard asserts every installed policy has IfIndex == testIfIndex, red if a policy were global (IfIndex 0), and TestIPsecDisabledInterfaceBypass asserts a disabled interface on the same installer gets no policy. Single-polarity marker: no reject path.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L282) | unit/verify | not verified |

### [`RFC4552-6-3`](#rfc4552-6-3)

The implementation MUST be able to use source address, destination address, protocol, and direction as selectors in the SPD (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L285) | unit/verify | not verified |

### [`RFC4552-6-4`](#rfc4552-6-4)

The implementation MUST be able to tag the inbound packets with the ID of the interface (physical or virtual) via which it arrived. (§6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecPoliciesInterfaceScopedWildcard moved. Forbidden: inbound OSPF handled without regard to its arrival interface. The kernel tags; Ze owns the interface selector it installs. TestIPsecPoliciesInterfaceScopedWildcard asserts the policy set contains the SADirIn direction and that every policy, the inbound one included, carries IfIndex == testIfIndex, red if the inbound policy were unscoped. Single-polarity marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L288) | unit/verify | not verified |

### [`RFC4552-6-5`](#rfc4552-6-5)

Manually configured keys MUST be able to secure the specified traffic (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestSAParamsSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L338) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC4552-6-6`](#rfc4552-6-6)

The implementation MUST NOT allow the user to choose stream ciphers as the encryption algorithm for securing OSPFv3 packets (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPsecStreamCipherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L221) | unit/verify | not verified |
| positive | [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L202) | unit/verify | not verified |

### [`RFC4552-6-7`](#rfc4552-6-7)

Except when in conflict with the above statement, the key words "MUST", "MUST NOT", "REQUIRED", "SHOULD", and "SHOULD NOT" that appear in the [N6] document for algorithms to be supported are to be interpreted as described in [N7] for OSPFv3 support as well. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-6-7, so there is no linked test.

### [`RFC4552-6-9`](#rfc4552-6-9)

IP encapsulation of ESP packets MUST be supported (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-6-9, so there is no linked test.

### [`RFC4552-6-11`](#rfc4552-6-11)

Different SAs for different Differentiated Services Code Points (DSCPs) As per [N3], the IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-6-11, so there is no linked test.

### [`RFC4552-7-1`](#rfc4552-7-1)

Therefore, the implementations MUST use manually configured keys with the same SA parameters (Security Parameter Index (SPI), keys, etc.) for both inbound and outbound SAs (as shown in Figure 3). (§7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecSAPerDestinationWithOSPFSelector moved. Re-judged 2026-09-29 (independent judge). TestRFC4552EveryStateSharesSPIAndKeys builds the ESP interface's whole SA set (inbound unicast plus the two bidirectional multicast states) and asserts every state carries the one configured SPI and byte-identical integrity and encryption keys; the second half configures another SPI and other keys and finds those on every state, which refuses keys not taken from manual configuration. Records on buildIPsecInterfaceSAs and buildIPsecSA. The stale {single-polarity} annotation was removed from the row by the judge (./le rfc check demanded it). Not driven: the per-neighbour outbound unicast state (onNeighborSeen), which calls the same buildIPsecSA with the same spec.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4552EveryStateSharesSPIAndKeys`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_same_keys_test.go#L41) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestIPsecSAPerDestinationWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC4552EveryStateSharesSPIAndKeys`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_same_keys_test.go#L38) | unit/verify | revert, verified |

### [`RFC4552-9-1`](#rfc4552-9-1)

A different SA than the SA of the underlying interface MUST be provided for virtual links (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-9-1, so there is no linked test.

### [`RFC4552-9-2`](#rfc4552-9-2)

Hence, routers that implement this specification MUST change the way the source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-9-2, so there is no linked test.

### [`RFC4552-9-3`](#rfc4552-9-3)

The first IPv6 address with the "LA-bit" set in the list of prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-9-3, so there is no linked test.

### [`RFC4552-9-4`](#rfc4552-9-4)

The first IPv6 address with the "LA-bit" set in the list of prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-9-4, so there is no linked test.

### [`RFC4552-11-1`](#rfc4552-11-1)

The IPsec protection barrier MUST be around the OSPF protocol. (§11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecPoliciesInterfaceScopedWildcard moved. Re-judged 2026-09-30 (continuation 6, judge): single-polarity positive holds. TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection asserts out/in/fwd proto-89 protect policies with the ESP or AH transform on both esp and ah interfaces; the revert record on buildIPsecPolicies was re-observed red after the test gained selector assertions.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L291) | unit/verify | revert, verified |
| positive | [`TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_barrier_test.go#L27) | unit/verify | revert, verified |

### [`RFC4552-11-2`](#rfc4552-11-2)

The SPD selection function MUST return an SPD with the following rule for all the interfaces that have OSPFv3 authentication/confidentiality disabled. No. source destination protocol action 1 any any OSPF bypass (§11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecDisabledInterfaceBypass moved. Forbidden: a protect policy or SA on an interface with OSPFv3 security disabled. TestIPsecDisabledInterfaceBypass configures an interface with no ipsec block, brings it up, and asserts zero SAs and zero policies, so OSPF falls to the kernel default (bypass); TestIPsecPoliciesInterfaceScopedWildcard asserts an enabled interface's policies are IfIndex-scoped, so they cannot cover the disabled one. Single-polarity marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecDisabledInterfaceBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L370) | unit/verify | not verified |

### [`RFC4552-11-3`](#rfc4552-11-3)

The SPD selection function MUST return an SPD with the following rules for all the interfaces that have OSPFv3 authentication/confidentiality enabled. No. source destination protocol action 2 fe80::/10 any OSPF protect 3 fe80::/10 any ESP/OSPF or AH/OSPF protect (§11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/ospf/ipsec_install_test.go::TestIPsecInstallOnInterfaceUp moved. Re-judged 2026-09-30 (continuation 6, judge): the weak finding (source selector unasserted) is closed. TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection now asserts, for every out/in/fwd policy on esp and ah interfaces, that the source selector covers fe80::/10 (selectorCovers: prefix length <= 10 containing fe80::; Ze installs ::/0, wider, as the row annotation states) and the destination is any (::/0), plus upper protocol 89 and the protect transform. Records: installLocked mutant and buildIPsecPolicies revert, observed red. Single-polarity positive per annotation (the rules are installed, never refused). Virtual-link rules 4 and 5 stay gap row RFC4552-11-4.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L148) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4552_barrier_test.go#L30) | unit/verify | revert, verified |

### [`RFC4552-11-4`](#rfc4552-11-4)

These rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link. (§11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4552-11-4, so there is no linked test.

### [`RFC4552-12-1`](#rfc4552-12-1)

The implementations MUST allow the administrator to configure the cryptographic and authentication keys in hexadecimal format (§12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPsecNonHexKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L235) | unit/verify | not verified |
| positive | [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L43) | unit/verify | not verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc4552.txt |
| Source fingerprint | 13a1f16d1a8257fb |
| Record | rfc/extraction/rfc4552.json |
| Mapped sentences | 28 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 3 | reviewed | not stated |
| `3` | not stated | 4 | reviewed | not stated |
| `4` | not stated | 3 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 9 | reviewed | not stated |
| `7` | not stated | 4 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 6 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 2 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |
| `10.3` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 4 | reviewed | not stated |
| `12` | not stated | 1 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `15` | not stated | 0 | reviewed | not stated |
| `15.1` | not stated | 0 | reviewed | not stated |
| `15.2` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive of the IPsec architecture, not of an OSPFv3 implementation: the lowercase 'must' restates when the IPsec architecture calls for a tunnel mode SA. The same paragraph then removes the case from OSPFv3: 'since the packets are locally delivered, the routers assume the role of hosts in the context of tunnel mode SA.' | The SA must be a tunnel mode SA if either end of the security association is a router/gateway. |
| `6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The lead-in sentence that introduces the capability list in section 6. It carries no obligation of its own; the obligations are the numbered capability entries that follow, sites 6:2 through 6:9. | In order to implement this specification, the following IPsec capabilities are required. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the shape of the problem on a broadcast interface. 'required' names the security service OSPFv3 needs, and the sentence states no obligation on an implementation. | While running OSPFv3 over a broadcast interface, the authentication/confidentiality required is "one to many". |
| `7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes IKE, another system: it explains why a Diffie-Hellman two-party protocol cannot serve the one-to-many case. It places no obligation on an OSPFv3 implementation. | Since IKE is based on the Diffie-Hellman key agreement protocol and works only for two communicating parties, it is not possible to use IKE for providing the required "one to many" authentication/confidentiality. |
| `7:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A forward reference to the scalability discussion that follows. It is the rationale for the obligation in site 7:4, not an obligation itself. | The following discussion explains that it is not scalable and is practically infeasible to use different security associations for inbound and outbound traffic to provide the required "one to many" security. |
| `9:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The rationale sentence for the address-selection obligations that follow. 'the source address also needs to be predictable' explains why sites 9:4, 9:5 and 9:6 exist; the obligations are those three sentences. | In order to install the required security rules for virtual links, the source address also needs to be predictable. |
| `10.1:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | A step-ordering note inside the rekey procedure, which section 10.1 opens as 'The following three-step procedure SHOULD be provided to rekey the routers on a link without dropping OSPFv3 protocol packets or disrupting the adjacency.' The enclosing construction is a SHOULD, recorded as RFC4552-10.1-1. | Note that all routers on the link must complete step 1 before any begin step 2. |
| `10.1:2` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The second half of the same step-ordering note, inside the same construction: 'The following three-step procedure SHOULD be provided to rekey the routers on a link without dropping OSPFv3 protocol packets or disrupting the adjacency.' Recorded as RFC4552-10.1-1. | Likewise, all the routers on the link must complete step 2 before any begin step 3. |
| `15.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF intellectual property boilerplate in the back matter. It addresses any interested party about patent disclosure and carries no protocol obligation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4552, so its obligations are stated where they were written.
