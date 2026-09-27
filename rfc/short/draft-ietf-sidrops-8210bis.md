# draft-ietf-sidrops-8210bis - The RPKI to Router Protocol, Version 2

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-ietf-sidrops-8210bis-27 |
| Title | The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 |
| Status | Internet Draft (Standards Track) |
| Date | 13 August 2026 |
| Authors | R. Bush (Arrcus, DRL & IIJ Research), R. Austein (Dragon Research Labs), T. Harrison (APNIC) |
| Obsoletes | RFC 8210 (if approved) |
| Enrolment | backlog |
| Enrolment reason | Existing RTR v2 ledger awaiting requirement reattribution from the historical RFC 9582 identifiers. The cached authority is revision 27; proof relocation and enrolment must preserve the source requirements and their tests. |
| Implementation | ze |
| Implementation reason | Ze's RTR client decodes ASPA with `rtr_pdu.go::parseASPAPDU`, sends provider-list errors through `rtr_session.go::readLoop`, and applies received records with `RTRSession.handlePDU`. The cache's object validation and union construction are separate from these router operations. |
| Support | - |

**Purpose:** Defines version 2 of the RPKI-to-Router protocol, which adds ASPA
(Autonomous System Provider Authorization) PDUs to the ROA prefix and Router Key
payloads version 1 already carries, and fixes the version negotiation a router
and cache perform when they disagree.

The cached source is `rfc/drafts/draft-ietf-sidrops-8210bis.txt`. The earlier
summary copied thirteen RTR rows attributed to RFC 9582, which specifies the
ROA signed-object profile. Several copied rows also misstate the draft itself.
The correspondence below distinguishes a sourced correction from a claim with
no current clause. It does not claim that proof has already moved.

## ASPA (Section 5.12)

An ASPA PDU carries one Customer AS and its complete provider set. It has no
AFI field. Section 5.12 Figure 11 defines these byte offsets:

```
  byte       0          1          2          3
       +----------+----------+----------+----------+
       |Version=2 | Type=11  |  Flags   |   zero   |
       +----------+----------+----------+----------+
   4   |                 Length                  |
       +-----------------------------------------+
   8   |               Customer AS               |
       +-----------------------------------------+
  12   |          Provider AS numbers ...        |
       +-----------------------------------------+
```

| Field | Offset | Length | Meaning |
|-------|--------|--------|---------|
| Version / Type | 0 / 1 | 1 byte each | 2 / 11 |
| Flags | 2 | 1 byte | Bit 0: announce or replace = 1; withdraw = 0 |
| Reserved zero | 3 | 1 byte | Section 5 requires zero on transmission and ignoring on receipt |
| Length | 4 | 4 bytes | Entire PDU: 12 + 4*N |
| Customer AS | 8 | 4 bytes | Customer whose provider set is replaced or removed |
| Provider AS numbers | 12 | 4*N bytes | Increasing numeric order; each ASN unique |

Section 5.12 states:

> Each Provider Autonomous System Number in a given ASPA PDU MUST be unique.

> For an announcement, the PDU MUST contain at least one Provider Autonomous
> System Number.

An empty announcement requires Error Report 9. A singleton provider AS 0 is
permitted. The distinct mixed-provider constraint is:

> Also, an ASPA announcement PDU containing multiple Provider Autonomous
> System Numbers MUST NOT contain AS 0.

That violation also requires Error Report 9. The announcement minimum is 16
bytes. A withdrawal has no provider list and its length is exactly 12 bytes:

> A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove
> the entire ASPA record from that cache for that Customer AS.

At most one ASPA for each Customer AS is active from a particular cache. A new
announcement for the same customer replaces the old provider set; the cache
delivers the complete union in one PDU.

`internal/component/bgp/plugins/rpki/rtr_pdu.go::parseASPAPDU` reads these
offsets and distinguishes a singleton AS0 from a mixed list.
`rtr_session.go::readLoop` sends Error Report 9 for the provider-list error.
`RTRSession.handlePDU` applies records at End of Data through
`aSPACache.ApplyDelta` or `aSPACache.Replace`.

## Protocol Version Negotiation (Section 7)

Section 7 requires the router's first Reset Query or Serial Query to use the
highest protocol version it implements:

> Once a router has established a transport connection to a cache, it MUST
> attempt to open an RPKI-Router 'session' by issuing either a Reset Query
> Section 5.4) or a Serial Query (Section 5.3) with the highest version of this
> protocol the router implements in the Protocol Version field.

