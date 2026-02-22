---
paths:
  - "internal/bgp/**/*.go"
---

# RFC Compliance

Ze MUST be a fully RFC 4271 compliant BGP speaker.
Rationale: `.claude/rationale/rfc-compliance.md`

## Before Implementing BGP Features

1. Find RFC in `rfc/` — if missing: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
2. Read relevant sections, note MUST/SHOULD/MAY
3. Check ExaBGP reference

**Priority:** RFC > ExaBGP API compat > ExaBGP implementation

## Wire Format Documentation (MANDATORY)

Never modify protocol code without documenting wire format: ASCII diagram with field offsets, byte offset annotations, RFC section reference.

## RFC MUST Comments (BLOCKING)

Every MUST/MUST NOT enforced in code needs a comment directly above:
```
// RFC NNNN Section X.Y: "quoted requirement"
<enforcing code>
```

Document: validation rules, error conditions, state transitions, timer constraints, message ordering.

## MAY Clauses

ASK user: implement? skip? config option?

## Common RFCs

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `message/`, `fsm/` |
| MP-BGP | 4760 | `nlri/`, `attribute/mpreach.go` |
| 4-byte ASN | 6793 | `capability/asn4.go` |
| Add-Path | 7911 | `capability/addpath.go` |
| GR | 4724 | `capability/graceful.go` |
| Error handling | 7606 | revised error handling |

ExaBGP ref: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
