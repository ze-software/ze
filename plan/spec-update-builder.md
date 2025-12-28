# Spec: UPDATE Message Builder Pattern

## Task

Implement fluent builder pattern for UPDATE message construction with automatic attribute ordering and validation.

## Problem

Current UPDATE building involves repetitive `append()` calls:
- **Verbose** - Same pattern repeated in 10+ functions
- **Error-prone** - Easy to forget attributes or get ordering wrong
- **No validation** - Missing required attributes not caught
- **Scattered logic** - Each builder reimplements attribute handling

## Proposed Solution

```go
update, err := message.NewUpdateBuilder().
    Origin(attribute.OriginIGP).
    ASPath([]uint32{65001, 65002}).
    NextHop(netip.MustParseAddr("192.168.1.1")).
    Communities(community.NoExport).
    NLRI(prefix1, prefix2).
    Build()
```

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- RFC 4271 compliance is NON-NEGOTIABLE
- Verify byte-identical output during migration

### From RFC 4271
- Section 4.3: UPDATE Message Format
- Section 5: Path Attributes - SHOULD be ordered by type code
- Announcements REQUIRE: ORIGIN, AS_PATH, next-hop

## Codebase Context

### Files to Create

| File | Purpose |
|------|---------|
| `pkg/bgp/message/builder.go` | UpdateBuilder implementation |
| `pkg/bgp/message/builder_test.go` | Unit tests |

### Key Design Decisions

1. **Attribute Ordering:** Builder stores typed attributes, sorts by type code at `Build()`
2. **Required Validation:** `Build()` returns error if ORIGIN, AS_PATH, next-hop missing
3. **Address Family:** Auto-detects from NLRI type (IPv4 unicast vs MP families)
4. **iBGP/eBGP Context:** Builder accepts session context for defaults
5. **Message Chunking:** `Build()` returns `[]*Update` for large NLRI sets
6. **Buffer Management:** Pre-allocate based on negotiated max message size

## Implementation Steps

### Phase 1: Core Builder (TDD)
1. Write test: Builder with Origin, ASPath, NextHop, NLRI for IPv4 - MUST FAIL
2. Implement basic builder structure
3. Write test: Attribute ordering verification - MUST FAIL
4. Implement sort-by-type-code logic
5. Write test: Required attribute validation - MUST FAIL
6. Implement error returns for missing attrs
7. Run `make test`

### Phase 2: Full Attribute Support
1. Write test for each optional attribute type - MUST FAIL
2. Implement all attribute setters
3. Write test for communities handling - MUST FAIL
4. Implement community handling with dedup
5. Run `make test`

### Phase 3: MP Family Support
1. Write test: IPv6 unicast generates MP_REACH_NLRI - MUST FAIL
2. Implement family detection and MP routing
3. Write test: L3VPN, FlowSpec families - MUST FAIL
4. Implement family-specific next-hop encoding
5. Run `make test`

### Phase 4: Session Context
1. Write test: iBGP defaults (LOCAL_PREF, empty AS_PATH) - MUST FAIL
2. Implement session context methods
3. Write test: eBGP AS prepending - MUST FAIL
4. Implement AS_PATH modification logic
5. Run `make test`

### Phase 5: Message Chunking
1. Write test: Large NLRI set splits into multiple UPDATEs - MUST FAIL
2. Implement size checking and chunking
3. Write test: Extended message size support - MUST FAIL
4. Implement MaxSize configuration
5. Run `make test`

### Phase 6: Migration
1. Refactor `buildStaticRouteUpdate` to use builder
2. Write regression test comparing byte output
3. Verify byte-for-byte output match
4. Refactor remaining `build*Update` functions
5. Remove duplicate code
6. Run `make test && make lint`

## Verification Checklist

- [ ] TDD followed: Each test shown to FAIL first
- [ ] All existing `build*Update` tests pass with builder
- [ ] Byte-for-byte output compatibility verified
- [ ] Attribute ordering always correct (RFC 4271)
- [ ] Required attribute validation works
- [ ] IPv4 and IPv6 families handled correctly
- [ ] `make test` passes
- [ ] `make lint` passes

## API Design Summary

```go
// Core builder
func NewUpdateBuilder() *UpdateBuilder

// Session context
func (b *UpdateBuilder) ForSession(s *Session) *UpdateBuilder
func (b *UpdateBuilder) IBGP(ibgp bool) *UpdateBuilder
func (b *UpdateBuilder) LocalAS(as uint32) *UpdateBuilder

// Well-known mandatory
func (b *UpdateBuilder) Origin(o attribute.OriginType) *UpdateBuilder
func (b *UpdateBuilder) ASPath(path []uint32) *UpdateBuilder
func (b *UpdateBuilder) NextHop(addr netip.Addr) *UpdateBuilder

// Optional attributes
func (b *UpdateBuilder) LocalPref(pref uint32) *UpdateBuilder
func (b *UpdateBuilder) MED(med uint32) *UpdateBuilder
func (b *UpdateBuilder) Communities(comms ...uint32) *UpdateBuilder
func (b *UpdateBuilder) LargeCommunities(comms ...attribute.LargeCommunity) *UpdateBuilder

// NLRI
func (b *UpdateBuilder) NLRI(prefixes ...nlri.NLRI) *UpdateBuilder
func (b *UpdateBuilder) Withdraw(prefixes ...nlri.NLRI) *UpdateBuilder

// Build
func (b *UpdateBuilder) Build() ([]*Update, error)
```

## Success Criteria

1. All existing `build*Update` tests pass with builder
2. Byte-for-byte output compatibility verified
3. Attribute ordering always correct (RFC 4271)
4. Required attribute validation works
5. Code reduction: 10+ functions -> single builder
