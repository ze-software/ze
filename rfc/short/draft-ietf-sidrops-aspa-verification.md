# draft-ietf-sidrops-aspa-verification-24: Verification of AS_PATH Using ASPA Objects

## Meta

| Field | Value |
|-------|-------|
| Document | draft-ietf-sidrops-aspa-verification-24 |
| Title | Verification of AS_PATH Using the Resource Certificate PKI and Autonomous System Provider Authorization |
| Status | Internet-Draft (Standards Track, IESG approved) |
| Date | 2024 |
| Authors | A. Azimov (Yandex), E. Bogomazov (Qrator Labs), R. Bush (IIJ & Arrcus), K. Patel (Arrcus), K. Sriram (NIST) |
| RFC Number | Not yet assigned as of March 2026 |
| Enrolment | enrolled |
| Enrolment reason | ASPA AS_PATH verification: 4 tested rows (4-1, 5.1-1, 5.4-2, 5.6-1) + 1 single-polarity positive (5.4-1 upstream application) + 1 gap (5.6-2 Invalid route not made ineligible for route selection) + 3 untested MUST rows added by the 2026-09-21 extraction walk (5-1 treat-as-withdraw on neighbor mismatch, 6.2-1 and 6.2-2 AFI/SAFI scope). That walk also re-anchored every id to the section it cites: the rows had been written against an earlier draft revision, so they named sections 6, 7 and 8, which in draft-24 are Deployment Recommendations, Security Considerations and Relation to Other Technologies. The section 4 registration MUSTs and the section 6.5 operator-notification MUST bind the ASPA registrant and are excluded in rfc/extraction/. |
| Implementation | ze |
| Implementation reason | Ze's own Go implements this document; each requirement row cites its producer. |
| Support | drafts 20 |
| Support area | ASPA path verification |
| Support status | Partial |
| Support coverage | Section 5 verification algorithm (upstream/downstream), prepend compression, AS0-in-provider rejection, RTR ASPA PDU (Type 11) consumption, and re-validation on cache change. Three defects against draft-24 remain. (1) AS_SET: sections 5.4 and 5.5 step 3 halt with "Invalid"; ze returns Unknown, and three tagged tests in internal/component/bgp/plugins/rpki/aspa_verify_test.go assert the Unknown outcome (row 5.4-2). (2) Row 5.6-2: the default Invalid action is LogOnly, so an ASPA-Invalid route is never made ineligible for route selection. (3) Rows 5-1, 6.2-1 and 6.2-2 carry no test. |
| Support remaining | - |

**Purpose:** Defines a procedure for BGP speakers to verify the AS_PATH attribute of received routes using ASPA (Autonomous System Provider Authorization) objects from RPKI, detecting route leaks and path manipulation.

**Scope:** Upstream path verification only. Applies to routes received from customers and peers. Operates on the full AS_PATH, complementing RFC 6811 origin validation which checks only the origin AS.

## Core Concepts

### ASPA Record

An ASPA object authorizes a set of provider ASes for a given customer AS:

| Field | Type | Description |
|-------|------|-------------|
| Customer AS | uint32 | The AS that issued the authorization |
| Provider AS Set | []uint32 | Set of ASes authorized as upstream providers of the customer |

**Semantics:** "Customer AS X declares that ASes {P1, P2, P3} are its legitimate upstream transit providers."

If an AS has an ASPA record, any AS NOT in its provider set is NOT authorized as its provider. An AS without any ASPA record has unknown provider authorization.

### Hop Verification

For an adjacent pair (AS_a, AS_b) in the AS_PATH where AS_b is the customer (closer to origin):

| Condition | Hop Result |
|-----------|------------|
| AS_b has ASPA record AND AS_a is in AS_b's provider set | Provider+ (authorized) |
| AS_b has ASPA record AND AS_a is NOT in AS_b's provider set | Not Provider+ (unauthorized) |
| AS_b has NO ASPA record | No Attestation (unknown) |

### Validation States

| State | Meaning | Action |
|-------|---------|--------|
| Valid | All hops in the path are authorized (every customer-provider pair is confirmed) | Accept (high confidence) |
| Invalid | At least one hop is provably unauthorized (route leak or path manipulation detected) | Reject or deprioritize |
| Unknown | Some hops cannot be verified (missing ASPA records for some ASes in path) | Accept (insufficient data to judge) |

## Algorithm (Section 5)

The draft verifies the AS_PATH by measuring an up-ramp and a down-ramp of
customer-to-provider hops, not by walking every adjacent pair to a single
verdict.

### COMPRESSED_AS_PATH (Section 5.1)

