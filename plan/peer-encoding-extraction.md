# Peer Encoding Extraction Plan

**Status:** Planning (REVISED after critical review)
**Created:** 2025-12-22
**Revised:** 2025-12-22
**Priority:** High (major technical debt, regression risk)

---

## Critical Review Findings (2025-12-22)

### Issue 1: Route Types Should NOT Move

**Original plan proposed moving route types to builder package. This is WRONG.**

Route types (`StaticRoute`, `MVPNRoute`, etc.) are **config/settings types**:
- Defined in `pkg/reactor/peersettings.go`
- Used by config parsing, API, and session management
- Have serialization concerns (e.g., `RD string` + `RDBytes [8]byte`)

Moving them to `pkg/bgp/message/builder/` would:
- Mix wire encoding with config concerns
- Create import cycle: `config -> builder -> config`
- Violate layer separation

### Issue 2: Orchestration vs Pure Building

The plan conflated two types of code:

| Type | Functions | LOC | Should Move? |
|------|-----------|-----|--------------|
| **Orchestration** | `sendInitialRoutes`, `send*Routes` | ~200 | **No** |
| **Pure builders** | `build*Update`, `build*NLRI` | ~900 | Yes |

Orchestration code (grouping, EOR, iteration) uses `PeerSettings` and should stay in reactor.

### Issue 3: LOC Estimate Was Inflated

Corrected breakdown:

| Category | LOC | Movable? |
|----------|-----|----------|
| Connection management | ~350 | No |
| FSM callbacks | ~20 | No |
| Route orchestration | ~200 | No |
| Grouping helpers | ~100 | Maybe |
| **Pure builders** | **~900** | **Yes** |
| Other (helpers) | ~150 | Partial |

**Actual movable code: ~900-1000 LOC** (not ~1350)

### Issue 4: Existing Code Duplication

`pkg/bgp/attribute/origin.go` already provides:
- `PackAttribute()` - pack single attribute
- `PackAttributesOrdered()` - pack with RFC ordering

But peer.go manually appends attributes without using `PackAttributesOrdered`.

### Issue 5: Attribute Ordering Bug Found

In `buildGroupedUpdate` (lines 728-755):
```go
// 4. LOCAL_PREF (for iBGP).     ← Code 5
// ...
// 7. MED (if set).               ← Code 4 (WRONG ORDER!)
```

MED (code 4) should come BEFORE LOCAL_PREF (code 5) per RFC 4271 Appendix F.3.
`buildStaticRouteUpdate` has correct order; `buildGroupedUpdate` does not.

---

## Problem Statement (Revised)

`pkg/reactor/peer.go` (1725 LOC) contains ~900 LOC of UPDATE building logic that:

1. **Violates SRP**: Wire encoding mixed with connection management
2. **Has bugs**: Inconsistent attribute ordering between functions
3. **Duplicates code**: Doesn't use existing `PackAttributesOrdered()`
4. **Increases risk**: New AFIs require modifying connection code

### What Should Move

| Function | Purpose | Target |
|----------|---------|--------|
| `buildStaticRouteUpdate` | IPv4/IPv6/VPN UPDATE | `message/` |
| `buildGroupedUpdate` | Grouped UPDATE | `message/` |
| `buildMPReachNLRI` | VPN MP_REACH | `message/` |
| `buildMPReachNLRIUnicast` | IPv6 MP_REACH | `message/` |
| `buildVPNNLRIBytes` | VPN NLRI | `message/` |
| `buildMVPNUpdate` | MVPN UPDATE | `message/` |
| `buildMVPNMPReachNLRI` | MVPN MP_REACH | `message/` |
| `buildMVPNNLRI` | MVPN NLRI | `message/` |
| `buildVPLSUpdate` | VPLS UPDATE | `message/` |
| `buildVPLSMPReachNLRI` | VPLS MP_REACH | `message/` |
| `buildFlowSpecUpdate` | FlowSpec UPDATE | `message/` |
| `buildFlowSpecMPReachNLRI` | FlowSpec MP_REACH | `message/` |
| `buildMUPUpdate` | MUP UPDATE | `message/` |
| `buildMUPMPReachNLRI` | MUP MP_REACH | `message/` |
| `routeGroupKey` | Grouping key | `message/` |
| `groupRoutesByAttributes` | Route grouping | `message/` |

