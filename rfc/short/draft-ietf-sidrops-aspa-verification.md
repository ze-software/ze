# draft-ietf-sidrops-aspa-verification-28: BGP AS_PATH Verification Based on ASPA Objects

## Meta

| Field | Value |
|-------|-------|
| Document | draft-ietf-sidrops-aspa-verification-28 |
| Title | BGP AS_PATH Verification Based on Autonomous System Provider Authorization (ASPA) Objects |
| Status | Internet-Draft (Standards Track) |
| Date | 24 August 2026 |
| Authors | A. Azimov (Yandex), E. Bogomazov (Qrator Labs), R. Bush (IIJ & Arrcus), K. Patel (Arrcus), J. Snijders (BSD), K. Sriram (USA NIST) |
| RFC Number | The cached source is an Internet-Draft |
| Enrolment | enrolled |
| Enrolment reason | Router-verification requirements follow the cached revision 28. ASPA registration and AS-migration duties remain separate from router verification. |
| Implementation | ze |
| Implementation reason | Ze's RPKI plugin implements the verification and policy decisions; Adj-RIB-In and the selecting RIB apply route eligibility. The producers are `aspa_verify.go::aspaStateForPath`, `rpki.go::buildDecisions`, `adj_rib_in/rib_validation.go::applyDecision` and `rib/rib_validation.go::validationChanged`. |
| Support | drafts 20 |
| Support area | ASPA path verification |
| Support status | Partial |
| Support coverage | Sections 5.5 and 5.6 upstream/downstream verification, consecutive-prepend compression, Empty/AS_SET Invalid outcomes, IPv4/IPv6 unicast scope, role-based algorithm selection, and cache-change re-evaluation. `aspa_verify.go` produces path verdicts; `rpki_config.go` selects the algorithm and defaults Invalid policy to reject; `rpki.go` combines origin and path policy and dispatches eligibility decisions. This describes the source, not a fresh test or discrimination result. Section 5.1 prerequisite-order coverage and proof for the restored permanent IDs remain unverified. Extraction and discrimination provenance migration remains pending; the former neighbor-check SHOULD and mismatch SHALL have unresolved source-level identities in revision 28. Current validation and discrimination of the implemented paths, Section 5.1 prerequisite ordering, and final migration of the retired revision-27 source identities remain unresolved. The two retired IDs are reserved, not reassigned to different obligations. |
| Support remaining | - |

**Primary:** https://www.ietf.org/archive/id/draft-ietf-sidrops-aspa-verification-28.txt,
cached in `rfc/drafts/draft-ietf-sidrops-aspa-verification.txt`.
The associated `rfc/extraction/draft-ietf-sidrops-aspa-verification.json` still
records the earlier source walk. The discrimination records in
`rfc/discrimination/draft-ietf-sidrops-aspa-verification.json` require separate
native observation against the current producers and tags. Updated citations
and preserved test assertions do not establish a new execution result.

**Purpose:** Defines a procedure for BGP speakers to verify the AS_PATH attribute of received routes using ASPA (Autonomous System Provider Authorization) objects from RPKI, detecting route leaks and path manipulation.

**Scope:** AS_PATH verification on ingress eBGP routes, with upstream or
downstream verification selected from the local peering relationship. Section
6.2 requires IPv4 and IPv6 unicast and forbids application to other families by
default. The procedure complements RFC 6811 origin validation.

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

| State | Meaning | Section 5.7 recommended policy |
|-------|---------|--------------------------------|
| Valid | The selected algorithm's minimum ramp bounds establish a valid path | Eligible for route selection |
| Invalid | A structural check fails or the maximum ramp bounds cannot establish a valid path | Retain in Adj-RIB-In and make ineligible |
| Unknown | The maximum bounds permit a valid path but the minimum bounds leave insufficient attestation | Same preference as Valid |

## Algorithm (Section 5)

The draft verifies the AS_PATH by measuring customer-to-provider ramps. For
upstream paths, every hop belongs to the up-ramp. For downstream paths, the
up-ramp starts at the origin and the down-ramp starts at the neighbor towards
the path's apex; their bounds can meet across one unverified apex hop.

### Prerequisite checks (Section 5.1)

The draft attributes AS_SET treat-as-withdraw handling to RFC 9774 and the
neighbor-AS check to RFC 4271, with RFC 7606 error handling. It then requires:

> If the aforementioned AS_PATH checks and error handling are implemented,
> they MUST be applied prior to ASPA verification.

`reactor/session_read.go::processMessage` runs RFC 7606 validation, reconstructs
the AS_PATH, and calls `session_validation.go::firstASMismatch` before delivering
the semantic UPDATE to consumers. The existing first-AS receive tests remain;
they are not relabeled as proof of this newly explicit ordering requirement.
The entrypoint coverage of Section 5.1 remains unverified.

