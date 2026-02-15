---
paths:
  - "internal/bgp/**/*.go"
---

# RFC Compliance

Ze MUST be a fully RFC 4271 compliant BGP speaker.

## Before Implementing BGP Features

1. Find RFC in `rfc/` folder
2. If missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
3. Read relevant sections
4. Note MUST/SHOULD/MAY requirements
5. Check ExaBGP reference

## Priority Order

1. **RFC compliance** - Always follow RFC specification
2. **ExaBGP API compatibility** - Match ExaBGP's interface
3. **ExaBGP implementation** - Follow approach when RFC-compliant

## Wire Format Documentation (MANDATORY)

**Never modify protocol code without documenting the wire format.**

### Required Documentation

```go
// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2)
//
// Wire format (19 bytes):
//
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |           Length (2)          |    Route Distinguisher (8)    |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |          VE ID (2)            |      Label Block Offset (2)   |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Byte offsets:
//   [0:2]   - Length
//   [2:10]  - Route Distinguisher
//   [10:12] - VE ID
type VPLS struct { ... }
```

### Pre-Modification Checklist

```
## Pre-Change: [Feature Name]

- [ ] RFC identified: RFC ____
- [ ] Section: ____
- [ ] Wire format documented in code
- [ ] Byte offsets verified against RFC
- [ ] TestPack exists
- [ ] TestUnpack exists
- [ ] TestRoundtrip exists
```

## Code Requirements

```go
// parseOpenMessage parses a BGP OPEN message.
// RFC 4271 Section 4.2 - OPEN Message Format
func parseOpenMessage(data []byte) (*OpenMessage, error) { ... }
```

### RFC MUST Requirement Comments (BLOCKING)

**BLOCKING:** Every RFC MUST/MUST NOT requirement implemented in code MUST have a comment directly above the enforcing code that:

1. Cites the RFC number and section
2. Quotes the requirement text (or paraphrases with "MUST"/"MUST NOT")
3. Is placed immediately above the code that enforces it

**This applies to:**
- Validation checks (field ranges, required values, format constraints)
- Error conditions and responses (NOTIFICATION codes, session resets)
- State machine transitions (FSM event ordering, timer behavior)
- Message ordering requirements (OPEN before UPDATE, etc.)
- Wire format constraints (minimum lengths, field sizes, flag bits)
- Any code path triggered by a MUST/MUST NOT/SHALL/SHALL NOT from an RFC

**Format:**

```go
// RFC NNNN Section X.Y: "quoted requirement text"
// Brief explanation if the connection between quote and code isn't obvious.
<code that enforces it>
```

**Examples:**

```go
// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values of one or two seconds."
if holdTime == 1 || holdTime == 2 {
    return ErrInvalidHoldTime
}

// RFC 7606 Section 3.g: "If the MP_REACH_NLRI attribute or the MP_UNREACH_NLRI attribute
// appears more than once in the UPDATE message, then a NOTIFICATION message MUST be sent"
if mpReachCount > 1 || mpUnreachCount > 1 {
    return sessionReset("multiple MP_REACH/MP_UNREACH")
}

// RFC 7606 Section 2: treat-as-withdraw "MUST be handled as though all of the routes
// contained in an UPDATE message ... had been withdrawn"
// Do not dispatch to plugins — the routes are treated as withdrawn.
return nil
```

**Anti-patterns (FORBIDDEN):**

```go
// BAD: No RFC citation
if length != 4 {
    return ErrBadLength
}

// BAD: Citation but no quoted requirement
// RFC 7606 Section 7.4
if length != 4 {
    return ErrBadMED
}

// BAD: Comment far from enforcing code
// RFC 7606 Section 7.1: ORIGIN must be length 1
... 20 lines of other code ...
if length != 1 {  // <- too far from comment
```

**When reviewing code:** If you see RFC-enforcing logic without a quoted MUST comment, add one before proceeding.

## RFC MAY Clauses

When encountering MAY clauses, ASK user:
1. Implement this behavior?
2. Skip it?
3. Add configuration option?

## When ExaBGP Differs from RFC

```go
// parseFeature implements RFC NNNN Section X.Y.
// NOTE: ExaBGP does [X] differently, but RFC requires [Y].
// We follow RFC here for compliance.
func parseFeature(...) { ... }
```

## Common RFCs

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `internal/bgp/message/`, `internal/bgp/fsm/` |
| MP-BGP | 4760 | `internal/bgp/nlri/`, `internal/bgp/attribute/mpreach.go` |
| EVPN | 7432 | `internal/bgp/nlri/evpn/` |
| FlowSpec | 8955 | `internal/bgp/nlri/flowspec.go` |
| BGP-LS | 7752 | `internal/bgp/nlri/bgpls/` |
| 4-byte ASN | 6793 | `internal/bgp/capability/asn4.go` |
| Add-Path | 7911 | `internal/bgp/capability/addpath.go` |
| Graceful Restart | 4724 | `internal/bgp/capability/graceful.go` |

## Key ExaBGP Directories

Base: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- `bgp/message/` - Message encoding/decoding
- `bgp/message/open/capability/` - Capability negotiation
- `bgp/message/update/attribute/` - Path attributes
- `bgp/message/update/nlri/` - NLRI types

## Red Flags - STOP

- No RFC reference → Find and read the RFC
- No wire format diagram → Add one before changing
- No pack/unpack tests → Write them first
- "I'll document later" → No. RFC first.
