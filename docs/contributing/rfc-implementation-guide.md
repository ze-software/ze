# RFC Implementation Guide for Ze

This guide provides a step-by-step checklist for implementing an RFC in Ze. Use it alongside `planning.md` to ensure complete implementations.

## Overview

An RFC implementation typically touches these areas (not all apply to every RFC):

| Component | Package | When Needed |
|-----------|---------|-------------|
| Capability | `internal/core/bgp/capability/` | RFC introduces a capability |
| Attribute | `internal/core/bgp/attribute/` | RFC introduces path attributes |
| NLRI | `internal/core/bgp/nlri/` | RFC introduces new AFI/SAFI |
| Message | `internal/component/bgp/message/` | RFC modifies message format |
| FSM | `internal/component/bgp/fsm/` | RFC affects state machine |
| Config | `internal/component/config/` | RFC needs configuration |
| Plugin | `internal/component/plugin/` | RFC needs plugin commands |
| Context | `internal/core/bgp/context/` | RFC affects encoding context |

<!-- source: internal/core/bgp/capability/capability.go -- capability codes and parsing -->
<!-- source: internal/core/bgp/attribute/attribute.go -- attribute code constants -->
<!-- source: internal/core/bgp/context/context.go -- PackContext encoding rules -->

## Phase 0: Preparation

### 0.1 RFC Analysis

```
[ ] Download RFC: curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt
[ ] Create summary: rfc/short/rfcNNNN.md (use /rfc-summarisation)
[ ] Identify RFC dependencies (other RFCs this one references)
[ ] Check dependency RFCs are implemented or summarized
[ ] Identify which components this RFC affects (table above)
```

### 0.2 Codebase Analysis

```
[ ] Search for existing partial implementation: grep -r "RFC NNNN" internal/
[ ] Check if related capabilities exist: internal/core/bgp/capability/
[ ] Check if related attributes exist: internal/core/bgp/attribute/
[ ] Check if related NLRI types exist: internal/core/bgp/nlri/
[ ] Read architecture docs for affected areas (see planning.md keyword table)
```

### 0.3 ExaBGP Migration Consideration

If this RFC adds features that ExaBGP users might rely on, check if migration support is needed:

| RFC Affects | Migration Impact | Action |
|-------------|------------------|--------|
| API commands/events | ExaBGP plugins expect different JSON format | Update `internal/exabgp/bridge/` |
| Config syntax | ExaBGP configs have different syntax | Update `internal/exabgp/migration/` |
| Capabilities | ExaBGP may configure differently | Check migration handles it |

```
[ ] Does ExaBGP support this RFC feature?
[ ] If yes: is config migration needed? (internal/exabgp/migration/)
[ ] If yes: is API bridge update needed? (internal/exabgp/bridge/)
```

See `ai/rules/go-standards.md` for architecture details.

### 0.4 Spec Creation

```
[ ] Create spec: plan/spec-rfcNNNN-<feature>.md
[ ] Fill Required Reading section with identified docs
[ ] git add the spec immediately
```

---

## Phase 1: Capability (if applicable)

**When:** RFC introduces a BGP capability (advertised in OPEN message)

### 1.1 Define Capability

```
[ ] Add capability code constant to internal/core/bgp/capability/capability.go
    - Code<Name> Code = NN  // RFC NNNN

[ ] Create capability struct in appropriate file (or new file)
    - encoding.go for wire-format affecting caps
    - session.go for session behavior caps
    - new file for complex capabilities

[ ] Implement Capability interface:
    - Code() Code
    - Len() int
    - WriteTo(buf []byte, off int) int

[ ] Implement ConfigProvider interface if cap provides plugin config:
    - ConfigValues() map[string]string
```

### 1.2 Wire Format

```
[ ] Document wire format with ASCII diagram in code comment
[ ] Implement constructor: New<Name>(...) *<Name>
[ ] Implement parser: parse<Name>(data []byte) (*<Name>, error)
[ ] Add case to parseCapability() switch in capability.go
[ ] Handle malformed data gracefully (return error, don't panic)
```