### What Should Stay in Reactor

| Function | Reason |
|----------|--------|
| `sendInitialRoutes` | Uses PeerSettings, orchestrates |
| `sendMVPNRoutes` | Uses PeerSettings, sends EOR |
| `sendVPLSRoutes` | Uses PeerSettings, sends EOR |
| `sendFlowSpecRoutes` | Uses PeerSettings, sends EOR |
| `sendMUPRoutes` | Uses PeerSettings, sends EOR |
| `groupMVPNRoutesByNextHop` | Uses MVPNRoute type |

---

## Target Architecture (Revised)

**Key change:** No separate `builder/` subpackage. Add to `pkg/bgp/message/` directly.

```
┌─────────────────────────────────────────────────────────────────┐
│                     pkg/reactor/peer.go                          │
│                     (~750 LOC after extraction)                  │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ TCP Lifecycle   │  │ FSM Callbacks   │  │ send*Routes     │  │
│  │ - Connect       │  │ - State changes │  │ - Orchestration │  │
│  │ - Backoff       │  │ - Established   │  │ - Uses builders │  │
│  │ - Reconnect     │  │                 │  │ - Sends EOR     │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
         │                                            │
         │ imports                                    │ calls
         ▼                                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                      pkg/bgp/message/                            │
│                                                                  │
│  Existing:                    New (builders):                    │
│  ┌──────────────┐            ┌─────────────────────────────┐    │
│  │ update.go    │            │ update_build.go             │    │
│  │ header.go    │            │ - BuildUnicastUpdate()      │    │
│  │ open.go      │            │ - BuildVPNUpdate()          │    │
│  │ ...          │            │ - BuildMVPNUpdate()         │    │
│  └──────────────┘            │ - BuildVPLSUpdate()         │    │
│                              │ - BuildFlowSpecUpdate()     │    │
│                              │ - BuildMUPUpdate()          │    │
│                              │ - GroupRoutesByAttributes() │    │
│                              └─────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Why No Subpackage?

1. **Simpler**: No new package to maintain
2. **Cohesive**: Builders produce `message.Update` - same package
3. **No import cycles**: message package has no reactor deps
4. **Existing pattern**: `message/eor.go` already builds EOR UPDATEs

---

## Implementation Plan (Revised)

### Phase 0: Fix Attribute Ordering Bug (IMMEDIATE)

**Before any extraction, fix the bug in `buildGroupedUpdate`:**

```go
// Current (WRONG):
// 4. LOCAL_PREF   ← code 5
// 7. MED          ← code 4

// Should be:
// 4. MED          ← code 4
// 5. LOCAL_PREF   ← code 5
```

**Or better:** Use `attribute.PackAttributesOrdered()` to ensure correct ordering.

### Phase 1: Create Builder Context Type

**File:** `pkg/bgp/message/update_build.go`

```go
package message

import "net/netip"

// UpdateBuilder provides context for building UPDATE messages.
type UpdateBuilder struct {
    LocalAS uint32
    IsIBGP  bool
    ASN4    bool // 4-byte AS negotiated
}

// UnicastParams contains parameters for unicast UPDATE.
// Uses primitive types to avoid importing reactor.
type UnicastParams struct {
    Prefix           netip.Prefix
    NextHop          netip.Addr
    Origin           uint8
    LocalPreference  uint32
    MED              uint32
    Communities      []uint32
    LargeCommunities [][3]uint32
    ExtCommunities   []byte
    ASPath           []uint32
    PathID           uint32
    // VPN fields
    Label    uint32
    RDBytes  [8]byte
    HasRD    bool
}
```

**Key insight:** Use primitive parameters, NOT reactor types. This avoids import cycles.

### Phase 2: Extract Unicast Builder (TDD)

**Step 2.1: Write failing tests**

```go
// pkg/bgp/message/update_build_test.go

