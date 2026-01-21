# RFC Implementation Guide for ZeBGP

This guide provides a step-by-step checklist for implementing an RFC in ZeBGP. Use it alongside `planning.md` to ensure complete implementations.

## Overview

An RFC implementation typically touches these areas (not all apply to every RFC):

| Component | Package | When Needed |
|-----------|---------|-------------|
| Capability | `internal/bgp/capability/` | RFC introduces a capability |
| Attribute | `internal/bgp/attribute/` | RFC introduces path attributes |
| NLRI | `internal/bgp/nlri/` | RFC introduces new AFI/SAFI |
| Message | `internal/bgp/message/` | RFC modifies message format |
| FSM | `internal/bgp/fsm/` | RFC affects state machine |
| Config | `internal/config/` | RFC needs configuration |
| API | `internal/api/` | RFC needs plugin commands |
| Engine | `internal/engine/` | RFC affects reactor/peer handling |

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
[ ] Check if related capabilities exist: internal/bgp/capability/
[ ] Check if related attributes exist: internal/bgp/attribute/
[ ] Check if related NLRI types exist: internal/bgp/nlri/
[ ] Read architecture docs for affected areas (see planning.md keyword table)
```

### 0.3 ExaBGP Migration Consideration

If this RFC adds features that ExaBGP users might rely on, check if migration support is needed:

| RFC Affects | Migration Impact | Action |
|-------------|------------------|--------|
| API commands/events | ExaBGP plugins expect different JSON format | Update `internal/exabgp/bridge.go` |
| Config syntax | ExaBGP configs have different syntax | Update `internal/exabgp/migrate.go` |
| Capabilities | ExaBGP may configure differently | Check migrate.go handles it |

```
[ ] Does ExaBGP support this RFC feature?
[ ] If yes: is config migration needed? (internal/exabgp/migrate.go)
[ ] If yes: is API bridge update needed? (internal/exabgp/bridge.go)
```

See `.claude/rules/compatibility.md` for architecture details.

### 0.4 Spec Creation

```
[ ] Create spec: docs/plan/spec-rfcNNNN-<feature>.md
[ ] Fill Required Reading section with identified docs
[ ] git add the spec immediately
```

---

## Phase 1: Capability (if applicable)

**When:** RFC introduces a BGP capability (advertised in OPEN message)

### 1.1 Define Capability

```
[ ] Add capability code constant to internal/bgp/capability/codes.go
    - Code<Name> Code = NN  // RFC NNNN

[ ] Create capability struct in appropriate file (or new file)
    - encoding.go for wire-format affecting caps
    - session.go for session behavior caps
    - new file for complex capabilities

[ ] Implement Capability interface:
    - Code() Code
    - Pack() []byte
    - String() string (for debugging)

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
[ ] Unit test: Pack() round-trips correctly
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
[ ] Add attribute code constant to internal/bgp/attribute/codes.go
    - Attr<Name> AttributeCode = NN  // RFC NNNN

[ ] Create attribute struct (new file if complex, or add to existing)

[ ] Implement Attribute interface:
    - Code() AttributeCode
    - Flags() AttributeFlags
    - Len() int
    - WriteTo(buf []byte, off int) int
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
[ ] Add AFI constant if new: internal/bgp/nlri/afi.go
[ ] Add SAFI constant if new: internal/bgp/nlri/safi.go
[ ] Add Family constant: var <Name> = Family{AFI: ..., SAFI: ...}
[ ] Register in familyNames map for string parsing
```

### 3.2 Define NLRI Type

```
[ ] Create NLRI struct (new file for complex types)

[ ] Implement NLRI interface:
    - Family() Family
    - Bytes() []byte
    - Len() int
    - PathID() uint32
    - WriteTo(buf []byte, off int) int
    - String() string

[ ] If ADD-PATH supported:
    - LenWithContext(addPath bool) int
    - Use WriteNLRI() helper for writing
