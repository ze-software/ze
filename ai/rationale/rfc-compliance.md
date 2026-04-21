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