### 1.3 Negotiation

```
[ ] If cap affects encoding: add field to EncodingCaps (encoding.go)
[ ] If cap affects session: add field to SessionCaps (session.go)
[ ] Update Negotiate() to handle intersection logic
[ ] Update Negotiated accessors if needed
[ ] Document negotiation rules in code comments with RFC section refs
```

### 1.4 Tests

```
[ ] Unit test: WriteTo round-trips correctly
[ ] Unit test: Parse valid wire bytes
[ ] Unit test: Parse rejects malformed bytes
[ ] Unit test: Negotiation logic (both have, one has, neither has)
[ ] Boundary test: min/max values for any numeric fields
```

---

## Phase 2: Attribute (if applicable)

**When:** RFC introduces new BGP path attribute(s)

### 2.1 Define Attribute

```
[ ] Add attribute code constant to internal/core/bgp/attribute/attribute.go
    - Attr<Name> AttributeCode = NN  // RFC NNNN

[ ] Create attribute struct (new file if complex, or add to existing)

[ ] Implement Attribute interface:
    - Code() AttributeCode
    - Flags() AttributeFlags
    - Len() int
    - WriteTo(buf []byte, off int) int
    - WriteToWithContext(buf []byte, off int, ctx *PackContext) int (if context-dependent)
    - String() string
```

### 2.2 Wire Format

```
[ ] Document wire format with ASCII diagram
[ ] Implement constructor: New<Name>(...) *<Name>
[ ] Implement parser: Parse<Name>(data []byte) (*<Name>, error)
[ ] Handle optional/transitive flags per RFC
[ ] If context-dependent: implement WriteToWithContext()
```

### 2.3 Builder Integration

```
[ ] Add setter to Builder: Set<Name>(...) *Builder
[ ] Add parser support in builder_parse.go if text syntax needed
[ ] Document text syntax in builder_parse.go
```

### 2.4 Iterator Support

```
[ ] Ensure Iterator can return attribute via existing pattern
[ ] Add helper to extract typed attribute if frequently needed
```

### 2.5 Tests

```
[ ] Unit test: WriteTo produces correct wire bytes
[ ] Unit test: Parse valid wire bytes
[ ] Unit test: Parse rejects malformed bytes
[ ] Unit test: Flags are set correctly
[ ] Unit test: Builder integration
[ ] Boundary test: length limits, value ranges
```

---

## Phase 3: NLRI (if applicable)

**When:** RFC introduces new AFI/SAFI or NLRI encoding

### 3.1 Define Family

```
[ ] Add AFI constant if new: internal/core/bgp/nlri/constants.go
[ ] Add SAFI constant if new: internal/core/bgp/nlri/constants.go
[ ] Add Family constant: var <Name> = Family{AFI: ..., SAFI: ...}
[ ] Register in familyNames map for string parsing
```

### 3.2 Define NLRI Type

```
[ ] Create NLRI struct (new file for complex types)

[ ] Implement NLRI interface:
    - Family() Family
    - Bytes() []byte (payload only, no path ID; returned slice may be shared)
    - Len() int (payload length, no path ID)
    - PathID() uint32
    - WriteTo(buf []byte, off int) int (payload only, no path ID)
    - SupportsAddPath() bool
    - String() string

[ ] If ADD-PATH supported:
    - SupportsAddPath() returns true
    - Use WriteNLRI() helper for ADD-PATH aware encoding
```

### 3.3 Wire Format

```
[ ] Document wire format with ASCII diagram
[ ] Implement constructor(s) for creating NLRI
[ ] Implement parser for wire bytes
[ ] Handle variable-length fields correctly
[ ] Add UPDATE builder in internal/component/bgp/message/update_build_<type>.go if needed
```

