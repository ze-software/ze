# Spec: AFI/SAFI Map-Based Refactor

## Task
Consolidate Family type to nlri package, separate "what was negotiated" from "how to encode", eliminate data duplication between NegotiatedFamilies and EncodingContext.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/wire/CAPABILITIES.md` - AFI/SAFI in capability negotiation
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build path vs Forward path
- [x] `.claude/zebgp/wire/NLRI.md` - Family type, pre-computed constants
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - EncodingContext structure

**Key insights from docs:**
1. EncodingContext already uses `map[Family]bool` for AddPath/ExtendedNextHop
2. EncodingContext is directional: recvCtx (parsing) vs sendCtx (encoding)
3. NegotiatedFamilies duplicates AddPath/ExtNH/ASN4 that's already in sendCtx
4. ADD-PATH negotiation is asymmetric (send vs receive capabilities differ)

## Current State

### Tests
```
make test       - PASS
make lint       - 0 issues
make functional - 37/37 passed
```

### Last Commit
`3f2d828` docs(plan): mark spec-update-size-limiting as done

## Problem Analysis

### 1. Data Duplication

| Field | NegotiatedFamilies | EncodingContext | Duplicate? |
|-------|-------------------|-----------------|------------|
| Enabled families | `IPv4Unicast bool` | ❌ | No |
| AddPath | `IPv4UnicastAddPath bool` | `AddPath map` | **YES** |
| ExtNH | `IPv4UnicastExtNH bool` | `ExtendedNextHop map` | **YES** |
| ASN4 | `ASN4 bool` | `ASN4 bool` | **YES** |
| ExtendedMessage | `ExtendedMessage bool` | ❌ | No |

`nf.IPv4UnicastAddPath` == `sendCtx.AddPath[IPv4Unicast]` — same data, two places.

### 2. Family Type Duplication

Three packages define Family:
- `nlri.Family` (nlri/nlri.go)
- `capability.Family` (capability/capability.go)
- `context.Family` (context/context.go)

### 3. Boolean Field Explosion (25 fields)

Adding a new family requires:
- 1 bool for Enabled
- 1 bool for AddPath
- 1 bool for ExtNH
- N switch cases updated

## Solution Design

### Architecture: Separation of Concerns

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│   NegotiatedCapabilities    │     │      EncodingContext        │
│   "What was negotiated"     │     │   "How to encode/decode"    │
├─────────────────────────────┤     ├─────────────────────────────┤
│ families map[Family]bool    │     │ ASN4 bool                   │
│ ExtendedMessage bool        │     │ AddPath map[Family]bool     │
│                             │     │ ExtNH map[Family]AFI        │
│ Has(f) bool                 │     │ IsIBGP bool                 │
│ Families() []Family (sorted)│     │ LocalAS, PeerAS uint32      │
└─────────────────────────────┘     └─────────────────────────────┘
              │                                  │
              │                                  │
              ▼                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                            Peer                                  │
├─────────────────────────────────────────────────────────────────┤
│ negotiated *NegotiatedCapabilities  // What families enabled     │
│ recvCtx    *EncodingContext         // How to parse incoming     │
│ sendCtx    *EncodingContext         // How to encode outgoing    │
└─────────────────────────────────────────────────────────────────┘
```

### Phase 1: Consolidate Family Type

Make `nlri.Family` the canonical type. Other packages import from nlri.

