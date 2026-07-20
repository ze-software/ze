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

## Extraction Completeness (BLOCKING when enrolling a summary)

`make ze-rfc-check` verifies that every requirement **listed** in a summary is
covered. It cannot know about an obligation nobody wrote down. A green gate is
bounded by what was extracted, so a missing extraction is invisible to it and to
any audit that only re-checks classifications.

Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, walk the RFC's own
text section by section and confirm every MUST / MUST NOT / SHALL / SHALL NOT /
REQUIRED has a checklist row. Fetch the source first if it is absent:
`curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`. A
claim of "verified against the RFC" is not reproducible when `rfc/full/` lacks
the file.

Two signals that an extraction is missing, both seen in practice:

| Signal | Why it matters |
|--------|----------------|
| A `{not-applicable}` whose reason is "ze has no X producer at all" | That admission is often the violation of a separate MUST requiring X to exist. RFC 4271 §5.1.4's "MUST implement a mechanism ... that allows MULTI_EXIT_DISC to be removed" was unextracted, and two requirements cited its absence as their exemption. |
| A section whose siblings are enumerated but one clause is not | RFC 8666 §5's "MUST be ignored on reception" was omitted while §6, §7.1 and §7.2 each had it. An enumeration hole, not a style choice. |

Also verify the requirement TEXT matches the RFC. A misquoted obligation licenses
a justification that never engages it: RFC 4271 §5.1.6 binds a speaker **that
receives** a route with ATOMIC_AGGREGATE, and recording it as an aggregator rule
let the readvertisement path be cited as evidence of non-applicability when it is
the bound path.

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
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/core/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/core/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/core/bgp/capability/capability.go` |
| GR | 4724 | `internal/core/bgp/capability/capability.go` |
| Error handling | 7606 | revised error handling |

ExaBGP ref: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