### 3.4 Iterator Support

```
[ ] Add parsing support to NLRI iterator if needed
[ ] Ensure family-specific parsing in iterator works
```

### 3.5 Tests

```
[ ] Unit test: WriteTo produces correct wire bytes
[ ] Unit test: Parse valid wire bytes
[ ] Unit test: Round-trip (create -> write -> parse -> compare)
[ ] Unit test: ADD-PATH handling (with/without path ID)
[ ] Unit test: String() produces readable output
[ ] Boundary test: max prefix length, label values, etc.
```

---

## Phase 4: Message Changes (if applicable)

**When:** RFC modifies BGP message format or introduces new message type

### 4.1 New Message Type

```
[ ] Add message type constant to internal/component/bgp/message/message.go
[ ] Create message struct implementing Message interface:
    - Type() MessageType
    - Len(ctx *EncodingContext) int
    - WriteTo(buf []byte, off int, ctx *EncodingContext) int
[ ] Add case to message dispatcher in message.go
```

### 4.2 Message Modification

```
[ ] Update affected message struct
[ ] Update Len() calculation
[ ] Update WriteTo() implementation
[ ] Update parser if receiving this message
[ ] Document changes with RFC section references
```

### 4.3 Tests

```
[ ] Unit test: Message builds correctly
[ ] Unit test: Message parses correctly
[ ] Unit test: Round-trip encoding
[ ] Boundary test: max lengths, extended message handling
```

---

## Phase 5: FSM Changes (if applicable)

**When:** RFC affects BGP state machine behavior

### 5.1 State/Event Changes

```
[ ] Add new states if needed (rare)
[ ] Add new events if needed
[ ] Update state transition table
[ ] Document RFC section for each change
```

### 5.2 Timer Changes

```
[ ] Add new timers if needed
[ ] Update timer handling logic
[ ] Document timer semantics with RFC refs
```

### 5.3 Tests

```
[ ] Unit test: State transitions
[ ] Unit test: Timer behavior
[ ] Integration test if complex
```

---

## Phase 6: Configuration

**When:** RFC feature needs user configuration

### 6.1 Schema Definition

```
[ ] Add schema nodes to internal/component/config/schema.go
[ ] Define value types and constraints
[ ] Add validation rules
[ ] Document config syntax in schema comments
```

### 6.2 Parsing

```
[ ] Update parser if new syntax patterns needed
[ ] Add to appropriate config section (global, peer, family)
[ ] Handle defaults appropriately
```

### 6.3 Validation

```
[ ] Config rejects unknown keys (Ze rule)
[ ] Config validates value ranges
[ ] Config validates inter-field dependencies
```

### 6.4 Tests

```
[ ] Valid config test: test/parse/<feature>.ci (expect=exit:code=0)
[ ] Invalid config test: test/parse/<feature>-invalid.ci (expect=exit:code=1 + expect=stderr:contains=)
[ ] Test all validation rules trigger appropriately
```

---

## Phase 7: API Commands (if applicable)

**When:** RFC feature needs plugin control/visibility

### 7.1 Text Commands

```
[ ] Design command syntax (see docs/architecture/api/update-syntax.md)
[ ] Implement parser in appropriate location
[ ] Implement handler
[ ] Document syntax in architecture docs
```

### 7.2 Response Format

```
[ ] Define JSON response structure if applicable
[ ] Ensure consistency with existing response patterns
```

### 7.3 Tests

```
[ ] Functional test: test/plugin/<feature>/
[ ] Test valid command variations
[ ] Test error handling for invalid commands
```

---

## Phase 8: Engine Integration

**When:** RFC affects how router processes messages

### 8.1 Reactor Changes

```
[ ] Update message handling in reactor
[ ] Add new handlers if needed
[ ] Update routing logic if affected
```

### 8.2 Peer Changes

```
[ ] Update peer state management
[ ] Update capability negotiation handling
[ ] Update message sending logic
```

