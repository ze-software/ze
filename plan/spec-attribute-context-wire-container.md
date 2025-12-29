# Spec: Unified Attribute Packing Context and Wire Container

## Task

Two related architectural improvements:

1. **PackWithContext**: Add `PackWithContext(ctx *nlri.PackContext) []byte` to `Attribute` interface so attributes like ASPath and Aggregator can encode based on peer capabilities (ASN4, AddPath) without separate methods.

2. **Wire Container**: Add wire-format container (like ExaBGP's `Attributes` class) to store raw packed bytes as canonical representation, enabling zero-copy route reflection and lazy parsing.

## Current State (verified)

```
🔍 Verified test status: 23 passed, 14 failed
📋 Functional tests: 0, 6, 7, 8, J, L, N, Q, S, T, U, V, Z, a
📋 Last commit: 9c94a2b (static route UpdateBuilder conversion)
```

## Problem Analysis

### Problem 1: Context-Free Pack()

Current `Attribute` interface:
```go
type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte  // No peer context!
}
```

**Issues:**
- `ASPath.Pack()` hardcodes 4-byte ASN format
- Callers must use `PackWithASN4(bool)` separately
- `Aggregator.Pack()` always uses 8-byte format (4-byte ASN)
- For 2-byte ASN peers, must generate AS4_PATH/AS4_AGGREGATOR separately
- No single interface method handles capability-aware encoding

**Affected attributes:**
| Attribute | Context Needed | Current Workaround |
|-----------|----------------|-------------------|
| AS_PATH | ASN4 | `PackWithASN4(bool)` |
| AGGREGATOR | ASN4 | None (always 8-byte) |
| AS4_PATH | None | Generated separately |
| AS4_AGGREGATOR | None | Generated separately |

### Problem 2: No Wire Container

Current flow:
```
Config/API → Semantic objects → Pack to bytes → Send
Receive → Parse to semantic → Store semantic → Re-pack to send
```

For route reflection, this is inefficient:
```
Receive bytes → Parse to semantic → Store → Re-pack to bytes → Send
              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
              Unnecessary parse/repack for unchanged routes
```

Optimal flow (ExaBGP pattern):
```
Receive bytes → Store raw bytes → Send raw bytes (zero-copy)
             → Parse lazily only when modification needed
```

## Design

### Part 1: PackWithContext

**New interface method:**
```go
type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    Pack() []byte  // Keep for backward compat, defaults to ASN4=true

    // PackWithContext serializes with peer capability context.
    // RFC 6793: ctx.ASN4 determines 2-byte vs 4-byte AS encoding.
    PackWithContext(ctx *nlri.PackContext) []byte
}
```

**Implementation approach:**
- Default `Pack()` calls `PackWithContext(&nlri.PackContext{ASN4: true})`
- `PackWithContext()` uses `ctx.ASN4` to determine encoding
- Deprecate `PackWithASN4()` after migration

**ASPath changes:**
```go
func (p *ASPath) PackWithContext(ctx *nlri.PackContext) []byte {
    asn4 := ctx != nil && ctx.ASN4
    return p.packInternal(asn4)
}

func (p *ASPath) Pack() []byte {
    return p.PackWithContext(&nlri.PackContext{ASN4: true})
}

// Deprecated: Use PackWithContext instead.
func (p *ASPath) PackWithASN4(asn4 bool) []byte {
    return p.packInternal(asn4)
}
```

**Aggregator changes:**
```go
func (a *Aggregator) PackWithContext(ctx *nlri.PackContext) []byte {
    if ctx != nil && !ctx.ASN4 {
        // 2-byte ASN format (6 bytes total)
        asn := a.ASN
        if asn > 65535 {
            asn = 23456 // AS_TRANS
        }
        buf := make([]byte, 6)
        binary.BigEndian.PutUint16(buf[0:2], uint16(asn))
        copy(buf[2:6], a.Address.AsSlice())
        return buf
    }
    // 4-byte ASN format (8 bytes)
    buf := make([]byte, 8)
    binary.BigEndian.PutUint32(buf[0:4], a.ASN)
    copy(buf[4:8], a.Address.AsSlice())
    return buf
}
```

**Simple attributes (no change needed):**
```go
func (o Origin) PackWithContext(ctx *nlri.PackContext) []byte {
    return o.Pack() // No context dependency
}
```

### Part 2: Wire Container (AttributesWire)

**New type in `pkg/bgp/attribute/`:**
```go
// AttributesWire is a wire-format path attributes container.
//
// Stores raw packed attribute bytes as the canonical representation.
// Enables zero-copy route reflection and lazy parsing.
//
// RFC 4271 Section 4.3 - Path attributes are variable-length TLV sequences.
type AttributesWire struct {
    packed []byte                    // Raw wire bytes (canonical)
    ctx    *nlri.PackContext         // Context for parsing
    parsed map[AttributeCode]Attribute // Lazy-parsed cache (nil until needed)
}

// NewAttributesWire creates from raw packed bytes.
func NewAttributesWire(packed []byte, ctx *nlri.PackContext) *AttributesWire {
    return &AttributesWire{packed: packed, ctx: ctx}
}

// FromAttributes creates wire container from semantic slice.
func FromAttributes(attrs []Attribute, ctx *nlri.PackContext) *AttributesWire {
    packed := PackAttributesOrderedWithContext(attrs, ctx)
    return &AttributesWire{packed: packed, ctx: ctx}
}

// Packed returns raw wire bytes for transmission.
// Zero-copy for route reflection when attributes unchanged.
func (a *AttributesWire) Packed() []byte {
    return a.packed
}

// Get returns a specific attribute by code (lazy parse).
func (a *AttributesWire) Get(code AttributeCode) (Attribute, bool) {
    if a.parsed == nil {
        a.parse()
    }
    attr, ok := a.parsed[code]
    return attr, ok
}

// Has checks if attribute exists without full parse.
func (a *AttributesWire) Has(code AttributeCode) bool {
    // Scan headers only, don't parse values
    return a.scanFor(code)
}

// ToSemantic converts to semantic slice for modification.
// After modification, use FromAttributes to create new wire container.
func (a *AttributesWire) ToSemantic() []Attribute {
    if a.parsed == nil {
        a.parse()
    }
    result := make([]Attribute, 0, len(a.parsed))
    for _, attr := range a.parsed {
        result = append(result, attr)
    }
    sort.Slice(result, func(i, j int) bool {
        return result[i].Code() < result[j].Code()
    })
    return result
}
```

**Usage in route reflection:**
```go
// Before: Parse, store semantic, repack
update := ParseUpdate(data)
attrs := update.ParseAttributes(ctx)  // Parse all
// ... store attrs ...
packed := PackAttributes(attrs, ctx)   // Repack

// After: Store wire bytes, forward as-is
update := ParseUpdate(data)
attrsWire := NewAttributesWire(update.PathAttributes, ctx)
// ... store attrsWire ...
packed := attrsWire.Packed()  // Zero-copy!
```

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference

### From TDD_ENFORCEMENT.md
- Tests MUST exist and FAIL before implementation begins
- Every test MUST document VALIDATES and PREVENTS
- Show test failure output before implementation
- Show test pass output after implementation
- Paste exact output, no summaries

### From ESSENTIAL_PROTOCOLS.md (Refactoring)
- ONE function/type at a time, no batching
- Announce each step, complete ONLY that step
- Run verification after each step
- Stop if any failures

### RFC References
- RFC 4271 Section 4.3: Path attribute format (TLV)
- RFC 6793 Section 4: AS number encoding (2-byte vs 4-byte)
- RFC 4271 Appendix F.3: Attribute ordering (ascending by type code)

## Codebase Context

### Existing Files to Understand
- `pkg/bgp/attribute/attribute.go` - Current interface
- `pkg/bgp/attribute/aspath.go` - ASPath with PackWithASN4
- `pkg/bgp/attribute/aggregator.go` - Aggregator (always 8-byte)
- `pkg/bgp/attribute/origin.go` - PackAttribute helper
- `pkg/bgp/message/update_build.go` - Uses attribute.PackAttributesOrdered
- `pkg/bgp/nlri/pack.go` - PackContext definition

### ExaBGP Reference
- `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/message/update/attribute/collection.py`
  - `AttributeCollection` (semantic) vs `Attributes` (wire)
  - `pack_attribute(negotiated)` pattern
  - `for code in sorted(alls)` ordering

## Implementation Steps

### Phase 1: PackWithContext (Interface Change)

**Step 1.1: Add interface method**
- File: `pkg/bgp/attribute/attribute.go`
- Add `PackWithContext(ctx *nlri.PackContext) []byte` to interface
- Add `PackAttributesOrderedWithContext()` helper

**Step 1.2: Implement for simple attributes**
- Files: `pkg/bgp/attribute/simple.go`, `origin.go`, `nexthop.go`, etc.
- Default: `return a.Pack()` (no context dependency)

**Step 1.3: Implement for ASPath**
- File: `pkg/bgp/attribute/aspath.go`
- Use `ctx.ASN4` to select encoding
- Keep `PackWithASN4()` for backward compat (deprecated)

**Step 1.4: Implement for Aggregator**
- File: `pkg/bgp/attribute/aggregator.go`
- Add 6-byte format for 2-byte ASN peers
- Use AS_TRANS (23456) for large ASNs

**Step 1.5: Update callers**
- File: `pkg/bgp/message/update_build.go`
- Use `PackAttributesOrderedWithContext(attrs, ctx)`

### Phase 2: Wire Container

**Step 2.1: Create AttributesWire type**
- File: `pkg/bgp/attribute/wire.go`
- Implement `NewAttributesWire`, `FromAttributes`, `Packed`

**Step 2.2: Add lazy parsing**
- Implement `Get()`, `Has()`, `parse()`, `scanFor()`
- Use existing `ParseHeader` for scanning

**Step 2.3: Add ToSemantic conversion**
- Implement `ToSemantic()` for modification scenarios

**Step 2.4: Integrate with Update**
- Update `Update` struct to optionally use `AttributesWire`
- Maintain backward compatibility with `[]byte` field

## Test Specifications

### PackWithContext Tests

```go
// TestASPathPackWithContext_ASN4 verifies 4-byte ASN encoding.
//
// VALIDATES: RFC 6793 Section 4.1 - AS numbers as 4-octet entities.
//
// PREVENTS: Sending 4-byte ASNs to 2-byte-only peers causing parse errors.
func TestASPathPackWithContext_ASN4(t *testing.T)

// TestASPathPackWithContext_ASN2 verifies 2-byte ASN encoding with AS_TRANS.
//
// VALIDATES: RFC 6793 Section 4.2.2 - AS_TRANS for non-mappable ASNs.
//
// PREVENTS: Protocol errors when communicating with legacy peers.
func TestASPathPackWithContext_ASN2(t *testing.T)

// TestAggregatorPackWithContext_ASN4 verifies 8-byte Aggregator.
//
// VALIDATES: RFC 6793 - 4-byte ASN in Aggregator when negotiated.
func TestAggregatorPackWithContext_ASN4(t *testing.T)

// TestAggregatorPackWithContext_ASN2 verifies 6-byte Aggregator with AS_TRANS.
//
// VALIDATES: RFC 6793 Section 4.2.2 - Aggregator encoding for legacy peers.
func TestAggregatorPackWithContext_ASN2(t *testing.T)
```

### Wire Container Tests

```go
// TestAttributesWirePacked verifies zero-copy access to wire bytes.
//
// VALIDATES: Wire bytes are returned without modification.
//
// PREVENTS: Unnecessary repacking when forwarding unchanged routes.
func TestAttributesWirePacked(t *testing.T)

// TestAttributesWireGet verifies lazy parsing of single attribute.
//
// VALIDATES: Specific attribute can be retrieved without parsing all.
//
// PREVENTS: O(n) parse when only O(1) attribute access needed.
func TestAttributesWireGet(t *testing.T)

// TestAttributesWireFromAttributes verifies semantic-to-wire conversion.
//
// VALIDATES: Semantic attributes pack correctly with context.
//
// PREVENTS: Encoding mismatches between semantic and wire representations.
func TestAttributesWireFromAttributes(t *testing.T)
```

## Verification Checklist

### Phase 1: PackWithContext
- [ ] Tests written for ASPath with ASN4=true
- [ ] Tests written for ASPath with ASN4=false
- [ ] Tests written for Aggregator with ASN4=true
- [ ] Tests written for Aggregator with ASN4=false
- [ ] Tests FAIL before implementation
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Existing callers work (backward compat)

### Phase 2: Wire Container
- [ ] Tests written for Packed()
- [ ] Tests written for Get()
- [ ] Tests written for FromAttributes()
- [ ] Tests FAIL before implementation
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes

### Final
- [ ] All functional tests still pass (23+)
- [ ] No regressions in existing behavior
- [ ] Self-review completed

## Decision Points (ASK USER)

1. **Phase 2 Priority:** Wire container is lower priority than PackWithContext. Implement Phase 2 only if user confirms.

2. **Legacy method deprecation:** Should `PackWithASN4()` be:
   - (a) Deprecated with comment, keep working
   - (b) Removed after migration
   - (c) Keep indefinitely

3. **Aggregator AS4_AGGREGATOR:** When ASN4=false and ASN>65535:
   - (a) Return 6-byte with AS_TRANS, caller handles AS4_AGGREGATOR
   - (b) Return both AGGREGATOR + AS4_AGGREGATOR in same call
   - (c) Return error

---

**Created:** 2025-12-28
**Status:** Ready for implementation