For an unsupported query above the cache's highest version C, the cache sends
Error Report 4 with version C. The router SHOULD retry at C unless that version
already failed. If a version-0 query still gets Error Report 4, the router MUST
abort the transport. During negotiation an unrecognized version requires
downgrading to a known version or terminating, with Error Report 4 unless the
received PDU is itself an Error Report.

The negotiated version remains fixed for the session. ASPA is available at
version 2. `rtr_session.go::newRTRSession` starts at `rtrVersionMax`, and
`RTRSession.connectAndSync` writes the current version in its query.
Ze supports RTR versions 1 and 2 (`rtrVersionMin` and `rtrVersionMax`).
`RTRSession.handlePDU` uses the cache's advertised lower supported version for
a retry; an unsupported or already-rejected version returns an error and
`RTRSession.syncOnce` closes the transport. This describes no version-0
negotiation support.

The retry and repeated-version rejection clauses have SHOULD strength:

> The router SHOULD send another query with a Protocol Version Q with Q ==
> the version C in the Error Report PDU unless it has already failed at that
> version, which indicates a fatal error in programming of the cache which
> SHOULD result in transport termination.

Tests of that retry cannot prove the distinct MUST for an unrecognized
protocol version, quoted in the checklist.

## Compliance Checklist

- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-1] [MUST] For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-3] [MUST] There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-4] [MUST] The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-5] [MUST] If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-1] [MUST] Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) {single-polarity: positive; ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-2] [MUST] If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU with Error Code 4 ("Unsupported Protocol Version") unless the received PDU is itself an Error Report PDU. (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-3] [MUST] If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-4] [MUST] Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

## Historical Requirement Correspondence

`RFC9582-` below identifies the historical rows of `rfc9582.md`. The
destination prefix is `DRAFT-IETF-SIDROPS-8210BIS-`. Every row with a
destination was retired from `rfc9582.md` on 2026-09-26, naming it in
`rfc/corrections/rfc9582.md`, and its tags carry the destination id. The four
rows with no destination (`5.12-2`, `5.12-6`, `5.12-7`, `5.12-9`) were retired
from `rfc9582.md` on 2026-09-27, under owner decisions D-10 and D-2, as
`rfc/corrections/rfc9582.md` records.

| Historical suffix | Current clause or destination | Correction |
|-------------------|-------------------------------|------------|
| `5.12-1` | `5.12-1` | Applies to announcements; a withdrawal has no providers |
| `5.12-2` | No matching prohibition in Section 5.12 | The decoder rejects self-providers, but that policy is not stated by this RTR clause |
| `5.12-3` | `5.12-3` | Increasing order and uniqueness |
| `5.12-4` | `5.12-4` | One active Customer AS per cache; no AFI key |
| `5.12-5` | `5.12-5` | Entire Customer AS record withdrawal; no AFI key |
| `5.12-6` | No corresponding field or requirement | Revision 27 has no AFI field |
| `5.12-7` | No matching prohibition in Section 5.12 | Reserved-customer rejection is a decoder check, not the mixed-provider AS0 prohibition |
| `5.12-8` | `5.12-3` | Duplicates the ordering claim; no separate router SHOULD-sort clause |
| `5.12-9` | No corresponding permission | No AFI field exists to ignore |
| `7-1` | `7-1`, read with `7-4` | Opening query uses the highest implemented version, currently 2 |
| `7-2` | `7-2` | Unknown-version receive assertions must exercise the unrecognized-version clause; retry at cache version C and termination after repeated rejection are separately SHOULD |
| `7-3` | `7-3` | Preserve the cache's unsupported-query conditions |
| `7-4` | `7-4`, same source sentence as `7-1` | Highest-version opening is MUST |

The reserved-byte receive assertion belongs to Section 5:

> Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission
> and MUST be ignored on receipt.

It cannot inherit the identity of an invented AFI requirement. Likewise, the
mixed-AS0 Error Report 9 obligation needs its own sourced identity; neither the
self-provider nor customer-AS0 row states it.

The following two copied checklist rows are historical claims only. Their
current-draft defects are recorded above. This draft does not state either
obligation; draft-ietf-sidrops-aspa-profile does (§3.3 for the self-provider
prohibition, §3.2 for a positive customerASID), and it has no summary yet, so
both rows stay unquoted until a re-attribution can move them. Three other
copied rows were retired, as `rfc/corrections/draft-ietf-sidrops-8210bis.md`
records: the two AFI rows (5.12-6, 5.12-9) state what no document says, and the
ordering row (5.12-8) repeats what 5.12-3 quotes.