### 8.3 Tests

```
[ ] Integration tests with test peer
[ ] Functional tests for end-to-end behavior
```

---

## Phase 9: Functional Tests

**When:** Always (every RFC implementation needs functional tests)

**Purpose:** Verify the feature works as users expect, end-to-end. Unit tests verify internal correctness; functional tests verify user-facing behavior.

### 9.1 User Scenario Tests

Think from the user's perspective: "If I configure X and send command Y, what should happen?"

```
[ ] Identify user-facing scenarios this RFC enables
[ ] For each scenario, create a functional test that:
    -> Configures the feature as a user would
    -> Exercises the feature through normal usage (API commands, peer interaction)
    -> Verifies the observable outcome (wire bytes sent, events received, state changes)
```

**Example scenarios by RFC type:**

| RFC Type | User Scenario |
|----------|---------------|
| Capability | "When I enable X, peer receives capability in OPEN" |
| Attribute | "When I announce route with X, UPDATE contains correct attribute" |
| NLRI | "When I announce prefix in family X, wire encoding is correct" |
| FSM | "When peer does X, session transitions to correct state" |
| Error | "When I receive malformed X, session handles it correctly" |

### 9.2 Encoding Tests

```
[ ] Create test/encode/<feature>.conf (peer config with feature enabled)
[ ] Create test/encode/<feature>.ci (command/wire pairs)
[ ] Test happy path: feature used correctly produces correct wire bytes
[ ] Test variations: different parameter combinations
[ ] Test boundaries: min/max values that affect encoding
```

### 9.3 Plugin Tests

```
[ ] Create test/plugin/<feature>/ directory
[ ] Test plugin receives correct JSON events when feature is active
[ ] Test plugin commands produce correct behavior
[ ] Test error responses when plugin sends invalid commands
```

### 9.4 Config Tests

```
[ ] Create test/parse/<feature>.ci - valid configurations (expect=exit:code=0)
[ ] Create test/parse/<feature>-invalid.ci - invalid configs (expect=exit:code=1 + expect=stderr:contains=)
[ ] Test feature enables/disables correctly via config
[ ] Test config validation catches user mistakes
```

### 9.5 Integration Tests

```
[ ] Test end-to-end with ze-peer
[ ] Test capability negotiation (both peers support, one supports, neither supports)
[ ] Test message exchange in realistic scenarios
[ ] Test interop edge cases (if applicable)
```

### 9.6 Negative Tests

```
[ ] Test graceful handling of malformed input
[ ] Test behavior when feature is disabled but peer uses it
[ ] Test error messages are helpful to users
```

### 9.7 RFC Requirement Coverage Tags

Implementing a MUST is only half the job: the enforcing test must be **bound** to
the requirement, so the coverage gate can prove the link exists and catch a later
regression. When a test proves an RFC 2119 MUST-level obligation, tag it. In Go:

```go
// RFC requirement: RFC7606-7.1-1 positive -- valid ORIGIN length 1 is accepted
// RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
```

A `.ci` functional test uses a line-start `#` comment with the same fields (never
inside a `terminator=` block):

```
# RFC requirement: RFC7606-7.1-1 negative -- malformed ORIGIN withdraws the route
```

- **Allocate the id with `/ze-rfc <rfc>`.** Every Compliance Checklist line in
  `rfc/short/rfcNNNN.md` gets a permanent `RFC<n>-<section>-<ordinal>` id, and that
  id is the contract the tag references. Never renumber or reuse one.
- **Provide BOTH polarities.** Every gated MUST needs a `positive` AND a `negative`
  test; a one-sided test passes on blanket accept or blanket reject. If a
  requirement is genuinely testable only one way, annotate its summary line
  `{single-polarity: positive|negative; why}` instead. `{gap: why; ref}` and
  `{not-applicable: why}` cover deliberate divergence and inapplicability, each
  with a reason (a bare annotation is rejected).
