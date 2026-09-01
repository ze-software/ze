# draft-ietf-sidrops-8210bis - The RPKI to Router Protocol, Version 2

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-ietf-sidrops-8210bis-27 |
| Title | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 |
| Status | Internet Draft (Standards Track) |
| Date | 2027-02-14 |
| Authors | R. Bush (IIJ Research, Arrcus & DRL), R. Austein (Dragon Research Labs) |
| Obsoletes | RFC 8210 (if approved) |

**Purpose:** Defines version 2 of the RPKI-to-Router protocol, which adds ASPA
(Autonomous System Provider Authorization) PDUs to the ROA prefix and Router Key
payloads version 1 already carries, and fixes the version negotiation a router
and cache perform when they disagree.

**Why this file exists.** Every obligation below was declared under `rfc9582`
until 2026-09-01, and RFC 9582 is a different document: "A Profile for Route
Origin Authorizations (ROAs)", by Snijders, Maddison, Lepinski, Kong and Kent,
which obsoletes RFC 6482. It has no Section 5.12 and its Section 7 is IANA
Considerations, so the thirteen ids cited sections their document does not have
and no reader could check them. The obligations themselves are real and ze meets
them: only the document they were attributed to was wrong. The owner authorized
the re-attribution on 2026-09-01.

## ASPA (Section 5.12)

An ASPA PDU carries a Customer AS, an AFI flag and a list of Provider ASNs. The
router validates the shape before it caches the payload, because a malformed
list is a payload it would otherwise verify AS_PATHs against.

## Protocol Version Negotiation (Section 7)

A router opens with the highest version it supports and downgrades on an
Unsupported Protocol Version error. Ze constructs every session at version 2 and
answers error code 4 by reconnecting at the version the cache named.

## Compliance Checklist

- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-1] [MUST] Provider AS set in ASPA PDU MUST contain at least one provider ASN (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-2] [MUST NOT] Customer AS MUST NOT appear in its own provider set (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-3] [MUST] Provider ASNs MUST be in ascending order within the PDU (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-4] [MUST] Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12) {not-applicable: this is a cache-side emission/deduplication guarantee; ze is the RTR router that only consumes ASPA PDUs (no ASPA PDU writer exists, only query writers) and never enforces or emits this pairing (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, :103)}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-5] [MUST] Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12) {not-applicable: exact-(Customer-AS,AFI) withdraw matching binds the cache's emission and a per-AFI record model; ze consumes withdraws keyed on Customer-AS alone (the §5.12 router option to ignore AFI) and maintains no AFI dimension to match (internal/component/bgp/plugins/rpki/rtr_session.go:283, aspa_cache.go:114)}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-6] [MUST] Router MUST ignore ASPA PDUs with unknown AFI values (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-7] [MUST NOT] Customer AS 0 is reserved, MUST NOT appear (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-1] [MUST] Router starting a v2 session MUST send query with version=2 (§7) {single-polarity: positive; ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-2] [MUST] On Unsupported Protocol Version error, router MUST downgrade or disconnect (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-3] [MUST] Cache receiving a version it does not support MUST send error code 4 (§7) {not-applicable: this binds the RTR cache/server role; ze runs only an RTR client that dials out and reads error reports, with no listener and no error-report writer, so it never receives queries or sends error code 4 (internal/component/bgp/plugins/rpki/rtr_session.go:125)}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-4] [SHOULD] Router SHOULD start at highest supported version (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-8] [SHOULD] Provider ASNs SHOULD be sorted ascending; cache MUST sort, router SHOULD verify (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-9] [MAY] Router MAY ignore AFI field and apply ASPA to all address families (§5.12)