**nlri/nlri.go** (already exists, extend):
```go
type Family struct {
    AFI  AFI
    SAFI SAFI
}

// Pre-computed constants (add missing ones)
var (
    IPv4Unicast        = Family{AFI: AFIIPv4, SAFI: SAFIUnicast}
    IPv6Unicast        = Family{AFI: AFIIPv6, SAFI: SAFIUnicast}
    IPv4LabeledUnicast = Family{AFI: AFIIPv4, SAFI: SAFIMPLSLabel}
    IPv6LabeledUnicast = Family{AFI: AFIIPv6, SAFI: SAFIMPLSLabel}
    IPv4VPN            = Family{AFI: AFIIPv4, SAFI: SAFIVPN}
    IPv6VPN            = Family{AFI: AFIIPv6, SAFI: SAFIVPN}
    IPv4FlowSpec       = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
    IPv6FlowSpec       = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
    IPv4FlowSpecVPN    = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}
    IPv6FlowSpecVPN    = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpecVPN}
    L2VPNVPLS          = Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
    IPv4McastVPN       = Family{AFI: AFIIPv4, SAFI: SAFIMVPN}
    IPv6McastVPN       = Family{AFI: AFIIPv6, SAFI: SAFIMVPN}
    IPv4MUP            = Family{AFI: AFIIPv4, SAFI: SAFIMUP}
    IPv6MUP            = Family{AFI: AFIIPv6, SAFI: SAFIMUP}
)

// FamilyLess provides deterministic ordering for sorted iteration.
func FamilyLess(a, b Family) bool {
    if a.AFI != b.AFI {
        return a.AFI < b.AFI
    }
    return a.SAFI < b.SAFI
}
```

**capability/capability.go** - use nlri.Family:
```go
import "github.com/exa-networks/zebgp/pkg/bgp/nlri"

// Family is an alias for nlri.Family for backward compatibility.
type Family = nlri.Family
```

**context/context.go** - use nlri.Family:
```go
import "github.com/exa-networks/zebgp/pkg/bgp/nlri"

type EncodingContext struct {
    ASN4            bool
    AddPath         map[nlri.Family]bool
    ExtendedNextHop map[nlri.Family]nlri.AFI  // Value = next-hop AFI
    IsIBGP          bool
    LocalAS         uint32
    PeerAS          uint32
}
```

### Phase 2: New NegotiatedCapabilities

Replace NegotiatedFamilies with NegotiatedCapabilities:

```go
// NegotiatedCapabilities tracks what was negotiated (not how to encode).
// Encoding params live in EncodingContext.
type NegotiatedCapabilities struct {
    families        map[nlri.Family]bool  // private for O(1) lookup
    ExtendedMessage bool
}

// NewNegotiatedCapabilities creates from capability negotiation result.
func NewNegotiatedCapabilities(neg *capability.Negotiated) *NegotiatedCapabilities {
    nc := &NegotiatedCapabilities{
        families:        make(map[nlri.Family]bool),
        ExtendedMessage: neg.ExtendedMessage,
    }
    for _, f := range neg.Families() {
        nc.families[nlri.Family{AFI: nlri.AFI(f.AFI), SAFI: nlri.SAFI(f.SAFI)}] = true
    }
    return nc
}

// Has returns whether the family was negotiated.
func (nc *NegotiatedCapabilities) Has(f nlri.Family) bool {
    if nc == nil || nc.families == nil {
        return false
    }
    return nc.families[f]
}

// Families returns all negotiated families in deterministic order.
// Used for EOR sending where order should be reproducible for testing.
func (nc *NegotiatedCapabilities) Families() []nlri.Family {
    if nc == nil || nc.families == nil {
        return nil
    }
    result := make([]nlri.Family, 0, len(nc.families))
    for f := range nc.families {
        result = append(result, f)
    }
    sort.Slice(result, func(i, j int) bool {
        return nlri.FamilyLess(result[i], result[j])
    })
    return result
}
```

### Phase 3: Update Peer Struct

```go
type Peer struct {
    // ... existing fields ...

    // Capability negotiation results
    negotiated *NegotiatedCapabilities  // What families are enabled

    // Encoding contexts (already exist)
    recvCtx    *EncodingContext  // How to parse incoming
    sendCtx    *EncodingContext  // How to encode outgoing
}
```

### Phase 4: Update Usage Sites

| Before | After |
|--------|-------|
| `nf.IPv4Unicast` | `negotiated.Has(nlri.IPv4Unicast)` |
| `nf.IPv4UnicastAddPath` | `sendCtx.AddPath[nlri.IPv4Unicast]` |
| `nf.IPv4UnicastExtNH` | `sendCtx.ExtendedNextHop[nlri.IPv4Unicast] == nlri.AFIIPv6` |
| `nf.ASN4` | `sendCtx.ASN4` |
| `nf.ExtendedMessage` | `negotiated.ExtendedMessage` |