### COMPRESSED_AS_PATH (Section 5.2)

`COMPRESSED_AS_PATH {AS(N), AS(N-1), ..., AS(2), AS(1)}` is "the AS_PATH after
removing consecutive duplicate ASNs", where AS(1) is the origin AS and AS(N) is
the most recently added neighbor AS. AS(N+1) is the receiving AS and does not
appear in the path.

"The AS_PATH is invalid if apexes of the up-ramp and down-ramp (AS(K) and AS(L),
respectively) of the COMPRESSED_AS_PATH are apart by more than one hop; else, it
is valid."

### Provider authorization function (Section 5.3)

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

### Ramp bounds (Section 5.4)

| Parameter | Definition |
|-----------|------------|
| `max_up_ramp` | I, the minimum index for which `authorized(A(I), A(I+1))` returns `Not Provider+`; N if there is no such I |
| `min_up_ramp` | I, the minimum index for which it returns `No Attestation` or `Not Provider+`; N if there is no such I |
| `max_down_ramp` | N - J + 1, where J is the maximum index for which `authorized(A(J), A(J-1))` returns `Not Provider+`; N if there is no such J |
| `min_down_ramp` | N - J + 1, where J is the maximum index for which it returns `No Attestation` or `Not Provider+`; N if there is no such J |

### Upstream paths (Section 5.5)

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

### Downstream paths (Section 5.6)

Applied "when a route is received from a Provider". The receiving AS therefore
has the local Customer role. AS(N+1), the receiving AS, is outside the
COMPRESSED_AS_PATH; the down-ramp measured by Section 5.4 starts at AS(N).

1. If the AS_PATH is empty, halt with `Invalid`.
2. If the most recently added AS in the AS_PATH does not match the neighbor AS,
   halt with `Invalid`.
3. If the AS_PATH has an AS_SET, halt with `Invalid`.
4. If `max_up_ramp + max_down_ramp < N`, halt with `Invalid`.
5. If `min_up_ramp + min_down_ramp < N`, halt with `Unknown`.
6. Else, halt with `Valid`.

Ze reads this local relationship from `role/import` in
`rpki_config.go::configuredASPAMode`: `customer` selects downstream;
`provider`, `peer`, `rs`, and `rs-client` select upstream. With no configured
role, `aspa_verify.go::verifyASPAPath` returns Unknown for a non-empty ordered
path. Empty paths and AS_SET are Invalid before role selection.

**Critical:** compression removes consecutive duplicate ASNs only. [A, A, B, B, B]
becomes [A, B]; [A, B, A] is not reduced.

## MUST Requirements

| Requirement | Section | Context |
|-------------|---------|---------|
| If the prerequisite AS_PATH checks and error handling are implemented, they MUST precede ASPA verification | 5.1 | Conditional ordering obligation |
| An unexpected presence of AS 0 in a SPAS has no influence on path verification | 4 | Indicative verification rule |
| COMPRESSED_AS_PATH removes consecutive duplicate ASNs | 5.2 | Algorithm definition |
| The upstream algorithm applies to routes from Customers, Peers, RS-clients at an RS, and an RS at an RS-client | 5.5 | Algorithm selection |
| An AS_SET halts verification with `Invalid` | 5.5, 5.6 | Step 3 of both algorithms |
| An Invalid route MUST be kept in the Adj-RIB-In for future re-evaluation | 5.7 | RFC 9324 |
| The procedures MUST be applied to {AFI 1, SAFI 1} and {AFI 2, SAFI 1} | 6.2 | Address-family scope |
| The procedures MUST NOT be applied to other address families by default | 6.2 | Address-family scope |

The registration obligations of section 4 and the AS-migration notification
MUST of section 6.5 bind the ASPA registrant or AS operator. Revision 28 removes
revision 27's sentence requiring an AS without transit providers to register an
AS0 ASPA; its description of what an AS0 ASPA means remains.

## SHOULD/MAY

| Type | Requirement | Section | Notes |
|------|-------------|---------|-------|
| [SHOULD] | An Invalid route is ineligible for route selection | 5.7 | Within the RECOMMENDED mitigation policy |
| [SHOULD] | An Unknown route is treated at the same preference level as a Valid route | 5.7 | Not a lower preference |
| [SHOULD] | Log the cause of the Invalid state for any route with an Invalid AS_PATH | 6.6 | List the `Not Provider+` hops |
| [SHOULD] | Use the configured BGP Roles to automate upstream/downstream algorithm selection | 6.3 | |
| [SHOULD] | Select the algorithm per session from its peering relation type when a Complex relationship can be segregated | 6.4 | |
| [RECOMMENDED] | The mitigation policy of section 5.7 | 5.7 | Configuration is at operator discretion |
| [RECOMMENDED] | BGP Role configuration and its BGP OPEN cross-check (RFC 9234) | 6.3 | |
| [RECOMMENDED] | Implement the OTC Attribute procedures to complement ASPA | 8.4 | |
| [NOT RECOMMENDED] | Use on iBGP sessions or on eBGP sessions internal to an AS Confederation | 6.2 | |
| [MAY] | Apply the downstream algorithm when a Complex relation cannot be segregated and per-prefix application is not feasible | 6.4 | Avoids false positives |

