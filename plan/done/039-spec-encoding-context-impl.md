# Spec: EncodingContext Implementation

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/bgp/context/*.go - Current implementation               │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Create `pkg/bgp/context/` package with:
1. `EncodingContext` struct - capability-dependent encoding parameters
2. `ContextID` type (uint16) - compact identifier for fast comparison
3. `ContextRegistry` - register/get with hash-based deduplication
4. Helper methods: `Hash()`, `AddPathFor()`, `ToPackContext()`
5. Integration point for Peer recv/send contexts

## Current State (verified)

```
🔍 Functional tests: 24 passed, 13 failed [6, 7, 8, J, L, N, Q, S, T, U, V, Z, a]
📋 Last commit: 3a8ef7b
```

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof

### From TDD_ENFORCEMENT.md
- **TESTS MUST EXIST AND FAIL BEFORE IMPLEMENTATION BEGINS**
- Every test MUST document: VALIDATES (correct behavior) and PREVENTS (what bug it catches)
- If test passes before implementation: TEST IS WRONG
- Show test failure output, then implementation, then pass output

### From CODING_STANDARDS.md
- Go 1.21+ required
- `make lint` must pass with zero issues
- Never ignore errors - always handle or wrap with `%w`
- Prefer channels over mutexes for coordination
- Define package-level sentinel errors

## Codebase Context

### Existing Patterns to Follow

**`capability.Negotiated`** (pkg/bgp/capability/negotiated.go):
- Full session state with maps for families, addPath, extendedNextHop
- Created at session establishment via `Negotiate(local, remote, ...)`

**`nlri.PackContext`** (pkg/bgp/nlri/pack.go):
- Minimal struct with AddPath, ASN4 bools
- Used by NLRI.Pack(ctx) pattern

**`NegotiatedFamilies`** (pkg/reactor/peer.go):
- Pre-computed boolean flags for fast access
- Created from Negotiated via `computeNegotiatedFamilies()`

### New Package Location

```
pkg/bgp/context/
├── context.go      # EncodingContext struct
├── context_test.go # Tests
├── registry.go     # ContextRegistry
└── registry_test.go
```

## Implementation Steps

### Step 1: Create context.go with EncodingContext

**Test first:** `context_test.go`

```go
// TestEncodingContextHash verifies deterministic hashing.
//
// VALIDATES: Identical contexts produce identical hashes.
//
// PREVENTS: Registry deduplication failures from non-deterministic hashes.
func TestEncodingContextHash(t *testing.T)

// TestEncodingContextAddPathFor verifies per-family ADD-PATH lookup.
//
// VALIDATES: AddPathFor returns correct value per family.
//
// PREVENTS: Wrong encoding when ADD-PATH varies by family.
func TestEncodingContextAddPathFor(t *testing.T)

// TestEncodingContextToPackContext verifies PackContext conversion.
//
// VALIDATES: ToPackContext extracts correct per-family values.
//
// PREVENTS: NLRI encoding with wrong ADD-PATH setting.
func TestEncodingContextToPackContext(t *testing.T)
```

**Implementation:** `context.go`

```go
package context

import "codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"

// Family represents an AFI/SAFI combination.
// Matches capability.Family but avoids circular import.
type Family struct {
    AFI  uint16
    SAFI uint8
}

// EncodingContext holds capability-dependent encoding parameters.
// Same structure for source (receive) and destination (send).
//
// Created once per peer at session establishment.
// Registered in global registry for ID assignment.
type EncodingContext struct {
    // RFC 6793: Use 4-byte AS numbers
    ASN4 bool

    // RFC 7911: ADD-PATH enabled per family
    AddPath map[Family]bool

    // RFC 8950: Extended next-hop per family
    ExtendedNextHop map[Family]bool

    // Session context
    IsIBGP  bool
    LocalAS uint32
    PeerAS  uint32
}

// Hash returns a deterministic hash for deduplication.
func (ctx *EncodingContext) Hash() uint64

// AddPathFor returns whether ADD-PATH is enabled for a family.
func (ctx *EncodingContext) AddPathFor(f Family) bool

// ToPackContext creates nlri.PackContext for a specific family.
func (ctx *EncodingContext) ToPackContext(f Family) *nlri.PackContext
```

### Step 2: Create registry.go with ContextRegistry

**Test first:** `registry_test.go`

```go
// TestRegistryRegisterDeduplicates verifies identical contexts get same ID.
//
// VALIDATES: Register returns same ID for identical contexts.
//
// PREVENTS: Memory waste from duplicate context storage.
func TestRegistryRegisterDeduplicates(t *testing.T)

// TestRegistryGet verifies context retrieval by ID.
//
// VALIDATES: Get returns the registered context.
//
// PREVENTS: Nil dereference or wrong context on lookup.
func TestRegistryGet(t *testing.T)

// TestRegistryConcurrentAccess verifies thread safety.
//
// VALIDATES: Concurrent Register/Get don't race.
//
// PREVENTS: Data corruption under concurrent access.
func TestRegistryConcurrentAccess(t *testing.T)
```

**Implementation:** `registry.go`

```go
package context

import "sync"

// ContextID is a compact identifier for an EncodingContext.
// Enables fast compatibility checks via integer comparison.
type ContextID uint16

// ContextRegistry manages EncodingContext registration and lookup.
// Deduplicates identical contexts to save memory.
// Thread-safe for concurrent access.
type ContextRegistry struct {
    mu       sync.RWMutex
    contexts map[ContextID]*EncodingContext
    byHash   map[uint64]ContextID
    nextID   ContextID
}

// NewRegistry creates an empty registry.
func NewRegistry() *ContextRegistry

// Register returns ID for context, deduplicating identical ones.
func (r *ContextRegistry) Register(ctx *EncodingContext) ContextID

// Get retrieves context by ID.
func (r *ContextRegistry) Get(id ContextID) *EncodingContext

// Global registry instance
var Registry = NewRegistry()
```

### Step 3: Add FromNegotiated helper

**Test first:**

```go
// TestFromNegotiated verifies context creation from capability.Negotiated.
//
// VALIDATES: All relevant fields are extracted correctly.
//
// PREVENTS: Missing capability info in encoding context.
func TestFromNegotiated(t *testing.T)
```

**Implementation:**

```go
// FromNegotiated creates EncodingContext from capability negotiation result.
// Extracts encoding-relevant fields from the full Negotiated state.
func FromNegotiated(neg *capability.Negotiated) *EncodingContext
```

### Step 4: Integration Notes (Future)

After this package exists, Peer can be updated to:

```go
type Peer struct {
    // Receive context (source when storing routes)
    recvCtx   *context.EncodingContext
    recvCtxID context.ContextID

    // Send context (dest when encoding routes)
    sendCtx   *context.EncodingContext
    sendCtxID context.ContextID
}

func (p *Peer) onEstablished(neg *capability.Negotiated) {
    // Build contexts from negotiation
    p.recvCtx = context.FromNegotiatedRecv(neg)
    p.recvCtxID = context.Registry.Register(p.recvCtx)

    p.sendCtx = context.FromNegotiatedSend(neg)
    p.sendCtxID = context.Registry.Register(p.sendCtx)
}
```

This integration is OUT OF SCOPE for this spec - focus on the context package first.

## Test Specifications

### context_test.go

| Test | VALIDATES | PREVENTS |
|------|-----------|----------|
| `TestEncodingContextHash_Deterministic` | Same input → same hash | Dedup failures |
| `TestEncodingContextHash_Different` | Different input → different hash | False dedup |
| `TestEncodingContextAddPathFor_True` | Returns true when set | Wrong encoding |
| `TestEncodingContextAddPathFor_False` | Returns false when not set | Wrong encoding |
| `TestEncodingContextAddPathFor_Nil` | Handles nil map | Panic |
| `TestEncodingContextToPackContext` | Correct ASN4 and AddPath | NLRI mismatch |

### registry_test.go

| Test | VALIDATES | PREVENTS |
|------|-----------|----------|
| `TestRegistryRegister_NewContext` | Returns new ID | - |
| `TestRegistryRegister_Dedup` | Same ID for identical | Memory waste |
| `TestRegistryGet_Exists` | Returns correct context | Wrong lookup |
| `TestRegistryGet_NotExists` | Returns nil | Panic |
| `TestRegistryConcurrent` | No race conditions | Data corruption |

## Verification Checklist

### Phase 1: context.go
- [ ] Test `TestEncodingContextHash_Deterministic` written
- [ ] Test FAILS (no implementation)
- [ ] Implementation written
- [ ] Test PASSES
- [ ] Repeat for other context tests

### Phase 2: registry.go
- [ ] Test `TestRegistryRegister_NewContext` written
- [ ] Test FAILS
- [ ] Implementation written
- [ ] Test PASSES
- [ ] Repeat for other registry tests

### Final
- [ ] `go test -race ./pkg/bgp/context/...` passes
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] No circular imports

## Hash Implementation Notes

For deterministic hashing:

```go
import "hash/fnv"

func (ctx *EncodingContext) Hash() uint64 {
    h := fnv.New64a()

    // Write fixed fields
    if ctx.ASN4 {
        h.Write([]byte{1})
    } else {
        h.Write([]byte{0})
    }
    if ctx.IsIBGP {
        h.Write([]byte{1})
    } else {
        h.Write([]byte{0})
    }

    // Write ASNs
    binary.Write(h, binary.BigEndian, ctx.LocalAS)
    binary.Write(h, binary.BigEndian, ctx.PeerAS)

    // Write sorted map entries for determinism
    // ... sort keys, write each

    return h.Sum64()
}
```

---

**Created:** 2025-12-29
**Status:** Ready for TDD implementation
