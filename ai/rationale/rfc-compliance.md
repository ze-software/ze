# RFC Compliance Rationale

Why: `ai/rules/rfc-compliance.md`

## Wire Format Documentation Example

```go
// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2)
//
// Wire format (19 bytes):
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |           Length (2)          |    Route Distinguisher (8)    |
```

## RFC MUST Comment Examples

```go
// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values of one or two seconds."
if holdTime == 1 || holdTime == 2 { return ErrInvalidHoldTime }

// RFC 7606 Section 3.g: "If MP_REACH_NLRI appears more than once, NOTIFICATION MUST be sent"
if mpReachCount > 1 { return sessionReset("multiple MP_REACH") }
```

## Anti-Pattern: Citation Without Quote

```go
// BAD: No quoted requirement
// RFC 7606 Section 7.4
if length != 4 { return ErrBadMED }
```

## Pre-Modification Checklist

```
- [ ] RFC identified
- [ ] Section identified
- [ ] Wire format documented
- [ ] Byte offsets verified
- [ ] Pack/Unpack/Roundtrip tests exist
```

## Key ExaBGP Directories

Base: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- `bgp/message/` — encoding/decoding
- `bgp/message/open/capability/` — capabilities
- `bgp/message/update/attribute/` — attributes
- `bgp/message/update/nlri/` — NLRI types

## History moved from rule points (2026-09-27)

- From `ai/rules/points/rfc-compliance/directives/a-requirement-list-is-a-claim-until-it-is-walked.md`: "(owner directive, 2026-09-21)"
- From `ai/rules/points/rfc-compliance/directives/a-requirement-list-is-a-claim-until-it-is-walked.md`: "Measured 2026-09-21 over the first 40 documents walked: RFC 8571 published four MUSTs and contains no RFC 2119 keyword at all; RFC 7611 published five rows written in terms of iBGP, eBGP and AS_PATH, none of which appears in the document; RFC 5561 published the F-bit MUST be 1 where the RFC says it MUST be 0. Fabrication ran ahead of omission, and every instance was invisible until somebody read the document."
- From `ai/rules/points/rfc-compliance/directives/a-requirement-list-is-a-claim-until-it-is-walked.md`: "On 2026-09-21 the weekly update stated \"3,322 checked, 228 owing\" as a conformance measure. The walks that followed took the same corpus to 678 owing, because 401 MUST-level obligations the RFCs state had never been written down. Restating the new number alone repeats the original error; the correction says that the earlier figure counted a list nobody had checked."
- From `ai/rules/points/rfc-compliance/directives/name-who-implements-each-rfc.md`: "(owner directive, 2026-09-21)"
- From `ai/rules/points/rfc-compliance/directives/quote-each-requirement-row-verbatim.md`: "(owner directive, 2026-09-26)"
- From `ai/rules/points/rfc-compliance/directives/quote-the-rfc-text-a-new-function-implements.md`: "(owner directive, 2026-09-24)". The scope date "added from 2026-09-24" stays in the point, because it bounds which functions the directive covers.