## Mitigation Policy (Section 5.7)

"The specific configuration of a mitigation policy based on AS_PATH verification
using ASPA is at the discretion of the network operator.  However, the following
mitigation policy is RECOMMENDED."

| ASPA State | Draft's mitigation policy |
|------------|---------------------------|
| Valid | Eligible for route selection |
| Unknown | "SHOULD be treated at the same preference level as a route evaluated as Valid" |
| Invalid | "SHOULD be considered ineligible for route selection" and "MUST be kept in the Adj-RIB-In for potential future re-evaluation" |

Ze's default Invalid action is `reject`, while Unknown defaults to `accept`.
The explicit `accept` and `log-only` actions remain operator choices, as
Section 5.7 permits. `rpki_config.go::parseRPKIConfig` supplies the defaults and
`rpki.go::buildDecisions` applies the resolved peer, group, or global policy.
For an ASPA-evaluated route rejected by policy, the decision carries
`Ineligible=true` so the received bytes remain available for re-evaluation.

Origin validation and ASPA are independent verdicts. Either configured reject
action can make the route ineligible. `rpki.go::handleASPAChange` dispatches
every changed ASPA state with the current origin verdict and UPDATE generation,
so repaired ASPA data cannot bypass an origin-policy rejection.

`adj_rib_in/rib_validation.go::applyDecision` retains an ineligible route and
allows a later accepting decision for that UPDATE generation to restore its
eligibility. `rib/rib_validation.go::validationChanged` reruns selection and
Loc-RIB publication without a new UPDATE. The source tests
`TestASPAUpstreamRoles`, `TestASPARecoveryKeepsCurrentOriginVerdict`,
`TestASPARetentionReceiveRecovery` and `TestASPARetentionReplacementWithdrawal`
exercise these role and retention boundaries; no new execution result is
asserted by this summary.

## Special Cases

### Empty AS_PATH

Step 1 of both algorithms: "If the AS_PATH is empty, then the procedure halts
with the outcome \"Invalid\"."

### AS_PATH with AS_SET

Step 3 of both algorithms: "If the AS_PATH has an AS_SET, then the procedure
halts with the outcome \"Invalid\"." Section 5.1 attributes the earlier
treat-as-withdraw obligation to RFC 9774. Without those prerequisite checks,
ASPA verification evaluates an AS_SET path as Invalid.

### Neighbor AS mismatch

Section 5.1 describes the neighbor-AS check and its transparent route-server
exception. The error handling is attributed to RFC 4271 and RFC 7606; revision
28 does not repeat revision 27's separate SHALL sentence. Step 2 of upstream
verification exempts a receiving AS that is an RS-client.

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
- **Missing attestations:** An upstream path remains Unknown if a hop lacks an attestation and no hop is denied. Downstream paths use both ramp bounds; an unverified apex hop can still yield Valid.
- **Partial ASPA deployment:** Missing data can yield Unknown, but it does not override an Invalid structural check or a conclusive ramp failure.
- **AS0 in ASPA:** "Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures" (Section 4). A singleton AS0 ASPA states that the AS has no transit providers and is not an RS-client at a non-transparent RS. RTR revision 27 separately rejects a mixed-AS0 announcement with Error Report 9.
- **Non-consecutive repeats:** Compression preserves them. Verification still applies the relationship-specific ramp procedure.

### Interop

- **RTR v2:** `draft-ietf-sidrops-8210bis-27` carries ASPA records in PDU Type 11. The PDU has no AFI field. RFC 9582 specifies the ROA signed-object profile.
- **Complementary to ROA:** ASPA does not replace RFC 6811 origin validation. Both should be deployed together for defense in depth.
- **No BGP wire changes:** ASPA verification is purely local. No new BGP capabilities, attributes, or message types.
- **No propagation:** ASPA validation state is not carried in BGP UPDATE messages. Use communities for IBGP state propagation if needed.

### Security

