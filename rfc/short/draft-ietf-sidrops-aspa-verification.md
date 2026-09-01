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
| Enrolment reason | ASPA AS_PATH verification: 4 MET + 2 single-polarity (6-1, 7-2) + 2 gap (6-4 per-AFI records, 8-1 Invalid-not-preferred) |
| Support | drafts 20 |
| Support area | ASPA path verification |
| Support status | Partial |
| Support coverage | Section 6 verification algorithm (upstream/downstream), AS_SET to Unknown, prepend collapse, AS0-in-provider rejection, RTR ASPA PDU (Type 11) consumption, and re-validation on cache change. Two MUSTs unmet: per-AFI ASPA records (6-4, the AFI flag is parsed then discarded and the cache is keyed by customer AS alone, so per-AFI records overwrite each other); Invalid-not-preferred (8-1, ASPA state drives only reject/keep with default LogOnly, so an accepted Invalid route can outrank a Valid one for the same prefix). |
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

## Algorithm

### Upstream Path Verification (Section 6)

This algorithm verifies routes received from a customer or lateral peer. It checks that the AS_PATH represents a valid "valley-free" forwarding path from the perspective of provider authorization.

```
Input:  as_path (list of AS hops from receiver toward origin),
        aspa_database (Customer-AS -> Provider-AS-Set mapping),
        neighbor_as (the AS that sent this route)
Output: validation_state (Valid, Invalid, Unknown)

// Step 0: Normalize AS_PATH
//   - Remove prepends (consecutive duplicate ASNs)
//   - If any segment is AS_SET or AS_CONFED_SET: return Unknown
//   - Collapse AS_CONFED_SEQUENCE segments (remove confederation members)
//   - Result: unique_path = [AS_n, AS_n-1, ..., AS_1] where AS_n is neighbor, AS_1 is origin

unique_path = normalize(as_path)
if unique_path contains AS_SET:
    return Unknown

if len(unique_path) <= 1:
    return Valid  // Single-hop path, nothing to verify

// Step 1: Upstream check (from receiver toward origin)
// Walk from index 0 (neighbor) toward origin.
// For each adjacent pair (unique_path[i], unique_path[i+1]):
//   unique_path[i+1] is the "customer" (closer to origin)
//   unique_path[i] is the potential "provider" (closer to receiver)

for i := 0; i < len(unique_path) - 1; i++:
    customer_as = unique_path[i+1]
    provider_candidate = unique_path[i]

    hop_result = check_pair(provider_candidate, customer_as, aspa_database)

    if hop_result == "Not Provider+":
        return Invalid

    // If hop_result is "No Attestation" or "Provider+", continue

// Step 2: Determine final state
// If all hops were "Provider+": Valid
// If any hop was "No Attestation" (but none "Not Provider+"): Unknown

has_unknown = false
for i := 0; i < len(unique_path) - 1; i++:
    customer_as = unique_path[i+1]
    provider_candidate = unique_path[i]
    if not has_aspa_record(customer_as, aspa_database):
        has_unknown = true

if has_unknown:
    return Unknown
return Valid
```

### check_pair Function

```
Input:  provider_candidate (uint32), customer_as (uint32), aspa_db
Output: "Provider+" | "Not Provider+" | "No Attestation"

record = aspa_db.lookup(customer_as)
if record == nil:
    return "No Attestation"

if provider_candidate in record.provider_set:
    return "Provider+"

return "Not Provider+"
```

### AS_PATH Normalization

```
Input:  raw AS_PATH segments
Output: normalized unique hop list, or "contains AS_SET" flag

1. Flatten AS_SEQUENCE segments into a single list
2. If any AS_SET or AS_CONFED_SET segment exists: flag as "contains AS_SET"
3. Remove AS_CONFED_SEQUENCE members (confederation-internal hops)
4. Remove consecutive duplicates (prepending artifacts)
5. Result: ordered list of unique ASNs from neighbor (leftmost) to origin (rightmost)
```

**Critical:** Prepend removal is consecutive-duplicate only. The sequence [A, B, A] is NOT reduced to [A, B]; only [A, A, B, B, B] becomes [A, B].

## MUST Requirements

### Verification Procedure

| Requirement | Section | Context |
|-------------|---------|---------|
| Implementation MUST apply upstream verification to routes from customers and lateral peers | 6 | Scope of verification |
| AS_SET in path MUST result in Unknown state | 6 | Cannot verify unordered sets |
| Prepend removal MUST only collapse consecutive duplicates | 6 | Preserve path structure |
| Implementation MUST support per-AFI ASPA records if provided by cache | 6 | AFI-specific authorization |
| Invalid routes MUST NOT be preferred over Valid or Unknown routes | 8 | Policy integration |

### Data Handling

