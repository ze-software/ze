---
paths:
  - "internal/bgp/**/*.go"
---

# RFC Compliance

ZeBGP MUST be a fully RFC 4271 compliant BGP speaker.

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

### RFC Constraint Comments

When code enforces an RFC rule, document it:

```go
// RFC 4271 Section 6.3: "If the UPDATE message is received from an external peer"
// MUST check that AS_PATH first segment is neighbor's AS
if peer.IsExternal() && path.FirstAS() != peer.RemoteAS {
    return ErrInvalidASPath
}
```

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