- **Route leak detection:** Primary use case. Detects when a customer re-announces provider routes to other providers (valley violation).
- **Provider manipulation:** Section 7.3 warns that a provider's forged-origin or forged-segment paths, or manipulation of paths sent to its customers, may evade ASPA detection.
- **Prepend manipulation:** Section 7.4 says ASPA cannot detect removal or addition of repeated ASNs, but that change alone does not affect route-leak detection.
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
| draft-ietf-sidrops-8210bis | RTR v2 protocol delivering ASPA records to routers |
| RFC 9582 | ROA signed-object profile used by origin validation |
| RFC 6811 | Origin validation (complementary to ASPA path validation) |
| RFC 7908 | Problem definition: BGP route leaks |
| RFC 9234 | BGP Role: preventive mechanism at source (vs ASPA detection at receiver) |
| draft-ietf-sidrops-aspa-profile | ASPA object profile (how ASPAs are signed and published in RPKI) |

## Errata

Not applicable (Internet-Draft, no RFC number assigned).

## Permanent-ID Migration

The checklist keeps allocated IDs when their source section moves. The
compression definition now lives in section 5.2, upstream algorithm selection
in section 5.5, AS_SET handling in sections 5.5 and 5.6, and mitigation policy
in section 5.7. The restored indicative rules retain their existing gated
levels; these levels record algorithm obligations, rather than quoted
uppercase keywords in the draft.

Two allocated IDs have unresolved identities because revision 28 changed the
source obligations. They remain reserved and cannot be reused:

| Permanent ID | Source conflict |
|--------------|-----------------|
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-1` | Revision 27 section 5 said a failed neighbor-AS check made the UPDATE subject to a SHALL treat-as-withdraw directive. Revision 28 section 5.1 instead attributes the check and handling to RFC 4271 and RFC 7606, and explicitly describes ASPA Invalid when the checks are not enforced. The former unconditional SHALL cannot be restated as the new conditional ordering MUST. |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5-2` | Revision 27 section 5 said the neighbor-AS check SHOULD be performed, except for a route from a transparent IX. Revision 28 section 5.1 describes the necessary check and transparent route-server exception, but removes that SHOULD sentence and attributes the check to RFC 4271. Its former normative level has no direct counterpart in the current draft. |

Neither ID is assigned a substitute obligation in the checklist. The
canonical revision-28 source is
`rfc/drafts/draft-ietf-sidrops-aspa-verification.txt`; the current source
mapping is `rfc/extraction/draft-ietf-sidrops-aspa-verification.json`.
Historical migration records do not provide current verification.
The new ordering requirement keeps its distinct ID `5.1-2`.

## Compliance Checklist

- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1] [MUST] An unexpected presence of AS 0 in a SPAS has no influence on the AS_PATH verification procedures (Section 4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2] [MUST] If the prerequisite AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification (Section 5.1) {gap: entrypoint coverage for this newly explicit ordering obligation is unverified; Session.processMessage applies existing checks before semantic UPDATE delivery, but the prior neighbor-AS tests have not been established as ordering proof}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1] [MUST] The COMPRESSED_AS_PATH is the AS_PATH after removing consecutive duplicate ASNs (Section 5.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1] [MUST] The upstream verification algorithm is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS (Section 5.5)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2] [MUST] If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.5; Section 5.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1] [MUST] A route whose AS_PATH is determined to be Invalid MUST be kept in the Adj-RIB-In for potential future re-evaluation (Section 5.7)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1] [MUST] The verification procedures described in this document MUST be applied to BGP routes with {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2] [MUST NOT] The procedures MUST NOT be applied to other address families by default (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2] [SHOULD] If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.7)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-3] [SHOULD] When a route is evaluated as Unknown, it SHOULD be treated at the same preference level as a route evaluated as Valid (Section 5.7)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-4] [RECOMMENDED] The specific configuration of a mitigation policy based on AS_PATH verification using ASPA is at the discretion of the network operator; however, the mitigation policy of Section 5.7 is RECOMMENDED (Section 5.7)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-3] [NOT RECOMMENDED] The procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation (Section 6.2)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-1] [RECOMMENDED] The BGP Role configuration parameter and its cross-check in the BGP OPEN message as specified in [RFC9234] are RECOMMENDED (Section 6.3)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-2] [SHOULD] The configured BGP Roles SHOULD be used to automate the use of the AS_PATH verification procedures, helping to distinguish whether upstream or downstream procedures should be applied (Section 6.3)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-1] [SHOULD] If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.5 or Section 5.6) for each of the normal sessions based on its peering relation type (Section 6.4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.6-1] [SHOULD] For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes (Section 6.6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8.4-1] [RECOMMENDED] The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification (Section 8.4)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-2] [MAY] If a Complex peering relation cannot be segregated and per-prefix application is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.6) to avoid false positive outcomes (Section 6.4)