| Requirement | Section | Context |
|-------------|---------|---------|
| Router MUST re-run verification when ASPA data changes | 7 | Cache update triggers re-validation |
| Router MUST use the most recent ASPA data available | 7 | No stale cache usage |

## SHOULD/MAY

| Type | Requirement | Section | Notes |
|------|-------------|---------|-------|
| [SHOULD] | Reject Invalid routes by default | 8 | Strictest useful policy |
| [SHOULD] | Accept Unknown routes (treat as unverified) | 8 | Graceful during ASPA deployment |
| [SHOULD] | Log Invalid results for operational visibility | 8 | Debugging route leaks |
| [MAY] | Assign local-pref based on ASPA state | 8 | Prefer Valid over Unknown |
| [MAY] | Skip verification for routes from upstream providers | 6 | Upstream routes have different trust model |
| [MAY] | Apply to IBGP-learned routes | 6 | Optional; usually applied at eBGP ingress only |

## Policy Integration (Section 8)

| ASPA State | Recommended Action |
|------------|-------------------|
| Valid | Accept, highest preference |
| Unknown | Accept, normal preference (insufficient coverage to judge) |
| Invalid | Reject, or accept with lowest preference |

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

### Single-Hop AS_PATH

A path with only one AS (the origin, which is also the neighbor) is trivially Valid. No adjacent pairs to check.

### Empty AS_PATH

An empty AS_PATH means the route is from an IBGP peer or was originated locally. Result: Valid (nothing to verify).

### AS_PATH with AS_SET

Any AS_SET segment makes the entire path unverifiable. Result: Unknown.

Rationale: AS_SET represents route aggregation where the original paths are collapsed into an unordered set. The customer-provider relationships between set members cannot be determined.

### Confederation AS_PATH

AS_CONFED_SEQUENCE and AS_CONFED_SET segments represent internal confederation structure. They are stripped during normalization (only the confederation's external ASN matters for inter-domain verification).

### Route Server Transparent AS_PATH

Route servers (RFC 7947) that use transparent mode do not insert their own ASN. The AS_PATH seen by the receiver starts with the originating network's peer, not the route server. Verification proceeds normally on the visible path.

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
- **AS0 in ASPA:** A provider AS of 0 in an ASPA record is invalid and MUST be ignored. AS0 means "not authorized" in ROA context; it has no meaning in ASPA.
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

- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-1] [MUST] Apply upstream verification to routes received from customers and lateral peers (Section 6) {single-polarity: positive; verifyASPA runs on every received UPDATE carrying an AS_PATH whenever ASPA is enabled (a superset that includes customer and peer routes), and there is no required case where such a route must NOT be verified (internal/component/bgp/plugins/rpki/rpki.go:338)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-2] [MUST] AS_SET in path must result in Unknown validation state (Section 6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-3] [MUST] Prepend removal must only collapse consecutive duplicates (Section 6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-4] [MUST] Support per-AFI ASPA records if provided by cache (Section 6) {gap: the ASPA PDU AFI-flags byte is read only to reject unknown AFI and is then discarded; ASPARecord carries no AFI field and the cache is keyed by customer AS alone, so per-AFI records for one customer overwrite each other and one AS_PATH state is applied to both IPv4 and IPv6 NLRIs (internal/component/bgp/plugins/rpki/aspa_cache.go:10-13, rpki.go:344)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-x-1] [MUST] AS0 in ASPA provider set must be ignored (Pitfalls)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-1] [MUST NOT] Invalid routes must not be preferred over Valid or Unknown routes (Section 8) {gap: ASPA state drives only a binary reject/keep decision with no local-pref demotion or best-path tiebreak, and the default Invalid action is LogOnly (retain), so an accepted ASPA-Invalid route competes on ordinary BGP attributes and can outrank an ASPA-Valid route for the same prefix (internal/component/bgp/plugins/rpki/rpki.go:92-100, rpki_config.go:110)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1] [MUST] Re-run verification when ASPA data changes (Section 7)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-2] [MUST] Use the most recent ASPA data available (Section 7) {single-polarity: positive; ApplyDelta atomically replaces cache entries at each End of Data and verifyASPA reads the live cache under lock, so verification always reflects the applied delta; there is no stale-data mode to test negatively (internal/component/bgp/plugins/rpki/aspa_cache.go:110-129)}
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-2] [SHOULD] Reject Invalid routes by default (Section 8)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-3] [SHOULD] Accept Unknown routes as unverified (Section 8)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-4] [SHOULD] Log Invalid results for operational visibility (Section 8)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8-5] [MAY] Assign local-pref based on ASPA validation state (Section 8)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-5] [MAY] Skip verification for routes from upstream providers (Section 6)
- [ ] [DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6-6] [MAY] Apply verification to IBGP-learned routes (Section 6)
