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

- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-1] [MUST] An ASPA announcement MUST contain at least one Provider Autonomous System Number; otherwise the router MUST return Error Report 9 (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-3] [MUST] Each Provider Autonomous System Number in a given ASPA PDU MUST be unique; the provider fields are in increasing numeric order (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-4] [MUST] The router MUST see at most one active ASPA from a particular cache for a particular Customer Autonomous System Number; the cache MUST deliver the complete data for that customer in a single PDU (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-5] [MUST] An ASPA withdrawal MUST provide the Customer AS, contain no provider list, and have PDU Length 12; the router MUST remove that customer's entire ASPA record from that cache (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-1] [MUST] Router starting a v2 session MUST send query with version=2 (§7) {single-polarity: positive; ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively}
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-2] [MUST] If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during negotiation, it MUST either downgrade to a known version or terminate the connection, with Error Report 4 unless the received PDU is itself an Error Report (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-3] [MUST] A cache receiving an unsupported query version MUST send Error Report 4 with its supported version C; when Q < C and the cache supports no version <= Q, it MUST also disconnect the transport (§7)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-7-4] [MUST] The router MUST initiate the session with a Reset Query or Serial Query carrying the highest protocol version it implements (§7)

## Historical Requirement Correspondence

`RFC9582-` below identifies the historical rows retained in `rfc9582.md`.
The destination prefix is `DRAFT-IETF-SIDROPS-8210BIS-`. These are candidate
identity corrections for the existing reattribution design, not a claim of
completed tag migration.

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

The following five copied checklist rows are historical claims only. Their
current-draft defects are recorded above; they remain identifiable until the
reattribution and correction workflow can preserve or retire their associated
proof without assigning unrelated semantics to an existing ID.

- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-2] [MUST NOT] Customer AS MUST NOT appear in its own provider set (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-6] [MUST] Router MUST ignore ASPA PDUs with unknown AFI values (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-7] [MUST NOT] Customer AS 0 is reserved, MUST NOT appear (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-8] [SHOULD] Provider ASNs SHOULD be sorted ascending; cache MUST sort, router SHOULD verify (§5.12)
- [ ] [DRAFT-IETF-SIDROPS-8210BIS-5.12-9] [MAY] Router MAY ignore AFI field and apply ASPA to all address families (§5.12)