`COMPRESSED_AS_PATH {AS(N), AS(N-1), ..., AS(2), AS(1)}` is "the AS_PATH after
removing consecutive duplicate ASNs", where AS(1) is the origin AS and AS(N) is
the most recently added neighbor AS. AS(N+1) is the receiving AS and does not
appear in the path.

"The AS_PATH is invalid if apexes of the up-ramp and down-ramp (AS(K) and AS(L),
respectively) of the COMPRESSED_AS_PATH are apart by more than one hop; else, it
is valid."

### Provider authorization function (Section 5.2)

`authorized(AS x, AS y)` answers whether AS y is an attested provider of AS x
per the U-SPAS of AS x:

| Condition | Result |
|-----------|--------|
| No entry in the U-SPAS table for CAS = AS x | `No Attestation` |
| The U-SPAS entry for CAS = AS x includes AS y | `Provider+` |
| Else | `Not Provider+` |

The U-SPAS is the union of the SPAS of every cryptographically valid ASPA the
CAS holds. `No Attestation` "is returned only when no ASPA is retrieved for the
CAS or none of its ASPAs are cryptographically valid".

### Ramp bounds (Section 5.3)

| Parameter | Definition |
|-----------|------------|
| `max_up_ramp` | I, the minimum index for which `authorized(A(I), A(I+1))` returns `Not Provider+`; N if there is no such I |
| `min_up_ramp` | I, the minimum index for which it returns `No Attestation` or `Not Provider+`; N if there is no such I |
| `max_down_ramp` | N - J + 1, where J is the maximum index for which `authorized(A(J), A(J-1))` returns `Not Provider+`; N if there is no such J |
| `min_down_ramp` | N - J + 1, where J is the maximum index for which it returns `No Attestation` or `Not Provider+`; N if there is no such J |

### Upstream paths (Section 5.4)

Applied "when a route is received from a Customer or Peer, or is received by an
RS from an RS-client, or is received by an RS-client from an RS". `max_down_ramp`
and `min_down_ramp` are set to 0.

1. If the AS_PATH is empty, halt with `Invalid`.
2. If the receiving AS is not an RS-client and the most recently added AS in the
   AS_PATH does not match the neighbor AS, halt with `Invalid`.
3. If the AS_PATH has an AS_SET, halt with `Invalid`.
4. If `max_up_ramp < N`, halt with `Invalid`.
5. If `min_up_ramp < N`, halt with `Unknown`.
6. Else, halt with `Valid`.

### Downstream paths (Section 5.5)

Applied "when a route is received from a Provider".

1. If the AS_PATH is empty, halt with `Invalid`.
2. If the most recently added AS in the AS_PATH does not match the neighbor AS,
   halt with `Invalid`.
3. If the AS_PATH has an AS_SET, halt with `Invalid`.
4. If `max_up_ramp + max_down_ramp < N`, halt with `Invalid`.
5. If `min_up_ramp + min_down_ramp < N`, halt with `Unknown`.
6. Else, halt with `Valid`.

**Critical:** compression removes consecutive duplicate ASNs only. [A, A, B, B, B]
becomes [A, B]; [A, B, A] is not reduced.

## MUST Requirements

| Requirement | Section | Context |
|-------------|---------|---------|
| A failed neighbor-AS match makes the AS_PATH semantically invalid and the UPDATE SHALL be treat-as-withdraw | 5 | Leak author stripping its own ASN |
| An AS_SET in the AS_PATH halts the procedure with `Invalid` | 5.4, 5.5 | Step 3 of both algorithms |
| COMPRESSED_AS_PATH removes consecutive duplicate ASNs | 5.1 | Prepend compression |
| An Invalid route MUST be kept in the Adj-RIB-In for future re-evaluation | 5.6 | RFC 9324 |
| The procedures MUST be applied to {AFI 1, SAFI 1} and {AFI 2, SAFI 1} | 6.2 | Address-family scope |
| The procedures MUST NOT be applied to other address families by default | 6.2 | Address-family scope |

The five registration MUSTs of section 4 and the AS-migration notification MUST
of section 6.5 bind the AS registering the ASPA object, not the verifying BGP
speaker.

## SHOULD/MAY

