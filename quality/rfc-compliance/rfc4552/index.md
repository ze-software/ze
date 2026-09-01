# RFC 4552 - Authentication/Confidentiality for OSPFv3

Experimental. Every requirement this repository extracted from RFC 4552, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 20.0% | 4 of 20 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 55.0% | 11 of 20 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 20 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 19 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 27 | of 38 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 27 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 5 of 20 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 20 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 38 |
| Gated MUST-level | 27 |
| Obligations that bind Ze | 20 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 5 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4552.md` |
| Requirement shard | `rfc/requirements/rfc4552.md` |
| RFC text | `rfc/full/rfc4552.txt` |

## Enrolment

Enrolled: Authentication/Confidentiality for OSPFv3 (RFC 4552): OSPFv3 IPsec SA/policy installer over kernel XFRM. 4 MET (authentication support, confidentiality via ESP not AH, stream-cipher prohibition, hexadecimal key configuration) + 11 single-polarity positive (transport-mode SA + policies, per-interface SPD selection, source/dest/proto/direction selectors, inbound interface tagging, manual keying, shared inbound/outbound SA, IPsec barrier around all OSPF, protect + bypass SPD rules) + 5 gap (virtual-link IPsec: distinct SA, source/destination LA-bit selection, transit-area SPD rules) + 7 not-applicable (per-packet AH/ESP discard/encap and multi-SA storage delegated to kernel XFRM)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Manual AH/ESP IPsec config for OSPFv3 IPv6 interfaces, XFRM readiness check, SA and policy lifecycle.

**What the ledger says remains:**

Manual keying only; feature remains under OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 23 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **27** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC4552-3-1`](#rfc4552-3-1), [`RFC4552-4-2`](#rfc4552-4-2), [`RFC4552-6-6`](#rfc4552-6-6), [`RFC4552-12-1`](#rfc4552-12-1)

**Annotated instead of tested (23):** [`RFC4552-2-2`](#rfc4552-2-2), [`RFC4552-3-2`](#rfc4552-3-2), [`RFC4552-3-4`](#rfc4552-3-4), [`RFC4552-3-5`](#rfc4552-3-5), [`RFC4552-4-3`](#rfc4552-4-3), [`RFC4552-4-4`](#rfc4552-4-4), [`RFC4552-6-1`](#rfc4552-6-1), [`RFC4552-6-2`](#rfc4552-6-2), [`RFC4552-6-3`](#rfc4552-6-3), [`RFC4552-6-4`](#rfc4552-6-4), [`RFC4552-6-5`](#rfc4552-6-5), [`RFC4552-6-7`](#rfc4552-6-7), [`RFC4552-6-9`](#rfc4552-6-9), [`RFC4552-6-11`](#rfc4552-6-11), [`RFC4552-7-1`](#rfc4552-7-1), [`RFC4552-9-1`](#rfc4552-9-1), [`RFC4552-9-2`](#rfc4552-9-2), [`RFC4552-9-3`](#rfc4552-9-3), [`RFC4552-9-4`](#rfc4552-9-4), [`RFC4552-11-1`](#rfc4552-11-1), [`RFC4552-11-2`](#rfc4552-11-2), [`RFC4552-11-3`](#rfc4552-11-3), [`RFC4552-11-4`](#rfc4552-11-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4552-2-2` | All implementations conforming to this specification MUST support transport mode SA to provide required IPsec security to OSPFv3 packets (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L152). **negative:** no negative test. **{single-polarity}:** the installer only ever builds a transport-mode SA (buildIPsecSA Mode=ModeTransport, ipsec_install.go), so there is no tunnel-mode reject path to exercise |
| `RFC4552-3-1` | Implementations conforming to this specification MUST support authentication for OSPFv3 (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L48). **negative:** `unit/verify` [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L104) |
| `RFC4552-3-2` | In order to provide authentication to OSPFv3, implementations MUST support ESP (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L69). **negative:** no negative test. **{single-polarity}:** an esp interface always installs an SA with Proto=ProtoESP (ipsecProtoNumber, ipsec_install.go); "supporting ESP" has no reject path of its own |
| `RFC4552-3-4` | When OSPFv3 authentication is enabled, OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the per-packet silent discard of unprotected inbound OSPFv3 is done by the kernel XFRM inbound require-policy; ze installs that policy (buildIPsecPolicies SADirIn, ipsec_install.go) and only samples the drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32), never inspecting or dropping a packet |
| `RFC4552-3-5` | When OSPFv3 authentication is enabled, OSPFv3 packets that fail the authentication checks MUST be silently discarded (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the discard of an OSPFv3 packet that fails the integrity check is performed by the kernel XFRM transform; ze reads the resulting XfrmInIntegFailures/XfrmInStateProtoError counters (ipsec_drops_linux.go:24-30) but never verifies or drops a packet itself |
| `RFC4552-4-2` | If confidentiality is provided, ESP MUST be used (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L202). **negative:** `unit/verify` [`TestIPsecAHWithEncryptionRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L87) |
| `RFC4552-4-3` | When OSPFv3 confidentiality is enabled, OSPFv3 packets that are not protected with ESP MUST be silently discarded (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an unprotected packet on a confidentiality-enabled interface is dropped by the kernel XFRM inbound require-policy (buildIPsecPolicies SADirIn, ipsec_install.go), not by ze |
| `RFC4552-4-4` | When OSPFv3 confidentiality is enabled, OSPFv3 packets that fail the confidentiality checks MUST be silently discarded (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an ESP packet that fails decryption/integrity is dropped by the kernel XFRM transform; ze surfaces the drop via the XFRM counters (ipsec_drops_linux.go:32) but never decrypts or discards a packet |
| `RFC4552-6-1` | IPsec in transport mode MUST be supported (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L190). **negative:** no negative test. **{single-polarity}:** every installed policy is transport mode (buildIPsecPolicies Mode=ModeTransport, ipsec_install.go); there is no non-transport policy to reject |
| `RFC4552-6-2` | The implementation MUST support multiple SPDs with an SPD selection function that chooses a specific SPD based on interface (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L192). **negative:** no negative test. **{single-polarity}:** each interface's policies are scoped by IfIndex (buildIPsecPolicies IfIndex, ipsec_install.go), so the per-interface policy set is the interface-selected SPD; there is no reject path |
| `RFC4552-6-3` | The implementation MUST be able to use source address, destination address, protocol, and direction as selectors in the SPD (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L195). **negative:** no negative test. **{single-polarity}:** every policy carries source, destination, upper-protocol 89, and direction selectors (buildIPsecPolicies, ipsec_install.go); a selector set is emitted, never rejected |
| `RFC4552-6-4` | The implementation MUST be able to tag inbound packets with the ID of the (physical or virtual) interface via which they arrived (§6) | MUST | 6 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L198). **negative:** no negative test. **{single-polarity}:** the inbound require-policy is scoped to the arrival ifindex (buildIPsecPolicies SADirIn IfIndex, ipsec_install.go); the kernel does the per-packet tagging, ze only supplies the interface selector |
| `RFC4552-6-5` | Manually configured keys MUST be able to secure the specified traffic (§6) | MUST | 6 | **positive:** `unit/verify` [`TestSAParamsSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L248). **negative:** no negative test. **{single-polarity}:** the SA is keyed from the statically configured SPI+key with no IKE (buildIPsecSA, ipsec_install.go); manual keying has no reject path |
| `RFC4552-6-6` | The implementation MUST NOT allow the user to choose stream ciphers as the encryption algorithm for securing OSPFv3 packets (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L204). **negative:** `unit/verify` [`TestIPsecStreamCipherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L223) |
| `RFC4552-6-7` | The algorithm key words (MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT) that appear in RFC 4305 [N6] are to be interpreted per RFC 2119 for OSPFv3 support as well, except when in conflict with the stream-cipher prohibition (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an interpretation clause importing RFC 4305 [N6] algorithm keywords; RFC 4552 defines no independent behavior here. The concrete algorithm conformance lives in the config enums (ipsecAuthKeyLen/ipsecEncKeyLen, config.go:106-117) and the stream-cipher carve-out is RFC4552-6-6 |
| `RFC4552-6-9` | IP encapsulation of ESP packets MUST be supported (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the IP encapsulation of ESP packets is the per-packet ESP framing performed by the kernel XFRM transform (internal/component/ike/dataplane/xfrm_linux.go); ze installs the ESP SA but never builds an ESP header |
| `RFC4552-6-11` | The IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multiple SAs with the same selectors between two peers is a kernel SAD capability (states keyed by SPI); ze installs one SA per interface with a distinct SPI (buildIPsecSA, ipsec_install.go) and delegates SA storage to the kernel XFRM |
| `RFC4552-7-1` | The implementations MUST use manually configured keys with the same SA parameters (SPI, keys, etc.) for both inbound and outbound SAs (§7) | MUST | 7 | **positive:** `unit/verify` [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L139). **negative:** no negative test. **{single-polarity}:** one manually configured SPI/key drives the single shared SA that both protects egress and verifies ingress (buildIPsecSA installed once, ipsec_install.go); there is no reject path |
| `RFC4552-9-1` | A different SA than the SA of the underlying interface MUST be provided for virtual links (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv3 virtual-link IPsec is unimplemented; the installer consumes only configured-interface IPsec blocks (setConfig over cfg.Interfaces, ipsec_install.go) and no virtual link ever installs an SA. Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-2` | Routers that implement this specification MUST change the way source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never enables IPsec on a virtual link, so it does not switch virtual-link source/destination selection when IPsec is on; the endpoints stay RFC 5340 §2.9 routed globals (v6ResolveVirtualEndpointLocked, virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-3` | The first IPv6 address with the "LA-bit" set in prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** virtual-link IPsec is unimplemented, so the RFC 4552 §9 rule of using the first LA-bit intra-area-prefix address as the source is not applied; ze selects the source from RFC 5340 routed globals (v6RouterGlobalAddr, virtuallink_v6.go:42-81). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-9-4` | The first IPv6 address with the "LA-bit" set in prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the matching destination rule (first LA-bit intra-area-prefix address of the virtual neighbor) is likewise unapplied because virtual-link IPsec is unimplemented (virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-11-1` | The IPsec protection barrier MUST be around the OSPF protocol so that all inbound and outbound OSPF traffic goes through IPsec processing (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L201). **negative:** no negative test. **{single-polarity}:** the out/in/fwd proto-89 policies put the IPsec barrier around all OSPF traffic on the interface (buildIPsecPolicies, ipsec_install.go); the barrier is installed, never rejected |
| `RFC4552-11-2` | The SPD selection function MUST return an SPD with the bypass rule for all interfaces that have OSPFv3 authentication/confidentiality disabled (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecDisabledInterfaceBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L278). **negative:** no negative test. **{single-polarity}:** a disabled interface installs no require-policy (setConfig adds only interfaces with an IPsec block, ipsec_install.go), so its OSPF is bypassed by the kernel default; the bypass has no reject path |
| `RFC4552-11-3` | The SPD selection function MUST return an SPD with the protect rules for all interfaces that have OSPFv3 authentication/confidentiality enabled (§11) | MUST | 11 | **positive:** `unit/verify` [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L74). **negative:** no negative test. **{single-polarity}:** an enabled interface installs the out/in/fwd protect (require) policies (installLocked, ipsec_install.go); the protect rules are installed, never rejected |
| `RFC4552-11-4` | The virtual-link SPD rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no virtual-link SPD rules are installed on transit-area interfaces because virtual-link IPsec is unimplemented (ipsec_install.go installs only configured-interface policies). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| `RFC4552-12-1` | The implementations MUST allow the administrator to configure the cryptographic and authentication keys in hexadecimal format (§12) | MUST | 12 | **positive:** `unit/verify` [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L42). **negative:** `unit/verify` [`TestIPsecNonHexKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L237) |
| `RFC4552-4-1` | Implementations conforming to this specification SHOULD support confidentiality for OSPFv3 (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-6-8` | The routing module SHOULD be able to configure, modify, and delete IPsec rules on the fly (mainly for securing virtual links) (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-6-10` | For simplicity, UDP encapsulation of ESP packets SHOULD NOT be used (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-8-1` | The user SHOULD be given the choice of sharing the same SA among multiple interfaces or using a unique SA per interface (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10-1` | To maintain the security of a link, the authentication and encryption key values SHOULD be changed periodically (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10.1-1` | The three-step rekey procedure SHOULD be provided to rekey the routers on a link without dropping OSPFv3 protocol packets or disrupting the adjacency (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-10.3-1` | The encryption and authentication keys SHOULD be changed at least every 90 days (§10.3) | SHOULD | 10.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-2-1` | Two hosts MAY establish a tunnel mode SA between themselves (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-2-3` | Implementations MAY also support tunnel mode SA to provide required IPsec security to OSPFv3 packets (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-3-3` | In order to provide authentication to OSPFv3, implementations MAY support AH (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4552-11-5` | The virtual-link SPD rules MAY alternatively be installed on all the interfaces (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4552-3-4`](#rfc4552-3-4) When OSPFv3 authentication is enabled, OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded (§3) | no test | no test carries this requirement id; annotated {not-applicable}: the per-packet silent discard of unprotected inbound OSPFv3 is done by the kernel XFRM inbound require-policy; ze installs that policy (buildIPsecPolicies SADirIn, ipsec_install.go) and only samples the drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32), never inspecting or dropping a packet |
| [`RFC4552-3-5`](#rfc4552-3-5) When OSPFv3 authentication is enabled, OSPFv3 packets that fail the authentication checks MUST be silently discarded (§3) | no test | no test carries this requirement id; annotated {not-applicable}: the discard of an OSPFv3 packet that fails the integrity check is performed by the kernel XFRM transform; ze reads the resulting XfrmInIntegFailures/XfrmInStateProtoError counters (ipsec_drops_linux.go:24-30) but never verifies or drops a packet itself |
| [`RFC4552-4-3`](#rfc4552-4-3) When OSPFv3 confidentiality is enabled, OSPFv3 packets that are not protected with ESP MUST be silently discarded (§4) | no test | no test carries this requirement id; annotated {not-applicable}: an unprotected packet on a confidentiality-enabled interface is dropped by the kernel XFRM inbound require-policy (buildIPsecPolicies SADirIn, ipsec_install.go), not by ze |
| [`RFC4552-4-4`](#rfc4552-4-4) When OSPFv3 confidentiality is enabled, OSPFv3 packets that fail the confidentiality checks MUST be silently discarded (§4) | no test | no test carries this requirement id; annotated {not-applicable}: an ESP packet that fails decryption/integrity is dropped by the kernel XFRM transform; ze surfaces the drop via the XFRM counters (ipsec_drops_linux.go:32) but never decrypts or discards a packet |
| [`RFC4552-6-7`](#rfc4552-6-7) The algorithm key words (MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT) that appear in RFC 4305 [N6] are to be interpreted per RFC 2119 for OSPFv3 support as well, except when in conflict with the stream-cipher prohibition (§6) | no test | no test carries this requirement id; annotated {not-applicable}: an interpretation clause importing RFC 4305 [N6] algorithm keywords; RFC 4552 defines no independent behavior here. The concrete algorithm conformance lives in the config enums (ipsecAuthKeyLen/ipsecEncKeyLen, config.go:106-117) and the stream-cipher carve-out is RFC4552-6-6 |
| [`RFC4552-6-9`](#rfc4552-6-9) IP encapsulation of ESP packets MUST be supported (§6) | no test | no test carries this requirement id; annotated {not-applicable}: the IP encapsulation of ESP packets is the per-packet ESP framing performed by the kernel XFRM transform (internal/component/ike/dataplane/xfrm_linux.go); ze installs the ESP SA but never builds an ESP header |
| [`RFC4552-6-11`](#rfc4552-6-11) The IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver (§6) | no test | no test carries this requirement id; annotated {not-applicable}: multiple SAs with the same selectors between two peers is a kernel SAD capability (states keyed by SPI); ze installs one SA per interface with a distinct SPI (buildIPsecSA, ipsec_install.go) and delegates SA storage to the kernel XFRM |
| [`RFC4552-9-1`](#rfc4552-9-1) A different SA than the SA of the underlying interface MUST be provided for virtual links (§9) | {gap}, no test | OSPFv3 virtual-link IPsec is unimplemented; the installer consumes only configured-interface IPsec blocks (setConfig over cfg.Interfaces, ipsec_install.go) and no virtual link ever installs an SA. Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-2`](#rfc4552-9-2) Routers that implement this specification MUST change the way source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled (§9) | {gap}, no test | ze never enables IPsec on a virtual link, so it does not switch virtual-link source/destination selection when IPsec is on; the endpoints stay RFC 5340 §2.9 routed globals (v6ResolveVirtualEndpointLocked, virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-3`](#rfc4552-9-3) The first IPv6 address with the "LA-bit" set in prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link (§9) | {gap}, no test | virtual-link IPsec is unimplemented, so the RFC 4552 §9 rule of using the first LA-bit intra-area-prefix address as the source is not applied; ze selects the source from RFC 5340 routed globals (v6RouterGlobalAddr, virtuallink_v6.go:42-81). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-9-4`](#rfc4552-9-4) The first IPv6 address with the "LA-bit" set in prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link (§9) | {gap}, no test | the matching destination rule (first LA-bit intra-area-prefix address of the virtual neighbor) is likewise unapplied because virtual-link IPsec is unimplemented (virtuallink_v6.go:28-35). Disclosed in docs/features/rfc-status.md RFC 4552 row |
| [`RFC4552-11-4`](#rfc4552-11-4) The virtual-link SPD rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link (§11) | {gap}, no test | no virtual-link SPD rules are installed on transit-area interfaces because virtual-link IPsec is unimplemented (ipsec_install.go installs only configured-interface policies). Disclosed in docs/features/rfc-status.md RFC 4552 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4552-2-2`](#rfc4552-2-2)

All implementations conforming to this specification MUST support transport mode SA to provide required IPsec security to OSPFv3 packets (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L152) | unit/verify | unproven |

### [`RFC4552-3-1`](#rfc4552-3-1)

Implementations conforming to this specification MUST support authentication for OSPFv3 (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecESPRequiresIntegrity`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L104) | unit/verify | unproven |
| positive | [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L48) | unit/verify | unproven |

### [`RFC4552-3-2`](#rfc4552-3-2)

In order to provide authentication to OSPFv3, implementations MUST support ESP (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L69) | unit/verify | unproven |

### [`RFC4552-3-4`](#rfc4552-3-4)

When OSPFv3 authentication is enabled, OSPFv3 packets that are not protected with AH or ESP MUST be silently discarded (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-3-4, so no unit is bound to it.

### [`RFC4552-3-5`](#rfc4552-3-5)

When OSPFv3 authentication is enabled, OSPFv3 packets that fail the authentication checks MUST be silently discarded (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-3-5, so no unit is bound to it.

### [`RFC4552-4-2`](#rfc4552-4-2)

If confidentiality is provided, ESP MUST be used (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecAHWithEncryptionRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L87) | unit/verify | unproven |
| positive | [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L202) | unit/verify | unproven |

### [`RFC4552-4-3`](#rfc4552-4-3)

When OSPFv3 confidentiality is enabled, OSPFv3 packets that are not protected with ESP MUST be silently discarded (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-4-3, so no unit is bound to it.

### [`RFC4552-4-4`](#rfc4552-4-4)

When OSPFv3 confidentiality is enabled, OSPFv3 packets that fail the confidentiality checks MUST be silently discarded (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-4-4, so no unit is bound to it.

### [`RFC4552-6-1`](#rfc4552-6-1)

IPsec in transport mode MUST be supported (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L190) | unit/verify | unproven |

### [`RFC4552-6-2`](#rfc4552-6-2)

The implementation MUST support multiple SPDs with an SPD selection function that chooses a specific SPD based on interface (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L192) | unit/verify | unproven |

### [`RFC4552-6-3`](#rfc4552-6-3)

The implementation MUST be able to use source address, destination address, protocol, and direction as selectors in the SPD (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L195) | unit/verify | unproven |

### [`RFC4552-6-4`](#rfc4552-6-4)

The implementation MUST be able to tag inbound packets with the ID of the (physical or virtual) interface via which they arrived (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L198) | unit/verify | unproven |

### [`RFC4552-6-5`](#rfc4552-6-5)

Manually configured keys MUST be able to secure the specified traffic (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSAParamsSharedKey`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L248) | unit/verify | unproven |

### [`RFC4552-6-6`](#rfc4552-6-6)

The implementation MUST NOT allow the user to choose stream ciphers as the encryption algorithm for securing OSPFv3 packets (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecStreamCipherRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L223) | unit/verify | unproven |
| positive | [`TestIPsecESPConfidentialityValid`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L204) | unit/verify | unproven |

### [`RFC4552-6-7`](#rfc4552-6-7)

The algorithm key words (MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT) that appear in RFC 4305 [N6] are to be interpreted per RFC 2119 for OSPFv3 support as well, except when in conflict with the stream-cipher prohibition (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-6-7, so no unit is bound to it.

### [`RFC4552-6-9`](#rfc4552-6-9)

IP encapsulation of ESP packets MUST be supported (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-6-9, so no unit is bound to it.

### [`RFC4552-6-11`](#rfc4552-6-11)

The IPsec implementation MUST support the establishment and maintenance of multiple SAs with the same selectors between a given sender and receiver (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-6-11, so no unit is bound to it.

### [`RFC4552-7-1`](#rfc4552-7-1)

The implementations MUST use manually configured keys with the same SA parameters (SPI, keys, etc.) for both inbound and outbound SAs (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecSAIsWildcardWithOSPFSelector`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L139) | unit/verify | unproven |

### [`RFC4552-9-1`](#rfc4552-9-1)

A different SA than the SA of the underlying interface MUST be provided for virtual links (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-9-1, so no unit is bound to it.

### [`RFC4552-9-2`](#rfc4552-9-2)

Routers that implement this specification MUST change the way source and destination addresses are chosen for packets exchanged over virtual links when IPsec is enabled (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-9-2, so no unit is bound to it.

### [`RFC4552-9-3`](#rfc4552-9-3)

The first IPv6 address with the "LA-bit" set in prefixes advertised in intra-area-prefix-LSAs in the transit area MUST be used as the source address for packets exchanged over the virtual link (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-9-3, so no unit is bound to it.

### [`RFC4552-9-4`](#rfc4552-9-4)

The first IPv6 address with the "LA-bit" set in prefixes received in intra-area-prefix-LSAs from the virtual neighbor in the transit area MUST be used as the destination address for packets exchanged over the virtual link (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-9-4, so no unit is bound to it.

### [`RFC4552-11-1`](#rfc4552-11-1)

The IPsec protection barrier MUST be around the OSPF protocol so that all inbound and outbound OSPF traffic goes through IPsec processing (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecPoliciesInterfaceScopedWildcard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L201) | unit/verify | unproven |

### [`RFC4552-11-2`](#rfc4552-11-2)

The SPD selection function MUST return an SPD with the bypass rule for all interfaces that have OSPFv3 authentication/confidentiality disabled (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecDisabledInterfaceBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L278) | unit/verify | unproven |

### [`RFC4552-11-3`](#rfc4552-11-3)

The SPD selection function MUST return an SPD with the protect rules for all interfaces that have OSPFv3 authentication/confidentiality enabled (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPsecInstallOnInterfaceUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_install_test.go#L74) | unit/verify | unproven |

### [`RFC4552-11-4`](#rfc4552-11-4)

The virtual-link SPD rules MUST be installed in the SPD for the interfaces that are connected to the transit area for the virtual link (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4552-11-4, so no unit is bound to it.

### [`RFC4552-12-1`](#rfc4552-12-1)

The implementations MUST allow the administrator to configure the cryptographic and authentication keys in hexadecimal format (§12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPsecNonHexKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L237) | unit/verify | unproven |
| positive | [`TestParseOSPFIPsecConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_ipsec_test.go#L42) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 4552, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4552, so its obligations are stated where they were written.