- **Five RFC make targets, and each clears a different red.**
  - `make ze-rfc-check` gates coverage and validates the audit records.
  - `make ze-rfc-index` renders the ledger: one file per RFC under `rfc/requirements/`,
    and the index over them (`ai/RFC-REQUIREMENTS.md`).
  - `make ze-rfc-extract STEM=<stem>` writes an extraction skeleton.
  - `make ze-rfc-extraction-status` prints the sign-off counts.
  - `make ze-rfc-reseal` re-stamps an audit verdict a mechanical edit staled.

  For an enrolled RFC (`rfc/enrolled.txt`) the gate fails unless every MUST has its
  pair or a reasoned annotation. Writing a summary does NOT enroll an RFC.
  Enrollment is a separate, deliberate step taken once the tests exist.
  <!-- source: Makefile — ze-rfc-check, ze-rfc-index, ze-rfc-extract, ze-rfc-extraction-status, ze-rfc-reseal -->
- **Enrol it, or declare why not.** Every summary under `rfc/short/` is in
  `rfc/enrolled.txt` or in `rfc/not-enrolled.txt`. One in neither reds the gate.
  Un-enrolment used to be the one state that carried no information. So "the RFC
  imposes nothing", "nobody extracted it" and "we do not even have the text" all
  looked identical. A disposition row is `<stem>` TAB `<kind>` TAB `<reason>`, with
  kind one of `non-normative`, `backlog` or `blocked`.

  Only `non-normative` is a claim about conformance. Its reason must state a property
  of the DOCUMENT: its category, a missing RFC 2119 section, a keyword scan. It must
  never say the obligation does not apply to Ze. `backlog` and `blocked` are debt,
  and the ledger renders them as debt. A row leaves the file only by arriving in
  `rfc/enrolled.txt`.

  Two related reds come from the same place. First, a `docs/features/rfc-status.md`
  row that claims support over a summary with zero gated requirements. The escape is
  evidence that zero is real: a `non-normative` disposition, or a `manual-walk`
  extraction sign-off whose `register-reason` says why. Second, a Remaining cell that
  spells a gap count immediately before MUST or SHALL must agree with the summary's
  `{gap}` count.
  <!-- source: scripts/dev/rfc_requirements.py — check_summary_disposition, check_unproven_support, check_gap_count_agreement -->
- **Audit letter and spirit with `/ze-rfc-audit <rfc>`.** The gate proves a link
  exists, but it cannot read the test. The audit reads the RFC itself and each
  tagged test. It then judges whether the test would fail if the code stopped
  complying, and records a per-requirement verdict in `rfc/audit/<rfc>.json`.
  The verdict is one of five closed values, and the gate reads it:
  - `enforced` is the only one that means proven.
  - `weak`, `wrong`, `unimplemented` and `not-applicable` each subtract the
    requirement from the published proven count. That count is in the ledger's
    **Audit coverage** section, and the gate still exits 0.

  Recording a finding is free, and deleting one is not. `make ze-rfc-check`
  re-stales a verdict when the requirement text, the tagged test's own function,
  or a cited producer changes.
  <!-- source: scripts/dev/rfc_requirements.py — AUDIT_VERDICTS, check_audit_schema, audit_coverage -->
- **A `SHIFTED` verdict is not your problem to re-read.** When the gate says a
  verdict is SHIFTED, the tagged unit is byte-identical and only the file around it
  moved — a line shift, a sibling test, a rewritten import. Run
  `make ze-rfc-reseal` then `make ze-rfc-index`. It is the only command that writes
  `rfc/audit/`, and that is deliberate. A check that also wrote cannot be trusted
  to report. And a regen target that wrote evidence would re-stamp hand-authored
  judgements during unrelated work.
  <!-- source: scripts/dev/rfc_requirements.py — verdict_freshness, run_reseal -->