| Type | Requirement | Section | Notes |
|------|-------------|---------|-------|
| [SHOULD] | Perform the neighbor-AS match per RFC 4271 section 6.3, except for a route from a transparent IX | 5 | |
| [SHOULD] | An Invalid route is ineligible for route selection | 5.6 | Within the RECOMMENDED mitigation policy |
| [SHOULD] | An Unknown route is treated at the same preference level as a Valid route | 5.6 | Not a lower preference |
| [SHOULD] | Log the cause of the Invalid state for any route with an Invalid AS_PATH | 6.6 | List the `Not Provider+` hops |
| [SHOULD] | Use the configured BGP Roles to automate upstream/downstream algorithm selection | 6.3 | |
| [SHOULD] | Select the algorithm per session from its peering relation type when a Complex relationship can be segregated | 6.4 | |
| [RECOMMENDED] | The mitigation policy of section 5.6 | 5.6 | Configuration is at operator discretion |
| [RECOMMENDED] | BGP Role configuration and its BGP OPEN cross-check (RFC 9234) | 6.3 | |
| [RECOMMENDED] | Implement the OTC Attribute procedures to complement ASPA | 8.4 | |
| [NOT RECOMMENDED] | Use on iBGP sessions or on eBGP sessions internal to an AS Confederation | 6.2 | |
| [MAY] | Apply the downstream algorithm when a Complex relation cannot be segregated and per-prefix application is not feasible | 6.4 | Avoids false positives |

## Mitigation Policy (Section 5.6)

"The specific configuration of a mitigation policy based on AS_PATH verification
using ASPA is at the discretion of the network operator.  However, the following
mitigation policy is RECOMMENDED."

| ASPA State | Draft's mitigation policy |
|------------|---------------------------|
| Valid | Eligible for route selection |
| Unknown | "SHOULD be treated at the same preference level as a route evaluated as Valid" |
| Invalid | "SHOULD be considered ineligible for route selection" and "MUST be kept in the Adj-RIB-In for potential future re-evaluation" |

ASPA state is orthogonal to ROA state. Both should be evaluated:

| ROA State | ASPA State | Combined Action |
|-----------|------------|-----------------|
| Valid | Valid | Accept (high confidence in origin AND path) |
| Valid | Invalid | Reject (path manipulation despite valid origin) |
| Invalid | Valid | Reject (wrong origin despite valid path) |
| Invalid | Invalid | Reject |
| NotFound | Unknown | Accept (no RPKI data available) |
| Valid | Unknown | Accept (origin verified, path unverifiable) |

## Special Cases

### Empty AS_PATH

Step 1 of both algorithms: "If the AS_PATH is empty, then the procedure halts
with the outcome \"Invalid\"."

### AS_PATH with AS_SET

Step 3 of both algorithms: "If the AS_PATH has an AS_SET, then the procedure
halts with the outcome \"Invalid\"." Section 5 adds that "[RFC9774] specifies
that \"treat-as-withdraw\" error handling [RFC7606] MUST be applied to routes
with AS_SET in the AS_PATH".

### Neighbor AS mismatch

"a check is necessary to match the most recently added AS in the AS_PATH to the
BGP neighbor's ASN". If it fails, "the UPDATE SHALL be handled using the approach
of \"treat-as-withdraw\"". Step 2 of the upstream algorithm exempts a receiving
AS that is an RS-client.

### Confederations

The draft specifies no AS_CONFED stripping in COMPRESSED_AS_PATH. It requires the
ASes on the boundary of a Confederation to register ASPAs under the
Confederation's global ASN (section 4), and it makes the procedures NOT
RECOMMENDED on eBGP sessions internal to a Confederation (section 6.2).

### IPv4 / IPv6 incongruence

There are no per-AFI ASPA records: "The U-SPAS contains the union of Providers
for a CAS for both IPv4 and IPv6 unicast connectivity" (section 7.1). A
relationship present in one family makes verification in the other as permissive.

## Constants

| Name | Value | Usage |
|------|-------|-------|
| ASPA_VALID | - | Path fully verified |
| ASPA_INVALID | - | Unauthorized hop detected |
| ASPA_UNKNOWN | - | Insufficient ASPA coverage |

No IANA registry defined for ASPA validation states.

## Pitfalls

### Edge Cases

- **Prepending vs loops:** [64500, 64500, 64501] normalizes to [64500, 64501]. But [64500, 64501, 64500] does NOT normalize further (not consecutive duplicates). This is a potential loop or unusual topology.
- **Missing ASPA for neighbor:** If the first hop (neighbor AS) has no ASPA record, verification of that hop yields "No Attestation" but verification continues for remaining hops. A single unauthorized hop later still yields Invalid.
- **Partial ASPA deployment:** During early deployment, most ASes lack ASPA records. Result will be Unknown for nearly all paths. This is expected and correct.
- **AS0 in ASPA:** "Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures" (section 4). An ASPA showing only AS 0 as a provider is an AS0 ASPA, a statement that the registering AS has no transit providers.
- **Self-loop in path:** An AS appearing multiple times non-consecutively in the path is unusual but possible (e.g., traffic engineering). Each hop pair is verified independently.

### Interop

- **RTR v2 required:** ASPA records reach routers via RTR v2 (RFC 9582) ASPA PDU (Type 11). Without RTR v2, no ASPA data is available.
- **Complementary to ROA:** ASPA does not replace RFC 6811 origin validation. Both should be deployed together for defense in depth.
- **No BGP wire changes:** ASPA verification is purely local. No new BGP capabilities, attributes, or message types.
- **No propagation:** ASPA validation state is not carried in BGP UPDATE messages. Use communities for IBGP state propagation if needed.

