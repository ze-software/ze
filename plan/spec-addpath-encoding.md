# Spec: ADD-PATH Encoding Fix

## Task
Fix ADD-PATH (path-information) encoding so test R passes.

## Problem Analysis

The test `R` (path-information) fails because:

1. Config has `capability { add-path send/receive; }` - ADD-PATH is negotiated
2. Routes are received with path IDs (e.g., `00000001` for 0.0.0.1)
3. When ZeBGP sends routes back, `INET.Bytes()` is called
4. **BUG:** `INET.Bytes()` only includes path ID if `hasPath` is true internally
5. It doesn't check the negotiated ADD-PATH capability to decide wire format

**RFC 7911 Section 3:** When ADD-PATH is negotiated, NLRI MUST include 4-byte Path Identifier.

## Root Cause

In `pkg/rib/commit.go:138`:
```go
nlriBytes = append(nlriBytes, route.NLRI().Bytes()...)
```

The `Bytes()` method is capability-agnostic. It should check if ADD-PATH is negotiated for the family and:
- If ADD-PATH send is negotiated AND NLRI has path ID → include path ID
- If ADD-PATH send is negotiated AND NLRI has NO path ID → prepend NOPATH (4 zeros)
- If ADD-PATH send NOT negotiated → omit path ID

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From RFC 7911 (ADD-PATH)
- Section 3: Path Identifier is 4 octets, prepended to NLRI
- Section 4: Send/Receive negotiation determines who can send path IDs
- MUST use extended encoding when capability is negotiated

### From ExaBGP Implementation
- `PathInfo.NOPATH`: 4 zero bytes when ADD-PATH enabled but no specific ID
- `PathInfo.DISABLED`: returns empty when capability not negotiated
- `pack_nlri(negotiated)` adapts wire format based on capability

## Codebase Context

**Key files:**
- `pkg/bgp/nlri/inet.go` - INET type with Bytes() method
- `pkg/bgp/nlri/nlri.go` - NLRI interface definition
- `pkg/rib/commit.go` - CommitService building UPDATE messages
- `pkg/bgp/message/message.go` - Negotiated struct with AddPath map
- `pkg/bgp/capability/negotiated.go` - Full capability negotiation

**Pattern to follow:**
ExaBGP's `pack_nlri(negotiated)` method that adapts based on capability.

## Design Decision: Unified Pack Pattern

Use `Pack(ctx *PackContext)` instead of `PackNLRI(addpath bool)`:

| Approach | Pros | Cons |
|----------|------|------|
| `PackNLRI(addpath bool)` | Simpler | Needs refactor for ASN4/etc |
| `Pack(ctx *PackContext)` | Future-proof, extensible | Slightly more complex |

**Note:** Cannot use `*message.Negotiated` directly due to circular import
(message imports nlri for EOR). Solution: define `PackContext` in nlri package.

**Decision:** Use `Pack(ctx *PackContext)` for:
- No circular imports
- Future extensibility (ASN4, Extended Next Hop)
- Matches ExaBGP's `pack_nlri(negotiated)` pattern

See `plan/spec-negotiated-packing.md` for full architectural rationale.

## Implementation Steps

### Step 1: Add PackContext and Pack method

```go
// In pkg/bgp/nlri/pack.go (new file)

// PackContext holds capability-dependent packing options.
// Used to adapt wire format based on negotiated session parameters.
type PackContext struct {
    // AddPath indicates ADD-PATH is negotiated for this family.
    // RFC 7911: When true, NLRI includes 4-byte Path Identifier.
    AddPath bool

    // Future fields:
    // ASN4 bool           // RFC 6793: 4-byte AS numbers
    // ExtendedNextHop AFI // RFC 8950: Extended next-hop encoding
}
```

```go
// In pkg/bgp/nlri/nlri.go
type NLRI interface {
    // ... existing methods ...

    // Pack returns wire-format bytes adapted for negotiated capabilities.
    // RFC 7911: Handles ADD-PATH path identifier based on ctx.AddPath.
    // If ctx is nil, behaves like Bytes() (no capability adaptation).
    Pack(ctx *PackContext) []byte
}
```

### Step 2: Implement Pack for INET

```go
// In pkg/bgp/nlri/inet.go
func (i *INET) Pack(ctx *PackContext) []byte {
    // If no context, return raw bytes
    if ctx == nil {
        return i.Bytes()
    }

    if ctx.AddPath {
        if i.hasPath {
            return i.Bytes() // Already has path ID
        }
        // Prepend NOPATH (4 zero bytes)
        return append([]byte{0, 0, 0, 0}, i.Bytes()...)
    }

    // No ADD-PATH: strip path ID if present
    if i.hasPath {
        return i.Bytes()[4:] // Skip path ID
    }
    return i.Bytes()
}
```

### Step 3: Update CommitService to use Pack

In `pkg/rib/commit.go`:
```go
// Helper to create PackContext from Negotiated
func (c *CommitService) packContext(family nlri.Family) *nlri.PackContext {
    msgFamily := message.Family{AFI: uint16(family.AFI), SAFI: uint8(family.SAFI)}
    return &nlri.PackContext{
        AddPath: c.negotiated.AddPath[msgFamily],
    }
}

// In buildGroupedUpdateTwoLevel
ctx := c.packContext(attrGroup.Family)
for _, route := range aspGroup.Routes {
    nlriBytes = append(nlriBytes, route.NLRI().Pack(ctx)...)
}

// In buildSingleUpdate
ctx := c.packContext(route.NLRI().Family())
nlriBytes := route.NLRI().Pack(ctx)
```

### Step 4: Implement Pack for other NLRI types

Each type implements `Pack(ctx *PackContext) []byte`:
- For types without ADD-PATH support: `return n.Bytes()`
- For types with ADD-PATH: same logic as INET

### Step 5: Verify with test R

Run: `go run ./test/cmd/functional encoding R`

## Verification Checklist

- [ ] Tests written and shown to FAIL first
- [ ] Pack method added to NLRI interface
- [ ] INET.Pack implemented correctly
- [ ] CommitService uses Pack with negotiated
- [ ] Other NLRI types have Pack (delegate to Bytes if no ADD-PATH)
- [ ] `go run ./test/cmd/functional encoding R` passes
- [ ] `make test` passes
- [ ] `make lint` passes