func TestBuildUnicastUpdate_IPv4(t *testing.T) {
    // VALIDATES: IPv4 UPDATE with correct attribute ordering
    // PREVENTS: Attribute order bugs like in buildGroupedUpdate
}

func TestBuildUnicastUpdate_IPv6(t *testing.T) {
    // VALIDATES: IPv6 uses MP_REACH_NLRI
    // PREVENTS: Missing MP_REACH for non-IPv4
}

func TestBuildUnicastUpdate_AttributeOrder(t *testing.T) {
    // VALIDATES: Attributes ordered by type code per RFC 4271
    // PREVENTS: MED/LOCAL_PREF ordering bug
}
```

**Step 2.2: Implement builder**

```go
// BuildUnicastUpdate creates UPDATE for IPv4/IPv6 unicast route.
func (b *UpdateBuilder) BuildUnicast(p UnicastParams) *Update {
    attrs := b.buildAttributes(p)
    ordered := attribute.PackAttributesOrdered(attrs) // USE EXISTING!

    // ... NLRI handling
}
```

**Step 2.3: Update peer.go to use builder**

```go
// Before (peer.go)
update := buildStaticRouteUpdate(route, p.settings.LocalAS, ...)

// After (peer.go)
builder := &message.UpdateBuilder{
    LocalAS: p.settings.LocalAS,
    IsIBGP:  p.settings.IsIBGP(),
    ASN4:    asn4,
}
params := toUnicastParams(route) // Helper converts reactor.StaticRoute
update := builder.BuildUnicast(params)
```

### Phase 3: Extract VPN Builder (TDD)

Similar pattern - primitive params, use existing attribute packing.

### Phase 4: Extract MVPN Builder (TDD)

```go
type MVPNParams struct {
    RouteType uint8
    IsIPv6    bool
    RD        [8]byte
    SourceAS  uint32
    Source    netip.Addr
    Group     netip.Addr
    NextHop   netip.Addr
    // ... etc
}
```

### Phase 5: Extract VPLS Builder (TDD)

### Phase 6: Extract FlowSpec Builder (TDD)

### Phase 7: Extract MUP Builder (TDD)

### Phase 8: Cleanup & Verification

1. Remove extracted functions from `peer.go`
2. Add `toXxxParams()` conversion helpers in reactor
3. Verify all tests pass
4. Run `make test && make lint`

---

## Route Types Stay in Reactor

**DO NOT move route types.** They are config/settings types.

```
pkg/reactor/peersettings.go     ← Route types STAY HERE
    StaticRoute
    MVPNRoute
    VPLSRoute
    FlowSpecRoute
    MUPRoute

pkg/bgp/message/update_build.go ← Builders use PRIMITIVE params
    UnicastParams
    MVPNParams
    VPLSParams
    FlowSpecParams
    MUPParams
```

Reactor provides `toXxxParams()` helpers to convert route types to builder params.

---

## Testing Strategy (Revised)

### Unit Tests (per builder)

```go
// pkg/bgp/message/update_build_test.go

// VALIDATES: Correct attribute ordering per RFC 4271
// PREVENTS: MED/LOCAL_PREF ordering bugs
func TestBuildUnicast_AttributeOrder(t *testing.T)

// VALIDATES: IPv4 unicast in NLRI field
func TestBuildUnicast_IPv4(t *testing.T)

// VALIDATES: IPv6 uses MP_REACH_NLRI
func TestBuildUnicast_IPv6(t *testing.T)

// VALIDATES: VPN routes use MP_REACH with RD+label
func TestBuildVPN(t *testing.T)

// VALIDATES: Communities sorted per RFC 1997
func TestBuildUnicast_CommunitiesSorted(t *testing.T)