### Phase 5: Update EncodingContext.ExtendedNextHop

Change from `map[Family]bool` to `map[Family]AFI`:

```go
// Before
ExtendedNextHop map[Family]bool

// After - stores the next-hop AFI (e.g., AFIIPv6 for IPv4 prefix with IPv6 NH)
ExtendedNextHop map[nlri.Family]nlri.AFI
```

Usage:
```go
// Before
if sendCtx.ExtendedNextHop[family] {
    // use IPv6 next-hop for IPv4
}

// After
if nhAFI := sendCtx.ExtendedNextHop[family]; nhAFI == nlri.AFIIPv6 {
    // use IPv6 next-hop for IPv4
}
```

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/bgp/nlri/nlri.go` | Add missing Family constants, add SAFI constants, add FamilyLess |
| `pkg/bgp/capability/capability.go` | Type alias `Family = nlri.Family`, remove duplicate AFI/SAFI |
| `pkg/bgp/context/context.go` | Use nlri.Family, change ExtendedNextHop to map[Family]AFI |
| `pkg/bgp/context/registry.go` | Update for nlri.Family |
| `pkg/bgp/context/negotiated.go` | Update for nlri.Family |
| `pkg/reactor/peer.go` | Replace NegotiatedFamilies with NegotiatedCapabilities, update all usage |
| `pkg/reactor/reactor.go` | Update family checks to use negotiated.Has() and sendCtx |

## Implementation Steps

### Step 1: Add Missing SAFI Constants to nlri.go

```go
const (
    // ... existing ...
    SAFIMVPN        SAFI = 5   // RFC 6514
    SAFIVPLS        SAFI = 65  // RFC 4761
    SAFIMUP         SAFI = 85  // draft-mpmz-bess-mup-safi
    SAFIFlowSpecVPN SAFI = 134 // RFC 8955
)
```

### Step 2: Add Family Constants and FamilyLess to nlri.go

TDD: Write test first, see it fail, implement.

### Step 3: Update capability.go to Use nlri.Family

TDD: Ensure existing tests pass with type alias.

### Step 4: Update context.go for nlri.Family and ExtNH map[Family]AFI

TDD: Update context tests.

### Step 5: Create NegotiatedCapabilities

TDD: Write tests for Has(), Families() with sorted output.

### Step 6: Update Peer to Use NegotiatedCapabilities

TDD: Update peer tests.

### Step 7: Update All Usage Sites

Search and replace pattern:
- `nf.IPv4Unicast` → `negotiated.Has(nlri.IPv4Unicast)`
- `nf.IPv4UnicastAddPath` → `p.sendCtx.AddPath[nlri.IPv4Unicast]`
- etc.

### Step 8: Delete NegotiatedFamilies

Remove old struct and computeNegotiatedFamilies.

### Step 9: Final Test Run

```bash
make test && make lint && make functional
```

## Code Reduction Estimate

| Area | Before | After | Savings |
|------|--------|-------|---------|
| NegotiatedFamilies struct | 50 lines | 0 lines | 50 lines |
| NegotiatedCapabilities | 0 lines | 35 lines | -35 lines |
| computeNegotiatedFamilies | 100 lines | 15 lines | 85 lines |
| Duplicate AFI/SAFI in capability | 40 lines | 5 lines (alias) | 35 lines |
| Duplicate Family in context | 25 lines | 2 lines (import) | 23 lines |
| Switch statements | 60 lines | 10 lines | 50 lines |
| **Total** | **~275 lines** | **~67 lines** | **~208 lines** |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Import cycles | nlri is low-level, safe to import from capability/context |
| Type conversion | Use type alias in capability for backward compat |
| Missing family constant | Compiler catches missing constants in map literals |
| EOR order change | FamilyLess ensures deterministic sorted order |
| Nil map panics | Helper methods check nil before access |

## Progress (Updated 2026-01-02)

### ✅ Completed

| Step | Status |
|------|--------|
| Add FamilyLess to nlri.go | ✅ Done |
| Add IPv4LabeledUnicast, IPv6LabeledUnicast constants | ✅ Done |
| capability.go: `type AFI = nlri.AFI`, `type SAFI = nlri.SAFI`, `type Family = nlri.Family` | ✅ Done |
| context.go: Use nlri.Family, change ExtendedNextHop to `map[Family]AFI` | ✅ Done |
| Create NegotiatedCapabilities struct in reactor/negotiated.go | ✅ Done |
| Change Peer.families to `negotiated atomic.Pointer[NegotiatedCapabilities]` | ✅ Done |
| Partial migration of sendMVPNRoutes, sendMUPRoutes, sendFlowSpecRoutes | ✅ Partial |

### ❌ Build Broken - Interrupted Migration

**Current errors (`go build ./...`):**
```
pkg/reactor/peer.go:1887: undefined: nf   (should be nc.Has(nlri.IPv4MVPN))
pkg/reactor/peer.go:1891: undefined: nf   (should be nc.Has(nlri.IPv6MVPN))
pkg/reactor/peer.go:2111: undefined: nf   (should be nc.Has(...))
pkg/reactor/peer.go:2115: undefined: nf   (should be nc.Has(...))
pkg/reactor/peer.go:2166: p.families undefined (should be p.negotiated)
pkg/reactor/reactor.go:826: peer.families undefined
pkg/reactor/reactor.go:1662: peer.families undefined
pkg/reactor/reactor.go:1940: peer.families undefined
```

### 🔧 Remaining Work

#### 1. Fix peer.go (4 issues)

Lines 1887, 1891 - EOR sending in sendMVPNRoutes:
```go
// Before (broken)
if nf.IPv4McastVPN {

// After
if nc.Has(nlri.IPv4MVPN) {
```

Lines 2111, 2115 - EOR sending (similar pattern)

Line 2166 - families.Load():
```go
// Before (broken)
nf := p.families.Load()

// After
nc := p.negotiated.Load()
```

#### 2. Fix reactor.go (3 issues)

Lines 826, 1662, 1940 - Change `peer.families.Load()` → `peer.negotiated.Load()`

Then update usages from `nf.IPv4Unicast` → `nc.Has(nlri.IPv4Unicast)` pattern.

#### 3. Fix Test Files

Files with old API:
- `peer_test.go` - 7 occurrences of `peer.families.Store(&NegotiatedFamilies{...})`
- `forward_split_test.go` - 4 occurrences

Convert test setup from:
```go
peer.families.Store(&NegotiatedFamilies{IPv4Unicast: true})
```
To:
```go
peer.negotiated.Store(&NegotiatedCapabilities{...})
// or use NewNegotiatedCapabilities() helper
```

#### 4. Delete NegotiatedFamilies

After all usages migrated:
- Remove `type NegotiatedFamilies struct` from peer.go
- Remove `computeNegotiatedFamilies()` function
- Remove related test functions

#### 5. Final Verification

```bash
make test && make lint && make functional
```

## Checklist

- [x] Required docs read
- [x] Consolidated Family type to nlri.Family
- [x] Changed ExtendedNextHop to map[Family]AFI
- [x] Created NegotiatedCapabilities struct
- [x] Changed Peer.families to Peer.negotiated
- [x] Fix remaining build errors in peer.go
- [x] Fix remaining build errors in reactor.go
- [x] Fix test files
- [x] Delete NegotiatedFamilies struct
- [x] make test passes
- [x] make lint passes
- [x] make functional passes
- [x] Update `.claude/zebgp/ENCODING_CONTEXT.md` with new architecture

## Dependencies

None - internal refactor.

## API Impact

None - all changes are internal. External API unchanged.

## Documentation Updates

After implementation, update:
- `.claude/zebgp/ENCODING_CONTEXT.md` - document NegotiatedCapabilities
- `.claude/zebgp/wire/CAPABILITIES.md` - reference nlri.Family as canonical type
