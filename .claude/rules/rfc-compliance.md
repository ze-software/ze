---
globs: "pkg/bgp/**/*.go"
---

# RFC Compliance

ZeBGP MUST be a fully RFC 4271 compliant BGP speaker.

## Before Implementing BGP Features

1. Find RFC in `rfc/` folder
2. If missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
3. Read relevant sections
4. Note MUST/SHOULD/MAY requirements
5. Check ExaBGP reference: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/`

## Code Requirements

```go
// parseOpenMessage parses a BGP OPEN message.
// RFC 4271 Section 4.2 - OPEN Message Format
func parseOpenMessage(data []byte) (*OpenMessage, error) {
    // ...
}
```

## Priority Order
1. **RFC compliance** - Always follow RFC specification
2. **ExaBGP API compatibility** - Match ExaBGP's interface
3. **ExaBGP implementation** - Follow approach when RFC-compliant

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

## Key ExaBGP Directories
Base: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- `bgp/message/` - Message encoding/decoding
- `bgp/message/open/capability/` - Capability negotiation
- `bgp/message/update/attribute/` - Path attributes
- `bgp/message/update/nlri/` - NLRI types