// VALIDATES: AS_PATH uses 4-byte when ASN4=true
func TestBuildUnicast_ASN4(t *testing.T)
```

### Regression Tests

```go
// Compare wire output against current peer.go functions
func TestBuildUnicast_MatchesPeerGo(t *testing.T) {
    // Build with new builder
    // Build with old function
    // Compare bytes
}
```

### Integration Tests

Existing tests in `pkg/reactor/peer_test.go` should continue to pass.

---

## Risk Mitigation (Revised)

| Risk | Mitigation |
|------|------------|
| Breaking existing tests | Run `make test` after each phase |
| Wire format changes | Regression tests compare byte output |
| Import cycles | Builders use primitive params, not reactor types |
| Attribute ordering bugs | Use `PackAttributesOrdered()`, add order tests |
| Performance regression | Benchmark before/after (optional) |

---

## Effort Estimate (Revised)

| Phase | Effort | Risk |
|-------|--------|------|
| 0. Fix ordering bug | 0.5h | Low |
| 1. Create UpdateBuilder | 1h | Low |
| 2. Unicast extraction | 3h | Medium |
| 3. VPN extraction | 2h | Low |
| 4. MVPN extraction | 2h | Medium |
| 5. VPLS extraction | 1.5h | Low |
| 6. FlowSpec extraction | 1.5h | Low |
| 7. MUP extraction | 1.5h | Low |
| 8. Cleanup & verify | 2h | Low |

**Total: ~15 hours (2-3 days)**

---

## Success Criteria (Revised)

1. `peer.go` reduced to ~750 LOC (connection + orchestration)
2. All existing tests pass
3. No wire format changes (byte-identical output)
4. Attribute ordering is correct in all builders (test verified)
5. New address families can be added in `message/` without touching `peer.go`
6. Each builder has comprehensive tests

---

## Tasks (Revised)

- [ ] **Phase 0**: Fix attribute ordering bug in `buildGroupedUpdate`
- [ ] **Phase 1**: Create `UpdateBuilder` and `UnicastParams` in `message/`
- [ ] **Phase 2**: Extract unicast builder (TDD)
- [ ] **Phase 3**: Extract VPN builder (TDD)
- [ ] **Phase 4**: Extract MVPN builder (TDD)
- [ ] **Phase 5**: Extract VPLS builder (TDD)
- [ ] **Phase 6**: Extract FlowSpec builder (TDD)
- [ ] **Phase 7**: Extract MUP builder (TDD)
- [ ] **Phase 8**: Cleanup and verify

---

## Related Files (Revised)

| File | Current LOC | Target LOC |
|------|-------------|------------|
| `pkg/reactor/peer.go` | 1725 | ~750 |
| `pkg/reactor/peersettings.go` | 204 | 204 (unchanged) |
| `pkg/bgp/message/update_build.go` | 0 | ~700 |
| `pkg/bgp/message/update_build_test.go` | 0 | ~400 |

---

## Alternative Considered: Builder Subpackage

Original plan proposed `pkg/bgp/message/builder/`. Rejected because:

1. **Over-engineering**: One file (`update_build.go`) is simpler
2. **Existing pattern**: `message/eor.go` already builds UPDATEs in same package
3. **No benefit**: Subpackage adds complexity without solving any problem

---

## Open Questions

1. **Should grouping logic move?** `groupRoutesByAttributes()` uses `StaticRoute` type.
   - Option A: Keep in reactor (simpler)
   - Option B: Create generic grouping with key function

2. **Param struct per AFI or unified?** Current plan: separate structs.
   - Pro: Clear, type-safe
   - Con: Some duplication

---

## Summary of Critical Review Changes

| Original | Revised |
|----------|---------|
| Move route types to builder | Keep in reactor |
| Create `builder/` subpackage | Add to `message/` directly |
| ~1350 LOC extraction | ~900 LOC extraction |
| peer.go → 400 LOC | peer.go → 750 LOC |
| ~19h effort | ~15h effort |
| RouteBuilder interface | Primitive params |
| N/A | Fix ordering bug first |