- **A `STALE` verdict is.** The tagged unit itself changed, so re-run
  `/ze-rfc-audit <rfc>`. The re-seal refuses that case by design.
- **Never change a tagged test to make it pass.** Once a test carries an
  `RFC requirement:` tag it is the requirement: fix your code, not the test.
  Changing its behavior needs explicit user approval recorded as
  `// rfc-test-change-approved: <date> <what and why>`; the `rfc-tagged-test` hook
  blocks the edit otherwise.

<!-- source: scripts/dev/rfc_requirements.py -- scan_go_tags/scan_ci_tags, evaluate -->
<!-- source: ai/skills/ze-rfc.md -- requirement id allocation and annotations -->

Full rules: `ai/skills/ze-rfc.md`; audit method: `ai/skills/ze-rfc-audit.md`.

```
[ ] Every MUST-level line in rfc/short/rfcNNNN.md has an id (allocated by /ze-rfc)
[ ] Each gated MUST has a positive AND a negative tagged test, or a reasoned annotation
[ ] make ze-rfc-check passes; rfc/enrolled.txt lists the RFC once its tests exist
[ ] If not enrolled: rfc/not-enrolled.txt carries a kind and a reason for it
```

---

## Phase 10: Documentation

### 10.1 Architecture Docs

```
[ ] Update relevant docs in docs/architecture/
[ ] Add wire format documentation if new formats
[ ] Update capability list if new capability
[ ] Update attribute list if new attribute
[ ] Update NLRI list if new family
```

### 10.2 RFC Summary

```
[ ] Ensure rfc/short/rfcNNNN.md is complete
[ ] Add Ze implementation notes section
[ ] Cross-reference related RFCs
[ ] Every MUST-level line has a stable id (see 9.7); disclose any {gap} in docs/features/rfc-status.md
[ ] Extraction sign-off recorded: make ze-rfc-extract STEM=rfcNNNN, then classify
    every derived site and section in rfc/extraction/rfcNNNN.json (see 10.4)
```

### 10.3 Config Examples

```
[ ] Add example configs showing feature usage
[ ] Document in relevant architecture docs
```

### 10.4 Extraction Sign-Off

The checklist above proves the requirements you WROTE DOWN are enforced. Nothing
in it bounds what the summary MISSED, and a green `make ze-rfc-check` is bounded
by what was extracted. Record the walk in an artifact the gate re-checks:

```
make ze-rfc-extract STEM=rfcNNNN     # writes an UNCLASSIFIED skeleton
                                      # classify every site and section by hand
make ze-rfc-check                     # re-derives the inventory and judges it
```

Each derived site (`<section>:<n>`, with the sentence it came from) is `mapped`
to a requirement id or `excluded` with a kind from a closed set and a reason;
each section is `walked` or `skipped`. An unclassified site fails the gate, so
generating the skeleton cannot produce a sign-off, only the walk can. Enrolling
a stem that was not enrolled at HEAD REQUIRES this artifact. Contract and field
reference: `rfc/extraction/README.md`.

One exclusion kind does not dismiss its sentence. `relocated-to-spec` says the
obligation is owed by a named spec, under an id reserved there, because an owner
ruling moved it out of the summary. It authors `relocated-to`
(`plan/spec-<name>.md`) and `reserved-id`, and the gate refuses the sign-off
unless that spec exists and still names that id.

<!-- source: scripts/dev/rfc_requirements.py -- run_extract_skeleton/check_extraction_signoff/_relocation_errors -->

The counts machine-readably (signed, enrolled, the per-register split, the
relocated count, and the unsigned backlog): `make ze-rfc-extraction-status`. Do NOT spell it
`make ze-rfc-extraction-status --json` -- GNU make reads `--json` as one of its own
options and exits 2 before the recipe runs. The target always emits JSON.

<!-- source: Makefile -- ze-rfc-extraction-status -->
<!-- source: scripts/dev/rfc_requirements.py -- run_extraction_status -->



---

## Final Checklist

