# RFC 4577 - OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs)

Not supported. Every requirement this repository extracted from RFC 4577, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11.1% | 2 of 18 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 18 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 36 | of 56 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 18 | of 36 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 88.9% | 16 of 18 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 18 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Not supported |
| Enrolment | Enrolled |
| Requirements | 56 |
| Gated MUST-level | 36 |
| Obligations that bind Ze | 18 |
| Not applicable, so out of scope | 18 |
| Declared gaps | 20 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4577.md` |
| Requirement shard | `rfc/requirements/rfc4577.md` |
| RFC text | `rfc/full/rfc4577.txt` |

## Enrolment

Enrolled: OSPF as the PE/CE Protocol for BGP/MPLS IP VPNs

## What the public ledger says

**Status:** Not supported

**What the ledger says is covered**

None of the PE-side procedures: ze has no VRF, no per-VRF OSPF instance, no Domain Identifier / OSPF Route Type / OSPF Router ID extended community, no DN-bit setting or checking, no VPN Route Tag and no sham link. What exists is the OSPF machinery the spec builds on: independent OSPF instances per RFC 6549 Instance ID ([`internal/plugins/ospf/multi_instance.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multi_instance.go),68), Type 3 / Type 5 / Type 7 origination with a configurable external route tag ([`internal/plugins/ospf/redist_wiring.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/redist_wiring.go), [`internal/plugins/ospf/lsdb/origination.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination.go)), OSPF cryptographic authentication ([`internal/plugins/ospf/auth_keystore.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore.go), [`internal/plugins/ospf/packet/auth_verify.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify.go)), virtual links ([`internal/plugins/ospf/virtual_link.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link.go)), and an OSPF-to-BGP redistribution export carrying only route-table prefixes ([`internal/plugins/ospf/redistribute/source.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/redistribute/source.go)). Requirements bound per line in [`rfc/short/rfc4577.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4577.md).

**What the ledger says remains**

Twenty gaps, annotated in [`rfc/short/rfc4577.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4577.md) and gated by `./le rfc check`: [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1), [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1), [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2), [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1) and [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2) (independent instances exist, [`internal/plugins/ospf/multi_instance.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multi_instance.go),68, but nothing associates one with an OSPF domain, a VRF or a Domain Identifier, and no configuration surface offers such an association); [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1), [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2) and [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1) (OptionDN is defined and encodable, [`internal/plugins/ospf/types/options.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/options.go), but no originator sets it); [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3) (no receive-side DN check, [`internal/plugins/ospf/spf/external.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external.go)); [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4) and [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7) (the External Route Tag is decoded, [`internal/plugins/ospf/packet/lsa_external.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/lsa_external.go), and is copied through by the NSSA Type 7 -> Type 5 translator, [`internal/plugins/ospf/nssa.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa.go), but no receive-side producer ever tests it -- the route calculation never reads it, [`internal/plugins/ospf/spf/external.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external.go)); [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3), [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2), [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3), [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6), [`RFC4577-4.2.5.2-8`](#rfc4577-4.2.5.2-8) and [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2) (a redistribution route tag is configurable and placed in Type 5 LSAs, [`internal/plugins/ospf/redist_wiring.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/redist_wiring.go),169-176, but it defaults to 0 and is not a VRF-scoped VPN Route Tag); [`RFC4577-4.2.5.2-9`](#rfc4577-4.2.5.2-9) (Type 5 origination makes ze an ASBR, LSDB.SelfIsASBR at [`internal/plugins/ospf/lsdb/origination.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination.go), but no domain test scopes it); [`RFC4577-4.2.6-8`](#rfc4577-4.2.6-8) (the OSPF distance reaches the redistribution event, [`internal/plugins/ospf/redistribute/source.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/redistribute/source.go), then is dropped before the BGP announcement, [`internal/component/bgp/plugins/redistribute_egress/redistribute.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/redistribute_egress/redistribute.go)); [`RFC4577-4.2.8.1-3`](#rfc4577-4.2.8.1-3) (one shared DefaultExternalMetric, [`internal/plugins/ospf/config.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config.go)). The remaining thirty-four requirements are recorded {not-applicable} with the grep proving no producer exists.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 34 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **36** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4577-4.2.6-5`](#rfc4577-4.2.6-5), [`RFC4577-6-1`](#rfc4577-6-1)

**Annotated instead of tested (34):** [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1), [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1), [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2), [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2), [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1), [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2), [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3), [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4), [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5), [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1), [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2), [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1), [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2), [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1), [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3), [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4), [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3), [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1), [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2), [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3), [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4), [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5), [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6), [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7), [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2), [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1), [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1), [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2), [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3), [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1), [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1), [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1), [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2), [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4577-4.1.1-1` | A PE attaching to more than one OSPF domain must run an independent instance of OSPF for each domain (§4.1.1, §4.2.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does run fully independent OSPF instances -- one complete engine per configured Instance ID with its own LSDB and neighbor table, demuxed by the shared dispatcher (installInstanceEncoders / recordInstanceMismatch, internal/plugins/ospf/multi_instance.go:33,68; per-instance engine skeleton instance.go) -- but an instance is keyed by the RFC 6549 Instance ID, never by an OSPF domain: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), so nothing binds an instance to a domain and a PE attached to two domains cannot separate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.1-1` | The PE must support one OSPF instance for each OSPF domain to which it attaches (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-instance engine exists (multi_instance.go:33,68; instance.go) and several instances run side by side, but the instance set is derived from the configured Instance IDs (ospfConfig.instanceIDSet / forInstance, config.go), not from a set of OSPF domains, and ze has no OSPF-domain concept to derive it from: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), and `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing. These four requirements share one premise and now share one verdict: ze DOES run OSPF instances (multi_instance.go:33,68; per-instance engine in instance.go), and each is an unconditional MUST to associate that existing instance with an OSPF domain, a VRF or a Domain Identifier. "ze never implemented the thing to associate with" is the unmet obligation itself, not a reason the obligation does not apply -- otherwise every unimplemented feature would read not-applicable and no gap could ever be reached. The conditional siblings (RFC4577-4.1.1-2, 4.2.4-3, 4.2.4-4, 4.2.4-5) stay not-applicable: they constrain a relationship BETWEEN associations that do not exist, so no producer can violate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.1-2` | Each OSPF instance must be associated with a single VRF (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs OSPF instances but associates none of them with a VRF, because it has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). These four requirements share one premise and now share one verdict: ze DOES run OSPF instances (multi_instance.go:33,68; per-instance engine in instance.go), and each is an unconditional MUST to associate that existing instance with an OSPF domain, a VRF or a Domain Identifier. "ze never implemented the thing to associate with" is the unmet obligation itself, not a reason the obligation does not apply -- otherwise every unimplemented feature would read not-applicable and no gap could ever be reached. The conditional siblings (RFC4577-4.1.1-2, 4.2.4-3, 4.2.4-4, 4.2.4-5) stay not-applicable: they constrain a relationship BETWEEN associations that do not exist, so no producer can violate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.1.1-2` | If two interfaces belong to the same OSPF instance, both interfaces must be associated with the same VRF (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355); interfaces are enrolled into an area and an Instance ID (interfaceConfig, internal/plugins/ospf/config.go), never into a VRF |
| `RFC4577-4.2.4-1` | Each OSPF instance must be associated with one or more Domain Identifiers (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs OSPF instances but associates none of them with a Domain Identifier, because no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed |
| `RFC4577-4.2.4-2` | Domain Identifier association must be configurable (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no configuration surface binds an OSPF instance to a Domain Identifier, because no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; `grep -rniE "vpn\|domain\|sham" internal/plugins/ospf/yang/` returns nothing, so no such leaf is configurable |
| `RFC4577-4.2.4-3` | If an OSPF instance has multiple Domain Identifiers, the primary one must be determinable by configuration (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so there is no set of Domain Identifiers and no primary to select |
| `RFC4577-4.2.4-4` | If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier must not be one of them (§4.2.4) | MUST NOT | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so no instance can hold more than one |
| `RFC4577-4.2.4-5` | If an OSPF instance has a non-NULL Domain Identifier, BGP-distributed VPN-IPv4 routes from it must carry the Domain Identifier Extended Communities attribute for the instance's primary Domain Identifier (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| `RFC4577-4.2.6-1` | The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| `RFC4577-4.2.6-2` | The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Route Type extended community exists -- `grep -rnE "0x0306\|0x8000" --include=*.go internal/core/bgp internal/component/bgp` returns no OSPF-related hit and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| `RFC4577-4.2.5.1-1` | When a type 3 LSA is sent from a PE to a CE, the DN bit in the LSA Options field must be set (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Type 3 Summary-LSA origination exists and takes an Options argument (LSDB.OriginateSummary, internal/plugins/ospf/lsdb/origination.go:391, header built at :408), but the caller passes the AREA's options (internal/plugins/ospf/spf/summary.go:102 passing in.Options[dst]) and the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it, so a Type 3 LSA sent to a neighbor never carries DN. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.1-2` | When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Type 5 AS-External origination exists and takes an Options argument (LSDB.OriginateExternal, internal/plugins/ospf/lsdb/origination.go:421), but the redistribution caller passes types.OptionE alone (internal/plugins/ospf/redist_wiring.go:61) and the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.8.1-1` | The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the external LSA that reports a redistributed route is originated with types.OptionE only (internal/plugins/ospf/redist_wiring.go:61 -> internal/plugins/ospf/lsdb/origination.go:421); the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it, and there is no domain comparison to decide the LSA reports a different-domain route (no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.6-3` | When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path never consults the DN bit. The OSPFv2 external reader accepts any Type 5 / Type 7 LSA and inspects Options only for OptionNP (v4ExternalReader, internal/plugins/ospf/spf/external.go:131-161, the single Options read at :151); the OSPFv3 accessor PrefixOptions.Down() (internal/plugins/ospf/v3/types/prefix.go:69) has no non-test caller. A received LSA with DN set is used by the route calculation like any other. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.6-4` | If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the External Route Tag is decoded off the wire into ExternalLSA.ExternalRouteTag (internal/plugins/ospf/packet/lsa_external.go:33) but the route calculation never reads it -- v4ExternalReader builds its ExternalRecord from prefix, metric, type and forwarding address only (internal/plugins/ospf/spf/external.go:131-161); the one receive-side consumer, the NSSA Type 7 -> Type 5 translator, copies the tag through rather than testing it (internal/plugins/ospf/nssa.go:226) -- and no VPN Route Tag value exists to compare against (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments and no code). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.1-3` | All implementations adhering to this specification must by default support the VPN Route Tag procedures of Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2 (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPF route tag machinery is present on both origination (externalParams -> OriginateExternal tag argument, internal/plugins/ospf/redist_wiring.go:169-176,61) and decode (internal/plugins/ospf/packet/lsa_external.go:33), but none of the sec 4.2.5.2 / 4.2.8.1 / 4.2.8.2 VPN Route Tag procedures are built on it: no default VPN tag, no receive-side tag suppression, and ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.2-1` | If a VRF is associated with an OSPF instance, by default it must be configured with a VPN Route Tag value (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), so no VRF can be configured with a VPN Route Tag |
| `RFC4577-4.2.5.2-2` | By default the VPN Route Tag must be included in the Type 5 LSAs the PE originates from BGP VPN-IPv4 routes and sends to attached CEs (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does originate Type 5 LSAs from redistributed BGP routes and does place a route tag in them (internal/plugins/ospf/redist_wiring.go:61 -> internal/plugins/ospf/lsdb/origination.go:421 writing ExternalRouteTag), but the tag comes from the per-source redistribute configuration and defaults to 0 (externalParams, internal/plugins/ospf/redist_wiring.go:169-176) -- there is no VPN Route Tag default, and no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.2-3` | The VPN Route Tag value must be configurable (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route tag IS configurable, per redistribution source (redistributeConfig.Tag read by externalParams, internal/plugins/ospf/redist_wiring.go:169-176; parsed at config.go:1189-1190), but it is a redistribution route tag, not a VRF-scoped VPN Route Tag: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.2-4` | If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no VPN backbone AS number reaches the OSPF configuration and no VPN Route Tag exists (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments, options.go:30 and v3/types/prefix.go:53, and no code) |
| `RFC4577-4.2.5.2-5` | A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no VPN Route Tag and no VPN exist to be distinct from (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments and no code; ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355)) |
| `RFC4577-4.2.5.2-6` | Each PE-originated Type 5 LSA for an extra-domain route must contain an OSPF route tag whose value is the VPN Route Tag (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the PE-originated Type 5 carries whatever tag the redistribute entry configures (internal/plugins/ospf/redist_wiring.go:61,169-176), defaulting to 0; no VPN Route Tag value is computed or stored, and no extra-domain test exists to condition it on (no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.2-7` | The VPN Route Tag must be used to ensure a Type 5 LSA originated by a PE is not redistributed to another PE (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no receive-side producer FILTERS on a tag. A received tag is not entirely inert -- the NSSA translator copies a Type 7 body's ExternalRouteTag into the Type 5 it originates (internal/plugins/ospf/nssa.go:226) -- but it is never used as an acceptance test: the external reader builds its ExternalRecord from prefix, metric, type and forwarding address only and never reads ExternalRouteTag (internal/plugins/ospf/spf/external.go:131-161) even though the decoder fills it (internal/plugins/ospf/packet/lsa_external.go:33), so no tag value can stop a Type 5 LSA from being taken into the route calculation and re-exported by the redistribution source (internal/plugins/ospf/redistribute/source.go:99-137). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.8.1-2` | The VPN Route Tag must be placed in the external LSA unless its use has been turned off by configuration (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the external LSA is originated with the configured redistribution tag, which defaults to 0 rather than to a VPN Route Tag (externalParams, internal/plugins/ospf/redist_wiring.go:169-176 -> OriginateExternal, lsdb/origination.go:421), and there is no VPN Route Tag setting to turn off. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.6-5` | Routes that a PE receives in type 4 LSAs must not be redistributed to BGP (§4.2.6) | MUST NOT | 4.2.6 | **positive:** `unit/verify` [`TestRFC4577Type3SummaryBecomesRedistributableRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L18). **negative:** `unit/verify` [`TestRFC4577Type4SummaryNotRedistributed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L41) |
| `RFC4577-4.1.4-1` | If the OSPF domain has any area 0 routers other than the PE routers, at least one must be a CE router and must have an area 0 link (possibly a virtual link) to at least one PE router (§4.1.4, §4.2.3) | MUST | 4.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains the operator's OSPF domain topology around PE routers; ze models no PE or CE role -- interfaces are configured into an area with no provider/customer distinction (interfaceConfig, internal/plugins/ospf/config.go) and ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355) |
| `RFC4577-4.2.7.1-1` | The Sham Link Endpoint Address associated with a VRF must be configurable (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty) |
| `RFC4577-4.2.7.1-2` | The Sham Link Endpoint Address must be distributed by BGP as a VPN-IPv4 address whose IPv4 prefix part is 32 bits long (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty); and no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| `RFC4577-4.2.7.1-3` | The Sham Link Endpoint Address must not be advertised by OSPF (§4.2.7.1) | MUST NOT | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no such address can be advertised or withheld |
| `RFC4577-4.2.7.2-1` | The sham link endpoint address must not be used as the endpoint address of an OSPF Virtual Link (§4.2.7.2) | MUST NOT | 4.2.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty); virtual links exist (internal/plugins/ospf/virtual_link.go) but there is no sham link endpoint address that could be offered as a virtual-link endpoint |
| `RFC4577-4.2.7.3-1` | The OSPF metric associated with a sham link must be configurable, and there must be a configurable default (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so there is no sham-link metric to configure or default |
| `RFC4577-4.2.7.4-1` | Any route (other than one whose next hop is the sham link) advertised in an LSA transmitted over a sham link must also be redistributed into BGP (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no LSA is ever transmitted over one |
| `RFC4577-4.2.7.4-2` | When forwarding a packet whose preferred route has the sham link as its next-hop interface, the packet must be forwarded according to the corresponding BGP route (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no route can have a sham link as its next-hop interface |
| `RFC4577-4.2.7.4-3` | A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty) |
| `RFC4577-6-1` | OSPF cryptographic authentication must be implemented on each PE (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L22). **negative:** `unit/verify` [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L46) |
| `RFC4577-4.2.4-6` | The default Domain Identifier value (if none is configured) should be NULL (§4.2.4) | SHOULD | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so there is no value to default |
| `RFC4577-4.2.5.2-8` | If the VPN backbone AS number is two bytes long, the default VPN Route Tag should be the automatically computed tag based on that AS number (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the tag default is the literal 0 set by externalParams (internal/plugins/ospf/redist_wiring.go:170), never a value computed from an AS number; the OSPF configuration carries no VPN backbone AS to compute it from (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.5.2-9` | A PE distributing to a CE a route from outside the CE's OSPF domain should present itself as an ASBR and should report such routes as AS-external routes (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does present itself as an ASBR and does report redistributed routes as AS-external routes -- the redistribution consumer originates Type 5 AS-External-LSAs (internal/plugins/ospf/redistribute/consumer.go InjectRoute -> redist_wiring.go:61) and the self-originated-external index drives the Router-LSA E-bit (LSDB.SelfIsASBR, internal/plugins/ospf/lsdb/origination.go:518) -- but the condition this SHOULD is scoped to, the route coming from outside the CE's OSPF domain, has no producer: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.6-6` | For backward compatibility, OSPF Route Type type 8000 should be accepted and treated as 0306 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Route Type extended community is parsed at all -- `grep -rnE "0x0306\|0x8000" --include=*.go internal/core/bgp internal/component/bgp` returns no OSPF-related hit -- so there is no 0306 handling for 8000 to be mapped onto |
| `RFC4577-4.2.6-7` | For backward compatibility, OSPF Router ID type 8001 should be accepted and treated as 0107 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Router ID extended community is parsed at all -- `grep -rnE "0x0107\|0x8001" --include=*.go internal/core/bgp internal/component/bgp` returns no OSPF-related hit |
| `RFC4577-4.2.6-8` | The MED of a PE-originated VPN-IPv4 OSPF route should by default be set to the OSPF distance of the route plus 1 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPF distance IS carried out of the SPF route table into the redistribution event (addEntry / metricToUint32, internal/plugins/ospf/redistribute/source.go:127-143), but the BGP egress consumer drops it: dispatchEntryToConsumer builds a configredist.RouteEntry from Prefix, NextHop, Source, Peer, OriginASN and Community only (internal/component/bgp/plugins/redistribute_egress/redistribute.go:263-270) and RouteEntry has no MED or metric field (internal/component/config/redistribute/consumer.go:28-54), so no MED is derived from the OSPF distance for any redistributed OSPF route. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.2.7.3-2` | Sham links should be treated by OSPF as OSPF Demand Circuits (§4.2.7.3) | SHOULD | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty); the DoNotAge machinery exists (LSAge.DoNotAge, internal/plugins/ospf/types/lsage.go:48, honored in lsdb/entry.go:88) but no sham link exists to treat as a demand circuit |
| `RFC4577-4.2.7.4-4` | If a PE determines the next hop interface for a route is a sham link, it should not redistribute that route into BGP as a VPN-IPv4 route (§4.2.7.4) | SHOULD NOT | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no next-hop interface can be one |
| `RFC4577-6-2` | OSPF cryptographic authentication should be used between a PE and a CE (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC4577InterfaceUsesConfiguredCryptoAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L69). **negative:** no negative test. **{single-polarity}:** the negative -- an interface with no key chain sending unauthenticated OSPF packets -- is the state this SHOULD recommends against, not a behavior the implementation must exhibit, so asserting it would pin the unrecommended path instead of the requirement |
| `RFC4577-4.2.4-7` | If the OSPF instance's Domain Identifier is NULL, the Domain Identifier Extended Communities attribute may be omitted from BGP-distributed routes (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed |
| `RFC4577-4.2.4-8` | Alternatively, a Domain Identifier Extended Communities attribute value representing NULL may be carried with the route (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed |
| `RFC4577-4.2.6-9` | If the OSPF instance has only a NULL Domain Identifier, the OSPF Domain Identifier Extended Communities attribute may be omitted from a PE-originated route (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed |
| `RFC4577-4.2.6-10` | For backward compatibility, the OSPF Domain Identifier type 8005 may be used and is treated as if it were 0005 (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so neither 0005 nor 8005 is ever emitted or read |
| `RFC4577-4.2.8.1-3` | The default type 1 metric and the default type 2 metric may be different (§4.2.8.1) | MAY | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is a single default external metric constant, DefaultExternalMetric = 20 (internal/plugins/ospf/config.go:34), applied by externalParams regardless of metric type (internal/plugins/ospf/redist_wiring.go:170) and by the config parser (config.go:1179), so the type 1 and type 2 defaults cannot differ. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| `RFC4577-4.1.4-2` | The CE-to-PE area 0 adjacency may be via an OSPF virtual link (OPTIONAL feature) (§4.1.4, §4.2.3) | MAY | 4.1.4 | **positive:** `unit/verify` [`TestRFC4577VirtualLinkGivesArea0Adjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L92). **negative:** no negative test. **{single-polarity}:** this is a MAY, the permission to reach the area 0 adjacency over a virtual link. A negative would have to assert that some configuration does NOT yield a backbone virtual link, which exercises virtual-link configuration handling rather than the permission this requirement grants |
| `RFC4577-4.2.5.1-4` | When the VPN Route Tag is no longer needed for backward compatibility, its use (sending and receiving) may be disabled by configuration (§4.2.5.1, §4.2.5.2) | MAY | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no VPN Route Tag exists to enable or disable (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments and no code; the only configurable tag is the redistribution route tag, internal/plugins/ospf/redist_wiring.go:169-176) |
| `RFC4577-4.1.3-1` | Sham links are an OPTIONAL feature of this specification (§4.1.3, §4.2.7) | OPTIONAL | 4.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty) |
| `RFC4577-4.2.6-11` | The OSPF Router ID Extended Communities attribute is an OPTIONAL attribute (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no OSPF Router ID extended community exists -- `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing |
| `RFC4577-4.2.6-12` | The Site of Origin attribute is OPTIONAL for routes a PE learns from a CE via OSPF (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a Site of Origin extended community can be built from a text route specification (internal/core/bgp/attribute/builder_parse.go:264), but no PE learns routes from a CE via OSPF: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), and no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| `RFC4577-4.2.7.1-4` | If a VRF is associated with a single OSPF instance and the PE's router id there is an IP address, the Sham Link Endpoint Address may default to that Router ID (§4.2.7.1) | MAY | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so there is no endpoint address to default from a router ID |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1) A PE attaching to more than one OSPF domain must run an independent instance of OSPF for each domain (§4.1.1, §4.2.1) | {gap}, no test | ze does run fully independent OSPF instances -- one complete engine per configured Instance ID with its own LSDB and neighbor table, demuxed by the shared dispatcher (installInstanceEncoders / recordInstanceMismatch, internal/plugins/ospf/multi_instance.go:33,68; per-instance engine skeleton instance.go) -- but an instance is keyed by the RFC 6549 Instance ID, never by an OSPF domain: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), so nothing binds an instance to a domain and a PE attached to two domains cannot separate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1) The PE must support one OSPF instance for each OSPF domain to which it attaches (§4.2.1) | {gap}, no test | the per-instance engine exists (multi_instance.go:33,68; instance.go) and several instances run side by side, but the instance set is derived from the configured Instance IDs (ospfConfig.instanceIDSet / forInstance, config.go), not from a set of OSPF domains, and ze has no OSPF-domain concept to derive it from: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), and `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing. These four requirements share one premise and now share one verdict: ze DOES run OSPF instances (multi_instance.go:33,68; per-instance engine in instance.go), and each is an unconditional MUST to associate that existing instance with an OSPF domain, a VRF or a Domain Identifier. "ze never implemented the thing to associate with" is the unmet obligation itself, not a reason the obligation does not apply -- otherwise every unimplemented feature would read not-applicable and no gap could ever be reached. The conditional siblings (RFC4577-4.1.1-2, 4.2.4-3, 4.2.4-4, 4.2.4-5) stay not-applicable: they constrain a relationship BETWEEN associations that do not exist, so no producer can violate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2) Each OSPF instance must be associated with a single VRF (§4.2.1) | {gap}, no test | ze runs OSPF instances but associates none of them with a VRF, because it has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). These four requirements share one premise and now share one verdict: ze DOES run OSPF instances (multi_instance.go:33,68; per-instance engine in instance.go), and each is an unconditional MUST to associate that existing instance with an OSPF domain, a VRF or a Domain Identifier. "ze never implemented the thing to associate with" is the unmet obligation itself, not a reason the obligation does not apply -- otherwise every unimplemented feature would read not-applicable and no gap could ever be reached. The conditional siblings (RFC4577-4.1.1-2, 4.2.4-3, 4.2.4-4, 4.2.4-5) stay not-applicable: they constrain a relationship BETWEEN associations that do not exist, so no producer can violate them. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2) If two interfaces belong to the same OSPF instance, both interfaces must be associated with the same VRF (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355); interfaces are enrolled into an area and an Instance ID (interfaceConfig, internal/plugins/ospf/config.go), never into a VRF |
| [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1) Each OSPF instance must be associated with one or more Domain Identifiers (§4.2.4) | {gap}, no test | ze runs OSPF instances but associates none of them with a Domain Identifier, because no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed |
| [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2) Domain Identifier association must be configurable (§4.2.4) | {gap}, no test | no configuration surface binds an OSPF instance to a Domain Identifier, because no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; `grep -rniE "vpn\|domain\|sham" internal/plugins/ospf/yang/` returns nothing, so no such leaf is configurable |
| [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3) If an OSPF instance has multiple Domain Identifiers, the primary one must be determinable by configuration (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so there is no set of Domain Identifiers and no primary to select |
| [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4) If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier must not be one of them (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed, so no instance can hold more than one |
| [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5) If an OSPF instance has a non-NULL Domain Identifier, BGP-distributed VPN-IPv4 routes from it must carry the Domain Identifier Extended Communities attribute for the instance's primary Domain Identifier (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1) The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2) The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: no OSPF Route Type extended community exists -- `grep -rnE "0x0306\|0x8000" --include=*.go internal/core/bgp internal/component/bgp` returns no OSPF-related hit and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing; additionally no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1) When a type 3 LSA is sent from a PE to a CE, the DN bit in the LSA Options field must be set (§4.2.5.1) | {gap}, no test | Type 3 Summary-LSA origination exists and takes an Options argument (LSDB.OriginateSummary, internal/plugins/ospf/lsdb/origination.go:391, header built at :408), but the caller passes the AREA's options (internal/plugins/ospf/spf/summary.go:102 passing in.Options[dst]) and the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it, so a Type 3 LSA sent to a neighbor never carries DN. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2) When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1) | {gap}, no test | Type 5 AS-External origination exists and takes an Options argument (LSDB.OriginateExternal, internal/plugins/ospf/lsdb/origination.go:421), but the redistribution caller passes types.OptionE alone (internal/plugins/ospf/redist_wiring.go:61) and the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1) The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1) | {gap}, no test | the external LSA that reports a redistributed route is originated with types.OptionE only (internal/plugins/ospf/redist_wiring.go:61 -> internal/plugins/ospf/lsdb/origination.go:421); the DN bit itself is defined and wire-codable (OptionDN, internal/plugins/ospf/types/options.go:31, encoded by Options.WriteTo :52), but `grep -rn OptionDN --include=*.go internal/plugins/ospf/` finds only the constant, its display name table and the neighbor detail renderer (neighbor_detail.go:70) -- no originator sets it, and there is no domain comparison to decide the LSA reports a different-domain route (no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3) When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6) | {gap}, no test | the receive path never consults the DN bit. The OSPFv2 external reader accepts any Type 5 / Type 7 LSA and inspects Options only for OptionNP (v4ExternalReader, internal/plugins/ospf/spf/external.go:131-161, the single Options read at :151); the OSPFv3 accessor PrefixOptions.Down() (internal/plugins/ospf/v3/types/prefix.go:69) has no non-test caller. A received LSA with DN set is used by the route calculation like any other. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4) If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6) | {gap}, no test | the External Route Tag is decoded off the wire into ExternalLSA.ExternalRouteTag (internal/plugins/ospf/packet/lsa_external.go:33) but the route calculation never reads it -- v4ExternalReader builds its ExternalRecord from prefix, metric, type and forwarding address only (internal/plugins/ospf/spf/external.go:131-161); the one receive-side consumer, the NSSA Type 7 -> Type 5 translator, copies the tag through rather than testing it (internal/plugins/ospf/nssa.go:226) -- and no VPN Route Tag value exists to compare against (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments and no code). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3) All implementations adhering to this specification must by default support the VPN Route Tag procedures of Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2 (§4.2.5.1) | {gap}, no test | the OSPF route tag machinery is present on both origination (externalParams -> OriginateExternal tag argument, internal/plugins/ospf/redist_wiring.go:169-176,61) and decode (internal/plugins/ospf/packet/lsa_external.go:33), but none of the sec 4.2.5.2 / 4.2.8.1 / 4.2.8.2 VPN Route Tag procedures are built on it: no default VPN tag, no receive-side tag suppression, and ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1) If a VRF is associated with an OSPF instance, by default it must be configured with a VPN Route Tag value (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355), so no VRF can be configured with a VPN Route Tag |
| [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2) By default the VPN Route Tag must be included in the Type 5 LSAs the PE originates from BGP VPN-IPv4 routes and sends to attached CEs (§4.2.5.2) | {gap}, no test | ze does originate Type 5 LSAs from redistributed BGP routes and does place a route tag in them (internal/plugins/ospf/redist_wiring.go:61 -> internal/plugins/ospf/lsdb/origination.go:421 writing ExternalRouteTag), but the tag comes from the per-source redistribute configuration and defaults to 0 (externalParams, internal/plugins/ospf/redist_wiring.go:169-176) -- there is no VPN Route Tag default, and no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3) The VPN Route Tag value must be configurable (§4.2.5.2) | {gap}, no test | a route tag IS configurable, per redistribution source (redistributeConfig.Tag read by externalParams, internal/plugins/ospf/redist_wiring.go:169-176; parsed at config.go:1189-1190), but it is a redistribution route tag, not a VRF-scoped VPN Route Tag: ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4) If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: no VPN backbone AS number reaches the OSPF configuration and no VPN Route Tag exists (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments, options.go:30 and v3/types/prefix.go:53, and no code) |
| [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5) A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: no VPN Route Tag and no VPN exist to be distinct from (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments and no code; ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355)) |
| [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6) Each PE-originated Type 5 LSA for an extra-domain route must contain an OSPF route tag whose value is the VPN Route Tag (§4.2.5.2) | {gap}, no test | the PE-originated Type 5 carries whatever tag the redistribute entry configures (internal/plugins/ospf/redist_wiring.go:61,169-176), defaulting to 0; no VPN Route Tag value is computed or stored, and no extra-domain test exists to condition it on (no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7) The VPN Route Tag must be used to ensure a Type 5 LSA originated by a PE is not redistributed to another PE (§4.2.5.2) | {gap}, no test | no receive-side producer FILTERS on a tag. A received tag is not entirely inert -- the NSSA translator copies a Type 7 body's ExternalRouteTag into the Type 5 it originates (internal/plugins/ospf/nssa.go:226) -- but it is never used as an acceptance test: the external reader builds its ExternalRecord from prefix, metric, type and forwarding address only and never reads ExternalRouteTag (internal/plugins/ospf/spf/external.go:131-161) even though the decoder fills it (internal/plugins/ospf/packet/lsa_external.go:33), so no tag value can stop a Type 5 LSA from being taken into the route calculation and re-exported by the redistribution source (internal/plugins/ospf/redistribute/source.go:99-137). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2) The VPN Route Tag must be placed in the external LSA unless its use has been turned off by configuration (§4.2.8.1) | {gap}, no test | the external LSA is originated with the configured redistribution tag, which defaults to 0 rather than to a VPN Route Tag (externalParams, internal/plugins/ospf/redist_wiring.go:169-176 -> OriginateExternal, lsdb/origination.go:421), and there is no VPN Route Tag setting to turn off. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1) If the OSPF domain has any area 0 routers other than the PE routers, at least one must be a CE router and must have an area 0 link (possibly a virtual link) to at least one PE router (§4.1.4, §4.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains the operator's OSPF domain topology around PE routers; ze models no PE or CE role -- interfaces are configured into an area with no provider/customer distinction (interfaceConfig, internal/plugins/ospf/config.go) and ze has no VRF model at all: `grep -rni vrf --include=*.go internal/plugins/ospf/` returns nothing, and no VRF table, import/export engine or per-VRF instance exists anywhere (the only `vrf` spellings under internal/component/bgp are flowspec redirect-to-VRF comments, route_community.go:82,183,355) |
| [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1) The Sham Link Endpoint Address associated with a VRF must be configurable (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty) |
| [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2) The Sham Link Endpoint Address must be distributed by BGP as a VPN-IPv4 address whose IPv4 prefix part is 32 bits long (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty); and no PE-originated VPN-IPv4 OSPF route exists: OSPF exports plain IPv4 unicast prefixes into the redistribution bus (emitDelta sets AFI ipv4 / SAFI unicast, internal/plugins/ospf/redistribute/source.go:105-107) and nothing anywhere turns an OSPF route into a VPN-IPv4 (SAFI 128) route |
| [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3) The Sham Link Endpoint Address must not be advertised by OSPF (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no such address can be advertised or withheld |
| [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1) The sham link endpoint address must not be used as the endpoint address of an OSPF Virtual Link (§4.2.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty); virtual links exist (internal/plugins/ospf/virtual_link.go) but there is no sham link endpoint address that could be offered as a virtual-link endpoint |
| [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1) The OSPF metric associated with a sham link must be configurable, and there must be a configurable default (§4.2.7.3) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so there is no sham-link metric to configure or default |
| [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1) Any route (other than one whose next hop is the sham link) advertised in an LSA transmitted over a sham link must also be redistributed into BGP (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no LSA is ever transmitted over one |
| [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2) When forwarding a packet whose preferred route has the sham link as its next-hop interface, the packet must be forwarded according to the corresponding BGP route (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty), so no route can have a sham link as its next-hop interface |
| [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3) A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: `grep -rni sham --include=*.go internal/` returns nothing: no sham link, no Sham Link Endpoint Address, no sham-link forwarding, and no VRF to relate two of them (`grep -rni vrf --include=*.go internal/plugins/ospf/` is also empty) |
| [`RFC4577-4.2.5.2-8`](#rfc4577-4.2.5.2-8) If the VPN backbone AS number is two bytes long, the default VPN Route Tag should be the automatically computed tag based on that AS number (§4.2.5.2) | {gap} | the tag default is the literal 0 set by externalParams (internal/plugins/ospf/redist_wiring.go:170), never a value computed from an AS number; the OSPF configuration carries no VPN backbone AS to compute it from (`grep -rni vpn --include=*.go internal/plugins/ospf/` returns two comments). Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.5.2-9`](#rfc4577-4.2.5.2-9) A PE distributing to a CE a route from outside the CE's OSPF domain should present itself as an ASBR and should report such routes as AS-external routes (§4.2.5.2) | {gap} | ze does present itself as an ASBR and does report redistributed routes as AS-external routes -- the redistribution consumer originates Type 5 AS-External-LSAs (internal/plugins/ospf/redistribute/consumer.go InjectRoute -> redist_wiring.go:61) and the self-originated-external index drives the Router-LSA E-bit (LSDB.SelfIsASBR, internal/plugins/ospf/lsdb/origination.go:518) -- but the condition this SHOULD is scoped to, the route coming from outside the CE's OSPF domain, has no producer: no OSPF Domain Identifier exists: `grep -rniE "domain.identifier\|domain-id\|domainid" --include=*.go internal/plugins/ospf internal/core/bgp internal/component/bgp` returns nothing, and `grep -rni ospf --include=*.go internal/core/bgp/attribute internal/component/bgp/route` returns nothing, so no OSPF extended community is ever built or parsed. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.6-8`](#rfc4577-4.2.6-8) The MED of a PE-originated VPN-IPv4 OSPF route should by default be set to the OSPF distance of the route plus 1 (§4.2.6) | {gap} | the OSPF distance IS carried out of the SPF route table into the redistribution event (addEntry / metricToUint32, internal/plugins/ospf/redistribute/source.go:127-143), but the BGP egress consumer drops it: dispatchEntryToConsumer builds a configredist.RouteEntry from Prefix, NextHop, Source, Peer, OriginASN and Community only (internal/component/bgp/plugins/redistribute_egress/redistribute.go:263-270) and RouteEntry has no MED or metric field (internal/component/config/redistribute/consumer.go:28-54), so no MED is derived from the OSPF distance for any redistributed OSPF route. Disclosed in docs/features/rfc-status.md RFC 4577 row |
| [`RFC4577-4.2.8.1-3`](#rfc4577-4.2.8.1-3) The default type 1 metric and the default type 2 metric may be different (§4.2.8.1) | {gap} | there is a single default external metric constant, DefaultExternalMetric = 20 (internal/plugins/ospf/config.go:34), applied by externalParams regardless of metric type (internal/plugins/ospf/redist_wiring.go:170) and by the config parser (config.go:1179), so the type 1 and type 2 defaults cannot differ. Disclosed in docs/features/rfc-status.md RFC 4577 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1)

A PE attaching to more than one OSPF domain must run an independent instance of OSPF for each domain (§4.1.1, §4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.1-1, so no unit is bound to it.

### [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1)

The PE must support one OSPF instance for each OSPF domain to which it attaches (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.1-1, so no unit is bound to it.

### [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2)

Each OSPF instance must be associated with a single VRF (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.1-2, so no unit is bound to it.

### [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2)

If two interfaces belong to the same OSPF instance, both interfaces must be associated with the same VRF (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.1-2, so no unit is bound to it.

### [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1)

Each OSPF instance must be associated with one or more Domain Identifiers (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-1, so no unit is bound to it.

### [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2)

Domain Identifier association must be configurable (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-2, so no unit is bound to it.

### [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3)

If an OSPF instance has multiple Domain Identifiers, the primary one must be determinable by configuration (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-3, so no unit is bound to it.

### [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4)

If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier must not be one of them (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-4, so no unit is bound to it.

### [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5)

If an OSPF instance has a non-NULL Domain Identifier, BGP-distributed VPN-IPv4 routes from it must carry the Domain Identifier Extended Communities attribute for the instance's primary Domain Identifier (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-5, so no unit is bound to it.

### [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1)

The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-1, so no unit is bound to it.

### [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2)

The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-2, so no unit is bound to it.

### [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1)

When a type 3 LSA is sent from a PE to a CE, the DN bit in the LSA Options field must be set (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-1, so no unit is bound to it.

### [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2)

When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-2, so no unit is bound to it.

### [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1)

The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-1, so no unit is bound to it.

### [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3)

When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-3, so no unit is bound to it.

### [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4)

If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-4, so no unit is bound to it.

### [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3)

All implementations adhering to this specification must by default support the VPN Route Tag procedures of Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2 (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-3, so no unit is bound to it.

### [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1)

If a VRF is associated with an OSPF instance, by default it must be configured with a VPN Route Tag value (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-1, so no unit is bound to it.

### [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2)

By default the VPN Route Tag must be included in the Type 5 LSAs the PE originates from BGP VPN-IPv4 routes and sends to attached CEs (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-2, so no unit is bound to it.

### [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3)

The VPN Route Tag value must be configurable (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-3, so no unit is bound to it.

### [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4)

If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-4, so no unit is bound to it.

### [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5)

A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-5, so no unit is bound to it.

### [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6)

Each PE-originated Type 5 LSA for an extra-domain route must contain an OSPF route tag whose value is the VPN Route Tag (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-6, so no unit is bound to it.

### [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7)

The VPN Route Tag must be used to ensure a Type 5 LSA originated by a PE is not redistributed to another PE (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-7, so no unit is bound to it.

### [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2)

The VPN Route Tag must be placed in the external LSA unless its use has been turned off by configuration (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-2, so no unit is bound to it.

### [`RFC4577-4.2.6-5`](#rfc4577-4.2.6-5)

Routes that a PE receives in type 4 LSAs must not be redistributed to BGP (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4577Type4SummaryNotRedistributed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L41) | unit/verify | unproven |
| positive | [`TestRFC4577Type3SummaryBecomesRedistributableRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L18) | unit/verify | unproven |

### [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1)

If the OSPF domain has any area 0 routers other than the PE routers, at least one must be a CE router and must have an area 0 link (possibly a virtual link) to at least one PE router (§4.1.4, §4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.4-1, so no unit is bound to it.

### [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1)

The Sham Link Endpoint Address associated with a VRF must be configurable (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-1, so no unit is bound to it.

### [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2)

The Sham Link Endpoint Address must be distributed by BGP as a VPN-IPv4 address whose IPv4 prefix part is 32 bits long (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-2, so no unit is bound to it.

### [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3)

The Sham Link Endpoint Address must not be advertised by OSPF (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-3, so no unit is bound to it.

### [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1)

The sham link endpoint address must not be used as the endpoint address of an OSPF Virtual Link (§4.2.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.2-1, so no unit is bound to it.

### [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1)

The OSPF metric associated with a sham link must be configurable, and there must be a configurable default (§4.2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.3-1, so no unit is bound to it.

### [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1)

Any route (other than one whose next hop is the sham link) advertised in an LSA transmitted over a sham link must also be redistributed into BGP (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-1, so no unit is bound to it.

### [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2)

When forwarding a packet whose preferred route has the sham link as its next-hop interface, the packet must be forwarded according to the corresponding BGP route (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-2, so no unit is bound to it.

### [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3)

A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-3, so no unit is bound to it.

### [`RFC4577-6-1`](#rfc4577-6-1)

OSPF cryptographic authentication must be implemented on each PE (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L22) | unit/verify | unproven |

### [`RFC4577-6-2`](#rfc4577-6-2)

OSPF cryptographic authentication should be used between a PE and a CE (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4577InterfaceUsesConfiguredCryptoAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L69) | unit/verify | unproven |

### [`RFC4577-4.1.4-2`](#rfc4577-4.1.4-2)

The CE-to-PE area 0 adjacency may be via an OSPF virtual link (OPTIONAL feature) (§4.1.4, §4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4577VirtualLinkGivesArea0Adjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L92) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 4577, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4577, so its obligations are stated where they were written.