### Security

- **Route leak detection:** Primary use case. Detects when a customer re-announces provider routes to other providers (valley violation).
- **Path shortening attacks:** ASPA cannot detect path shortening (attacker removes intermediate hops). It only verifies that adjacent pairs are authorized.
- **Forged origin with valid path:** If attacker forges origin but constructs a plausible path through authorized providers, ASPA may yield Valid. Combined ROA+ASPA catches this (ROA detects wrong origin).
- **Incomplete ASPA:** A legitimate provider missing from an ASPA record causes its routes through that provider to be Invalid. Operators must maintain complete ASPA records.

## Compatibility

### Interaction with Other Features

| Feature | Interaction |
|---------|-------------|
| RFC 6811 (ROA) | Complementary; ROA validates origin, ASPA validates path |
| RFC 9234 (BGP Role) | Related but independent; BGP Role prevents leaks at the source, ASPA detects them at the receiver |
| RFC 7908 (Route Leak Problem) | ASPA is the verification solution for the problem described in RFC 7908 |
| RFC 8097 (Validation State Communities) | Could be extended for ASPA state propagation in IBGP |

## Related RFCs/Drafts

| Document | Relationship |
|----------|--------------|
| RFC 9582 | RTR v2 protocol delivering ASPA records to routers |
| RFC 6811 | Origin validation (complementary to ASPA path validation) |
| RFC 7908 | Problem definition: BGP route leaks |
| RFC 9234 | BGP Role: preventive mechanism at source (vs ASPA detection at receiver) |
| draft-ietf-sidrops-aspa-profile | ASPA object profile (how ASPAs are signed and published in RPKI) |

## Errata

Not applicable (Internet-Draft, no RFC number assigned).

## Compliance Checklist

- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1] [MUST] An unexpected presence of AS 0 in a SPAS has no influence on the AS_PATH verification procedures (Section 4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1] [SHALL] If the check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN fails, then the AS_PATH is considered semantically invalid, and the UPDATE SHALL be handled using the approach of "treat-as-withdraw" [RFC7606] (Section 5)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1] [MUST] The COMPRESSED_AS_PATH is the AS_PATH after removing consecutive duplicate ASNs (Section 5.1)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1] [MUST] The upstream verification algorithm is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS (Section 5.4) {single-polarity: positive; verifyASPA runs on every received UPDATE carrying an AS_PATH whenever ASPA is enabled (a superset that includes customer and peer routes), and there is no required case where such a route must NOT be verified (internal/component/bgp/plugins/rpki/rpki.go:338)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2] [MUST] If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1] [MUST] A route whose AS_PATH is determined to be Invalid MUST be kept in the Adj-RIB-In for potential future re-evaluation (Section 5.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1] [MUST] The verification procedures described in this document MUST be applied to BGP routes with {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2] [MUST NOT] The procedures MUST NOT be applied to other address families by default (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-2] [SHOULD] The check matching the most recently added AS in the AS_PATH to the BGP neighbor's ASN SHOULD be performed as specified in Section 6.3 of [RFC4271] with the exception when a route is received from a transparent Internet Exchange (IX) (Section 5)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2] [SHOULD] If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.6) {gap: the default Invalid action is LogOnly (retain) and ASPA state drives only a binary reject/keep decision, so an ASPA-Invalid route that is retained stays eligible for best-path selection (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-3] [SHOULD] When a route is evaluated as Unknown, it SHOULD be treated at the same preference level as a route evaluated as Valid (Section 5.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-4] [RECOMMENDED] The specific configuration of a mitigation policy based on AS_PATH verification using ASPA is at the discretion of the network operator; however, the mitigation policy of Section 5.6 is RECOMMENDED (Section 5.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-3] [NOT RECOMMENDED] The procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-1] [RECOMMENDED] The BGP Role configuration parameter and its cross-check in the BGP OPEN message as specified in [RFC9234] are RECOMMENDED (Section 6.3)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-2] [SHOULD] The configured BGP Roles SHOULD be used to automate the use of the AS_PATH verification procedures, helping to distinguish whether upstream or downstream procedures should be applied (Section 6.3)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-1] [SHOULD] If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.4 or Section 5.5) for each of the normal sessions based on its peering relation type (Section 6.4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.6-1] [SHOULD] For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes (Section 6.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8.4-1] [RECOMMENDED] The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification (Section 8.4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-2] [MAY] If a Complex peering relation cannot be segregated and per-prefix application is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.5) to avoid false positive outcomes (Section 6.4)