Before marking implementation complete:

```
[ ] All tests pass: make ze-test (timeout 300s)
[ ] All linting passes: make ze-lint (zero issues)
[ ] All functional tests pass: make ze-functional-test
[ ] RFC MUST tests tagged both polarities, make ze-rfc-check passes (see 9.7)
[ ] RFC section comments on all protocol code
[ ] RFC constraint comments with quoted requirements
[ ] RFC requirement coverage tags on tests (see 9.7): make ze-rfc-check passes
[ ] No backwards-compatibility shims (Ze rule)
[ ] No version numbers in config (Ze rule)
[ ] Architecture docs updated
[ ] Write learned summary to plan/learned/NNN-<name>.md
[ ] All changes in single commit
```

---

## Quick Reference: Common Patterns

### Wire Writing Pattern

```go
// All wire types implement this
func (x *Type) WriteTo(buf []byte, off int) int {
    // Write directly to buf at offset
    // Return number of bytes written
}

// Context-dependent types add this
func (x *Type) WriteToWithContext(buf []byte, off int, ctx *PackContext) int {
    // Use ctx for ASN4, ADD-PATH decisions
}
```

### Capability Pattern

```go
type MyCap struct {
    // fields
}

func (c *MyCap) Code() Code { return CodeMyCap }
func (c *MyCap) Len() int { /* TLV value length */ }
func (c *MyCap) WriteTo(buf []byte, off int) int { /* write into buf */ }

func parseMyCap(data []byte) (*MyCap, error) { /* parse */ }
```

### Attribute Pattern

```go
type MyAttr struct {
    // fields
}

func (a *MyAttr) Code() AttributeCode { return AttrMyAttr }
func (a *MyAttr) Flags() AttributeFlags { return FlagTransitive | FlagOptional }
func (a *MyAttr) Len() int { /* payload length */ }
func (a *MyAttr) WriteTo(buf []byte, off int) int { /* write */ }
```

### NLRI Pattern

```go
type MyNLRI struct {
    family  Family
    pathID  uint32
    // fields
}

func (n *MyNLRI) Family() Family { return n.family }
func (n *MyNLRI) PathID() uint32 { return n.pathID }
func (n *MyNLRI) Bytes() []byte { /* payload only, no path ID */ }
func (n *MyNLRI) Len() int { /* payload length, no path ID */ }
func (n *MyNLRI) WriteTo(buf []byte, off int) int { /* write payload */ }
func (n *MyNLRI) SupportsAddPath() bool { return true }
```

### Test Pattern

```go
func TestMyFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *MyType
        wantErr bool
    }{
        {"valid", []byte{...}, &MyType{...}, false},
        {"invalid", []byte{...}, nil, true},
        {"boundary-min", []byte{...}, &MyType{...}, false},
        {"boundary-max", []byte{...}, &MyType{...}, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

---

## RFC Implementation Examples

| RFC | Components | Good Reference |
|-----|------------|----------------|
| RFC 4724 (GR) | Capability, FSM | `internal/core/bgp/capability/session.go` |
| RFC 7911 (ADD-PATH) | Capability, NLRI encoding | `internal/core/bgp/capability/encoding.go` |
| RFC 4760 (MP) | Capability, NLRI, Attributes | `internal/core/bgp/nlri/`, `internal/core/bgp/attribute/mpnlri.go` |
| RFC 8955 (FlowSpec) | NLRI, UPDATE builder | `internal/component/bgp/message/update_build_flowspec.go` |
| RFC 7432 (EVPN) | NLRI, UPDATE builder | `internal/component/bgp/message/update_build_evpn.go` |

<!-- source: internal/core/bgp/capability/session.go -- GR capability -->
<!-- source: internal/core/bgp/capability/encoding.go -- ADD-PATH capability -->
<!-- source: internal/core/bgp/attribute/mpnlri.go -- MP_REACH/MP_UNREACH attributes -->
