# Spec: Unified Handle + 4-Byte NLRI Design

> **📍 LOCATION CHANGE:** This spec remains valid, but the pool now lives in
> **API programs** instead of the ZeBGP engine. Use this design when implementing
> RIB storage in API programs (Go: `pkg/rib/`, Python/Rust: implement equivalent).
> See `docs/architecture/rib-transition.md` for the overall architecture.

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. docs/plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. internal/pool/*.go, pkg/bgp/nlri/*.go                       │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Redesign pool.Handle and NLRI structs for minimal memory footprint:
- Extend `pool.Handle` to encode: `poolIdx(6) | flags(2) | slot(24)`
- Modify `Pool` to store `idx` and extract slot from handles
- NLRI types wrap `Handle` with 4-byte struct
- Context has `[63]*Pool` registry indexed by `Handle.PoolIdx()`
- Family derived from `poolIdx`, length from `pool.Length(handle)`
- Store `[]pool.Handle` in RIB, reconstruct typed NLRIs on demand

## Current State (verified 2025-12-28)

- **Functional tests:** 24 passed, 13 failed
- **Failing:** 0, 7, 8, J, L, N, Q, S, T, U, V, Z, a
- **Last commit:** `5a5c5a8` (docs: update plans)
- **Untracked files:** `pkg/bgp/message/update_build.go`, `update_build_test.go`

---

## Design Transition Alignment

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

### Role in Pool + Wire Design

This spec is **essential** for the Pool + Wire design:

| Component | Role in Design |
|-----------|---------------|
| Handle with PoolIdx | Identifies which pool owns the data |
| 4-byte NLRI structs | Memory-efficient route storage |
| `Ctx` with pools | Central pool registry for all families |
| `HandleToNLRI()` | Reconstruct typed NLRI from handle |

### Updated Relationship to Other Specs

| Spec | Relationship |
|------|-------------|
| `spec-pool-handle-migration.md` | **COMPLEMENTARY** - single UPDATE pool, derived accessors |
| `spec-attributes-wire.md` | **COMPLEMENTARY** - that wraps attr bytes, this wraps NLRI bytes |

### Execution Order

```
1. spec-pool-handle-migration.md  ← Single UPDATE pool, Route.Attrs()/NLRI() derived
        ↓
2. This spec (unified-handle-nlri) ← Optional: NLRI dedup if needed
```

---

## Design Decisions (Reviewed 2025-12-28)

| Issue | Decision |
|-------|----------|
| InvalidHandle sentinel | Reserve poolIdx=63, use 0-62 (63 pools sufficient) |
| Circular import | Free functions in nlri package, not methods on Handle |
| Interface storage overhead | Store `[]pool.Handle`, reconstruct typed NLRI on demand |
| Breaking NLRI interface | Accepted - methods take `*Ctx` parameter |
| Ctx ownership | Created in main, passed through all call chains |
| Pool API | Change `New(capacity)` to `New(idx, capacity)` (breaking) |
| Release semantics | Document: NLRIs must not be copied; if copied, don't release copy |
| NLRIHashable.Key | Takes `*Ctx` parameter (currently uses wrapper pattern) |
| Slot limit (16M) | Sufficient - global BGP table ~1M prefixes (April 2025) |

---

## Current Implementation Review (2025-12-28)

### Current NLRI Interface (`pkg/bgp/nlri/nlri.go`)

```go
type NLRI interface {
    Family() Family
    Bytes() []byte                    // Raw wire format
    Pack(ctx *PackContext) []byte     // Capability-aware encoding
    Len() int
    String() string
    PathID() uint32
    HasPathID() bool                  // Note: NOT HasPath()
}
```

### Current NLRI Struct Pattern

```go
type INET struct {
    family  Family        // 4 bytes (AFI uint16 + SAFI uint8 + padding)
    prefix  netip.Prefix  // 24 bytes
    pathID  uint32        // 4 bytes
    hasPath bool          // 1 byte + padding
}
// Total: ~40+ bytes per NLRI
```

### Current Pool API (`internal/pool/pool.go`)

```go
func New(initialCapacity int) *Pool  // No idx parameter yet!

type Handle uint32
const InvalidHandle Handle = 0xFFFFFFFF

func (h Handle) Valid() bool    // Just checks != InvalidHandle
func (h Handle) String() string
// No PoolIdx(), Flags(), Slot() methods yet
```

### Current NLRIHashable Pattern (`internal/store/nlri.go` + `pkg/rib/store.go`)

```go
// Interface in store package
type NLRIHashable interface {
    Key() []byte       // No Ctx parameter currently
    FamilyKey() uint32
}

// Wrapper in rib package (NLRI types don't implement directly)
type hashableNLRI struct {
    n nlri.NLRI
}

func (h hashableNLRI) Key() []byte {
    return h.n.Pack(nil)  // Uses Pack(nil) for dedup key
}

func (h hashableNLRI) FamilyKey() uint32 {
    f := h.n.Family()
    return uint32(f.AFI)<<16 | uint32(f.SAFI)
}
```

### Key Observations

1. **Method naming:** Current uses `HasPathID()`, not `HasPath()`
2. **NLRIHashable wrapper:** NLRI types don't implement NLRIHashable directly
3. **Key() without Ctx:** Current `Key()` calls `Pack(nil)` - works without pool access
4. **Large structs:** Current INET is ~40+ bytes, new design targets 4 bytes
5. **No idx in Pool:** Must add idx parameter to `New()`

---

## Embedded Protocol Requirements

### Default Rules (ALL tasks)

- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `docs/plan/CLAUDE_CONTINUATION.md` for current state
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md

- **TDD Blocking Rule:** Tests MUST exist and FAIL before implementation
- **Verification:** Never claim success without proof (run command, paste output)
- **Work Preservation:** NEVER discard uncommitted work without permission
- **Self-Review:** After completion, perform critical review, fix issues, re-review
- **Refactoring:** ONE function/type at a time, verify after each step

### From TDD_ENFORCEMENT.md

- Every test MUST document: VALIDATES (what behavior), PREVENTS (what bug)
- Show test FAILURE before implementation
- Show test PASS after implementation
- Fuzz tests MANDATORY for wire format parsing code

### From CODING_STANDARDS.md

- Go 1.21+ required (slog, generics)
- Error handling: NEVER ignore errors
- No panic() for normal errors
- No global mutable state (use explicit dependency injection)
- Context passed through call chains

---

## Design Summary

### Handle Bit Layout (32 bits)

```
┌──────────┬───────┬────────────────────────┐
│PoolIdx  │ Flags │        Slot            │
│ (6 bits)│(2 bit)│      (24 bits)         │
└──────────┴───────┴────────────────────────┘
 31     26 25   24 23                      0

PoolIdx: 0-62 valid, 63 reserved for InvalidHandle
Flags:   Bit 0 = hasPathID (ADD-PATH present), Bit 1 = reserved
Slot:    0 to 16,777,214 (0xFFFFFE), 0xFFFFFF reserved
```

- **InvalidHandle:** `0xFFFFFFFF` (poolIdx=63, flags=3, slot=0xFFFFFF)
- **Max pools:** 63 (sufficient for all NLRI families + attributes)
- **Max slots per pool:** ~16M (sufficient for global BGP table ~1M prefixes)

### Memory Layout

| Component | Current | New | Savings |
|-----------|---------|-----|---------|
| Handle | 4 bytes | 4 bytes | - |
| NLRIData | varies (12-24 bytes) | 4 bytes | 66-83% |
| INET struct | ~24 bytes | 4 bytes | 83% |
| 1M routes | ~24 MB | 4 MB | 83% |

### Key Insight: Remove Redundant Fields

| Field | Current | New | Rationale |
|-------|---------|-----|-----------|
| `offset` | Stored | **Removed** | Each NLRI interned separately, offset always 0 |
| `length` | Stored | **From Pool** | `pool.Length(handle)` already exists |
| `family` | Stored | **From PoolIdx** | `poolIdxToFamily[handle.PoolIdx()]` |
| `flags/hasPath` | Stored | **In Handle** | Encoded in bits 25-24 |

### Storage Strategy

Store raw handles, reconstruct typed NLRIs on demand:

```go
// RIB stores handles directly (4 bytes each)
type FamilyRIB struct {
    handles []pool.Handle  // Compact storage
}

// Reconstruct typed NLRI when needed
func (r *FamilyRIB) Get(i int) NLRI {
    return HandleToNLRI(r.handles[i])
}
```

This avoids Go interface overhead (16 bytes per `[]NLRI` element).

---

## Codebase Context

### Files to Modify

**Pool layer (Phase 1-2):**
1. **`internal/pool/handle.go`** - Extend Handle with bit encoding
2. **`internal/pool/pool.go`** - Add `idx` field, update methods to extract slot

**NLRI layer (Phase 3-6):**
3. **`pkg/bgp/nlri/context.go`** - New file: Ctx, free functions, family mapping
4. **`pkg/bgp/nlri/nlri.go`** - Redefine NLRI interface with Ctx parameter
5. **`pkg/bgp/nlri/inet.go`** - INET wraps Handle, type-specific accessors
6. **`pkg/bgp/nlri/ipvpn.go`** - IPVPN wraps Handle
7. **`pkg/bgp/nlri/evpn.go`** - EVPN wraps Handle
8. **`pkg/bgp/nlri/flowspec.go`** - FlowSpec wraps Handle
9. **`pkg/bgp/nlri/bgpls.go`** - BGPLS wraps Handle
10. **`pkg/bgp/nlri/other.go`** - MVPN, VPLS, RTC, MUP wrap Handle

**Store integration (after pool-integration done):**
11. **`internal/store/nlri.go`** - Update NLRIHashable.Key to take Ctx
12. **`pkg/rib/store.go`** - RouteStore owns Ctx, update hashableNLRI

### Existing Patterns to Follow

- Pool already has slot-based storage with refcounting
- NLRI types already use interface polymorphism
- Context pattern used elsewhere for dependency injection

---

## Implementation Steps

### Phase 1: Handle Encoding (internal/pool)

#### Step 1.1: Write Handle encoding tests

**Test file:** `internal/pool/handle_test.go`

```go
// TestHandleEncoding verifies bit-level encoding of poolIdx, flags, slot.
//
// VALIDATES: Handle correctly stores and retrieves all three fields.
//
// PREVENTS: Bit masking errors causing field corruption or overlap.
func TestHandleEncoding(t *testing.T) {
    tests := []struct {
        poolIdx uint8
        flags   uint8
        slot    uint32
    }{
        {0, 0, 0},
        {62, 3, 0xFFFFFE},  // Max valid values
        {31, 1, 0x800000},  // Mid values
    }
    for _, tt := range tests {
        h := NewHandle(tt.poolIdx, tt.flags, tt.slot)
        require.Equal(t, tt.poolIdx, h.PoolIdx())
        require.Equal(t, tt.flags, h.Flags())
        require.Equal(t, tt.slot, h.Slot())
    }
}

// TestHandleInvalidHandle verifies InvalidHandle sentinel behavior.
//
// VALIDATES: InvalidHandle uses reserved poolIdx=63.
//
// PREVENTS: Collision between valid handles and InvalidHandle.
func TestHandleInvalidHandle(t *testing.T) {
    require.False(t, InvalidHandle.Valid())
    require.Equal(t, uint8(63), InvalidHandle.PoolIdx())  // Reserved

    // Any handle with poolIdx < 63 is valid
    h := NewHandle(0, 0, 0)
    require.True(t, h.Valid())

    h = NewHandle(62, 3, 0xFFFFFE)
    require.True(t, h.Valid())
}

// TestHandleWithFlags verifies flag modification preserves other fields.
func TestHandleWithFlags(t *testing.T) {
    h := NewHandle(5, 0, 1000)
    h2 := h.WithFlags(1)

    require.Equal(t, uint8(5), h2.PoolIdx())   // Preserved
    require.Equal(t, uint32(1000), h2.Slot())  // Preserved
    require.Equal(t, uint8(1), h2.Flags())     // Changed
    require.True(t, h2.HasPathID())
}
```

#### Step 1.2: Implement Handle encoding

**File:** `internal/pool/handle.go`

```go
type Handle uint32

// InvalidHandle uses reserved poolIdx=63.
// Valid poolIdx range: 0-62 (63 pools max).
const InvalidHandle Handle = 0xFFFFFFFF

func (h Handle) PoolIdx() uint8   { return uint8(h >> 26) }
func (h Handle) Flags() uint8     { return uint8((h >> 24) & 0x3) }
func (h Handle) Slot() uint32     { return uint32(h) & 0x00FFFFFF }
func (h Handle) HasPathID() bool  { return h.Flags()&1 != 0 }  // Match existing NLRI method name
func (h Handle) Valid() bool      { return h.PoolIdx() < 63 }

func NewHandle(poolIdx uint8, flags uint8, slot uint32) Handle {
    return Handle(
        uint32(poolIdx&0x3F)<<26 |
        uint32(flags&0x3)<<24 |
        (slot & 0x00FFFFFF),
    )
}

func (h Handle) WithFlags(flags uint8) Handle {
    return Handle((uint32(h) & 0xFCFFFFFF) | uint32(flags&0x3)<<24)
}
```

### Phase 2: Pool Modifications (internal/pool)

#### Step 2.1: Write Pool idx tests

**Test file:** `internal/pool/pool_test.go`

```go
// TestPoolIdxEncoding verifies Pool embeds idx in returned handles.
//
// VALIDATES: Intern returns handles with correct poolIdx encoded.
//
// PREVENTS: Wrong pool lookup when multiple pools exist.
func TestPoolIdxEncoding(t *testing.T) {
    p := New(5, 1024)  // idx=5
    h := p.Intern([]byte("test"))
    require.Equal(t, uint8(5), h.PoolIdx())
    require.True(t, h.Valid())
}

// TestPoolExtractsSlot verifies Pool methods use slot portion of handle.
//
// VALIDATES: Get/Length/Release work with encoded handles.
//
// PREVENTS: Using full handle as slot index (would be wrong offset).
func TestPoolExtractsSlot(t *testing.T) {
    p := New(5, 1024)
    h := p.Intern([]byte("hello"))

    // Get works with encoded handle
    require.Equal(t, []byte("hello"), p.Get(h))

    // Length works
    require.Equal(t, 5, p.Length(h))

    // WithFlags doesn't break access
    h2 := h.WithFlags(1)
    require.Equal(t, []byte("hello"), p.Get(h2))
}

// TestPoolIdxValidation verifies pool rejects invalid idx.
func TestPoolIdxValidation(t *testing.T) {
    require.Panics(t, func() {
        New(63, 1024)  // Reserved idx
    })
}
```

#### Step 2.2: Implement Pool idx support

**File:** `internal/pool/pool.go`

```go
type Pool struct {
    idx uint8  // This pool's index for handle encoding (0-62)
    // ... existing fields
}

// New creates a pool with the given index and initial capacity.
// idx must be 0-62 (63 is reserved for InvalidHandle).
func New(idx uint8, initialCapacity int) *Pool {
    if idx >= 63 {
        panic("pool idx must be 0-62, 63 is reserved")
    }
    // ... existing initialization
    return &Pool{
        idx:   idx,
        data:  make([]byte, 0, initialCapacity),
        slots: make([]slot, 0, 64),
        index: make(map[string]Handle, 64),
    }
}

func (p *Pool) Intern(data []byte) Handle {
    // ... existing dedup logic, allocate slot ...
    slot := p.allocateSlot(data)
    return NewHandle(p.idx, 0, slot)
}

func (p *Pool) Get(h Handle) []byte {
    slot := h.Slot()
    s := &p.slots[slot]
    return p.data[s.offset : s.offset+uint32(s.length)]
}

func (p *Pool) Length(h Handle) int {
    return int(p.slots[h.Slot()].length)
}

func (p *Pool) Release(h Handle) {
    slot := h.Slot()
    // ... existing release logic using slot ...
}
```

### Phase 3: Free Functions and Context (pkg/bgp/nlri)

#### Step 3.1: Define Ctx and free functions

**File:** `pkg/bgp/nlri/context.go` (new file)

```go
package nlri

import "zebgp/internal/pool"

// Ctx provides pool access for NLRI operations.
// Owned by RouteStore, passed through call chains.
// Created after pool-integration wires RouteStore.
type Ctx struct {
    Pools [63]*pool.Pool  // Indexed by Handle.PoolIdx()
}

// NewCtx creates a context with pools for all families.
// Called by RouteStore during initialization.
func NewCtx() *Ctx {
    c := &Ctx{}
    for family, idx := range familyToPoolIdx {
        c.Pools[idx] = pool.New(idx, initialCapacity(family))
    }
    // Attribute pools (60-61) created on demand or by RouteStore
    return c
}

// --- Family Mapping ---

var familyToPoolIdx = map[Family]uint8{
    IPv4Unicast:   0,
    IPv6Unicast:   1,
    IPv4Multicast: 2,
    IPv6Multicast: 3,
    IPv4VPN:       4,
    IPv6VPN:       5,
    L2VPNEVPN:     6,
    IPv4FlowSpec:  7,
    IPv6FlowSpec:  8,
    // ... more families up to 62
}

var poolIdxToFamily [63]Family

func init() {
    for f, idx := range familyToPoolIdx {
        poolIdxToFamily[idx] = f
    }
}

// --- Free Functions (no circular import) ---

// Family returns the NLRI family from handle's poolIdx.
func Family(h pool.Handle) Family {
    return poolIdxToFamily[h.PoolIdx()]
}

// Raw returns the NLRI bytes from the pool.
func Raw(c *Ctx, h pool.Handle) []byte {
    return c.Pools[h.PoolIdx()].Get(h)
}

// Len returns the NLRI byte length.
func Len(c *Ctx, h pool.Handle) int {
    return c.Pools[h.PoolIdx()].Length(h)
}

// Release decrements the handle's refcount.
// WARNING: NLRIs must not be copied. If copied, do not release the copy.
func Release(c *Ctx, h pool.Handle) {
    c.Pools[h.PoolIdx()].Release(h)
}

// HasPathID returns true if ADD-PATH path ID is present.
func HasPathID(h pool.Handle) bool {
    return h.HasPathID()
}

// Intern stores NLRI bytes in the appropriate pool.
func Intern(c *Ctx, family Family, data []byte, hasPathID bool) pool.Handle {
    idx := familyToPoolIdx[family]
    h := c.Pools[idx].Intern(data)
    if hasPathID {
        h = h.WithFlags(1)
    }
    return h
}

// HandleToNLRI reconstructs a typed NLRI from a handle.
func HandleToNLRI(h pool.Handle) NLRI {
    family := Family(h)
    switch {
    case family.IsINET():
        return INET{H: h}
    case family.IsIPVPN():
        return IPVPN{H: h}
    case family.IsEVPN():
        return EVPN{H: h}
    case family.IsFlowSpec():
        return FlowSpec{H: h}
    case family.IsBGPLS():
        return BGPLS{H: h}
    case family.IsMVPN():
        return MVPN{H: h}
    default:
        return Generic{H: h}
    }
}
```

### Phase 4: NLRI Types (4 bytes each)

#### Step 4.1: Write NLRI size tests

**Test file:** `pkg/bgp/nlri/nlri_test.go`

```go
// TestNLRIHandleSize verifies NLRI structs are exactly 4 bytes.
//
// VALIDATES: Memory efficiency goal achieved.
//
// PREVENTS: Accidental field additions bloating struct size.
func TestNLRIHandleSize(t *testing.T) {
    require.Equal(t, 4, int(unsafe.Sizeof(INET{})))
    require.Equal(t, 4, int(unsafe.Sizeof(IPVPN{})))
    require.Equal(t, 4, int(unsafe.Sizeof(EVPN{})))
    require.Equal(t, 4, int(unsafe.Sizeof(FlowSpec{})))
    require.Equal(t, 4, int(unsafe.Sizeof(BGPLS{})))
    require.Equal(t, 4, int(unsafe.Sizeof(MVPN{})))
}
```

#### Step 4.2: Implement NLRI types

**File:** `pkg/bgp/nlri/types.go`

```go
package nlri

import "zebgp/internal/pool"

// NLRI types wrap Handle - 4 bytes each.
// WARNING: Do not copy NLRI values. If copied, do not release the copy.

type INET struct{ H pool.Handle }
type IPVPN struct{ H pool.Handle }
type EVPN struct{ H pool.Handle }
type FlowSpec struct{ H pool.Handle }
type BGPLS struct{ H pool.Handle }
type MVPN struct{ H pool.Handle }
type VPLS struct{ H pool.Handle }
type RTC struct{ H pool.Handle }
type MUP struct{ H pool.Handle }
type Generic struct{ H pool.Handle }

// --- INET Methods ---

func (i INET) Family() Family        { return Family(i.H) }
func (i INET) Raw(c *Ctx) []byte     { return Raw(c, i.H) }
func (i INET) Len(c *Ctx) int        { return Len(c, i.H) }
func (i INET) HasPathID() bool       { return HasPathID(i.H) }
func (i INET) Release(c *Ctx)        { Release(c, i.H) }

func (i INET) Prefix(c *Ctx) netip.Prefix {
    raw := i.Raw(c)
    off := 0
    if i.HasPathID() { off = 4 }
    prefixLen := int(raw[off])
    prefixBytes := (prefixLen + 7) / 8
    // Build netip.Prefix...
}

func (i INET) PathID(c *Ctx) uint32 {
    if !i.HasPathID() { return 0 }
    return binary.BigEndian.Uint32(i.Raw(c)[:4])
}

// Pack returns wire format, adapting for ADD-PATH if needed.
func (i INET) Pack(c *Ctx, pc *PackContext) []byte {
    raw := i.Raw(c)
    if pc == nil || pc.AddPath == i.HasPathID() {
        return raw  // Zero-copy
    }
    if pc.AddPath {
        buf := make([]byte, 4+len(raw))
        copy(buf[4:], raw)
        return buf
    }
    return raw[4:]
}

// Key returns bytes for NLRIHashable (takes Ctx).
func (i INET) Key(c *Ctx) []byte {
    return i.Raw(c)
}

// --- Similar methods for IPVPN, EVPN, etc. ---
```

### Phase 5: Updated NLRI Interface

**File:** `pkg/bgp/nlri/interface.go`

```go
package nlri

// NLRI is the interface for all NLRI types.
// All methods that access raw data require *Ctx.
type NLRI interface {
    Family() Family
    HasPathID() bool                   // Match existing method name
    PathID(c *Ctx) uint32
    Raw(c *Ctx) []byte
    Len(c *Ctx) int
    Pack(c *Ctx, pc *PackContext) []byte
    Key(c *Ctx) []byte                 // For NLRIHashable
    Release(c *Ctx)
    String(c *Ctx) string              // Needs Ctx for raw bytes
}

// NLRIHashable for deduplication stores.
type NLRIHashable interface {
    Key(c *Ctx) []byte
    FamilyKey() uint32
}
```

### Phase 6: Parsing Integration

```go
// ParseINET parses an INET NLRI and interns it.
func ParseINET(c *Ctx, family Family, data []byte, hasPathID bool) (INET, int) {
    off := 0
    if hasPathID { off = 4 }
    prefixLen := int(data[off])
    length := off + 1 + (prefixLen+7)/8

    h := Intern(c, family, data[:length], hasPathID)
    return INET{H: h}, length
}
```

---

## Verification Checklist

### Per-Phase Verification

- [ ] **Phase 1:** Handle encoding
  - [ ] Tests written for NewHandle, accessors, edge cases
  - [ ] Tests shown to FAIL
  - [ ] Implementation written
  - [ ] Tests shown to PASS
  - [ ] `go test -race ./internal/pool/...` passes

- [ ] **Phase 2:** Pool idx support
  - [ ] Tests written for Pool with idx
  - [ ] Tests shown to FAIL
  - [ ] Implementation written
  - [ ] Tests shown to PASS
  - [ ] Existing pool tests still pass

- [ ] **Phase 3:** Ctx and free functions
  - [ ] Tests for family ↔ poolIdx roundtrip
  - [ ] Tests for Intern/Raw/Release lifecycle
  - [ ] Tests shown to FAIL then PASS

- [ ] **Phase 4:** NLRI types
  - [ ] Size tests (4 bytes each)
  - [ ] Interface compliance tests
  - [ ] Tests shown to FAIL then PASS

- [ ] **Phase 5:** Accessors
  - [ ] INET.Prefix, PathID tests
  - [ ] IPVPN.RD, Labels tests
  - [ ] Other type accessor tests

- [ ] **Phase 6:** Parsing and integration
  - [ ] ParseINET/ParseIPVPN tests
  - [ ] Integration with existing code
  - [ ] HandleToNLRI reconstruction tests

### Final Verification

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Memory size assertions pass (4 bytes per NLRI type)
- [ ] Existing functional tests: 24+ passed (no regression)
- [ ] Self-review completed

---

## Prerequisites

**Complete spec-pool-handle-migration.md FIRST:**
- Single UPDATE pool with derived accessors
- Route/Update stores single `msgHandle`
- `Attrs()`, `NLRI()`, `Withdrawn()` derived on access

## Migration Strategy

This is a significant refactoring. Approach:

1. **Prerequisite:** Complete spec-pool-handle-migration.md
2. **Phase 1-2:** Add Handle encoding and Pool idx (backward compatible)
3. **Phase 3:** Add Ctx with NLRI pools, free functions
4. **Phase 4-5:** Add new NLRI types with `v2` suffix (e.g., `INETv2`)
5. **Phase 6:** Migrate parsing to use new types
6. **Final:** Update all call sites, remove old types

### Breaking Changes Summary

| Component | Change | Impact |
|-----------|--------|--------|
| `pool.New()` | Now takes `idx` parameter | All pool creation sites |
| `NLRI.Raw()` | Now takes `*Ctx` parameter | All raw access sites |
| `NLRI.Pack()` | Now takes `*Ctx` parameter | All pack sites |
| `NLRIHashable.Key()` | Now takes `*Ctx` parameter | Store operations |

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing NLRI consumers | v2 types during migration |
| Pool refcount bugs | Extensive lifecycle tests |
| Copy-then-release bugs | Document warning prominently |
| Performance regression | Benchmark before/after |
| Incorrect bit encoding | Edge case tests for max values |

---

## Appendix: Pool Index Assignments

```
Idx | Family              | Notes
----|---------------------|---------------------------
0   | IPv4 Unicast        | Most common
1   | IPv6 Unicast        |
2   | IPv4 Multicast      |
3   | IPv6 Multicast      |
4   | VPNv4               |
5   | VPNv6               |
6   | L2VPN EVPN          |
7   | IPv4 FlowSpec       |
8   | IPv6 FlowSpec       |
9   | IPv4 FlowSpec VPN   |
10  | IPv6 FlowSpec VPN   |
11  | IPv4 BGP-LS         |
12  | IPv6 BGP-LS         |
13  | IPv4 MVPN           |
14  | IPv6 MVPN           |
15  | VPLS                |
16  | RTC                 |
17  | MUP                 |
18-59| Reserved           | Future families
60  | Attributes (large)  | Path attributes > threshold
61  | Attributes (dedup)  | High-dedup attributes
62  | Reserved            | Future use
63  | INVALID             | Reserved for InvalidHandle
```

---

**Created:** 2025-12-28
**Reviewed:** 2025-12-28
**Status:** Ready for implementation