```

### 3.3 Wire Format

```
[ ] Document wire format with ASCII diagram
[ ] Implement constructor(s) for creating NLRI
[ ] Implement parser for wire bytes
[ ] Handle variable-length fields correctly
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
[ ] Unit test: Round-trip (create → write → parse → compare)
[ ] Unit test: ADD-PATH handling (with/without path ID)
[ ] Unit test: String() produces readable output
[ ] Boundary test: max prefix length, label values, etc.
```

---

## Phase 4: Message Changes (if applicable)

**When:** RFC modifies BGP message format or introduces new message type

### 4.1 New Message Type

```
[ ] Add message type constant to internal/bgp/message/types.go
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
[ ] Add schema nodes to internal/config/schema.go
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
[ ] Config rejects unknown keys (ZeBGP rule)
[ ] Config validates value ranges
[ ] Config validates inter-field dependencies
```

### 6.4 Tests

```
[ ] Valid config test: test/data/parse/valid/<feature>.conf
[ ] Invalid config test: test/data/parse/invalid/<feature>.conf + .expect
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
[ ] Functional test: test/data/plugin/<feature>/
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
    → Configures the feature as a user would
    → Exercises the feature through normal usage (API commands, peer interaction)
    → Verifies the observable outcome (wire bytes sent, events received, state changes)
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
[ ] Create test/data/encode/<feature>.conf (peer config with feature enabled)
[ ] Create test/data/encode/<feature>.ci (command/wire pairs)
[ ] Test happy path: feature used correctly produces correct wire bytes
[ ] Test variations: different parameter combinations
[ ] Test boundaries: min/max values that affect encoding
```

### 9.3 Plugin Tests

```
[ ] Create test/data/plugin/<feature>/ directory
[ ] Test plugin receives correct JSON events when feature is active
[ ] Test plugin commands produce correct behavior
[ ] Test error responses when plugin sends invalid commands
```

### 9.4 Config Tests

```
[ ] Create test/data/parse/valid/<feature>.conf - valid configurations
[ ] Create test/data/parse/invalid/<feature>.conf + .expect - invalid configs with expected errors
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
[ ] Add ZeBGP implementation notes section
[ ] Cross-reference related RFCs
```

### 10.3 Config Examples

```
[ ] Add example configs showing feature usage
[ ] Document in relevant architecture docs
```

---

## Final Checklist

Before marking implementation complete:

```
[ ] All unit tests pass: make test
[ ] All linting passes: make lint (zero issues)
[ ] All functional tests pass: make functional
[ ] RFC section comments on all protocol code
[ ] RFC constraint comments with quoted requirements
[ ] No backwards-compatibility shims (ZeBGP rule)
[ ] No version numbers in config (ZeBGP rule)
[ ] Architecture docs updated
[ ] Spec moved to docs/plan/done/NNN-<name>.md
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
func (x *Type) WriteToWithContext(buf []byte, off int, ctx *EncodingContext) int {
    // Use ctx for ASN4, ADD-PATH decisions
}
```

### Capability Pattern

```go
type MyCap struct {
    // fields
}

func (c *MyCap) Code() Code { return CodeMyCap }
func (c *MyCap) Pack() []byte { /* wire bytes */ }

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
func (n *MyNLRI) Len() int { /* payload length, no path ID */ }
func (n *MyNLRI) WriteTo(buf []byte, off int) int { /* write */ }
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
| RFC 4724 (GR) | Capability, FSM | `internal/bgp/capability/session.go` |
| RFC 7911 (ADD-PATH) | Capability, NLRI encoding | `internal/bgp/capability/encoding.go` |
| RFC 4760 (MP) | Capability, NLRI, Attributes | `internal/bgp/nlri/`, `internal/bgp/attribute/mpreach.go` |
| RFC 8955 (FlowSpec) | NLRI | `internal/bgp/nlri/flowspec.go` |
| RFC 7432 (EVPN) | NLRI | `internal/bgp/nlri/evpn.go` |
