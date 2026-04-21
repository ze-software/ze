# RFC Compliance

Ze MUST be a fully RFC 4271 compliant BGP speaker.
Rationale: `ai/rationale/rfc-compliance.md`

## RFC Summaries (`rfc/short/`)

RFC summaries are protocol-only reference documents. They must NOT contain
Ze-specific information: no Ze implementation notes, no Ze file paths, no
"Ze does/does not" statements, no "for ze" sections. Implementation
decisions belong in specs (`plan/`), architecture docs (`docs/architecture/`),
or code comments. A reader should be able to use any `rfc/short/` file
as a standalone protocol reference with no knowledge of Ze.

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
| BGP-4 base | 4271 | `internal/component/bgp/message/`, `internal/component/bgp/reactor/` |
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/component/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/component/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/component/bgp/capability/capability.go` |
| GR | 4724 | `internal/component/bgp/capability/capability.go` |
| Error handling | 7606 | revised error handling |

ExaBGP ref: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
