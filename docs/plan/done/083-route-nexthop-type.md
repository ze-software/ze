# Spec: route-nexthop-type

## Task

Replace the dual-field pattern (`NextHop netip.Addr` + `NextHopSelf bool`) with a unified `RouteNextHop` type that encapsulates next-hop policy. Resolution stays at peer level where negotiated capabilities are known.

## Required Reading

- [ ] `.claude/zebgp/wire/ATTRIBUTES.md` - need wire encoding for NEXT_HOP validation
- [ ] `.claude/zebgp/wire/CAPABILITIES.md` - need Extended NH capability for cross-family resolution
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - contains dual-field pattern to replace
- [ ] `.claude/zebgp/api/CAPABILITY_CONTRACT.md` - API capability negotiation context
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - need build path integration point
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - need ExtendedNextHop resolution pattern
- [ ] `.claude/zebgp/config/SYNTAX.md` - need static route next-hop syntax

**Key insights:**
- Extended NH already implemented: `sendCtx.ExtendedNextHopFor(family)` returns NH AFI
- `EncodingContext.ExtendedNextHop` is `map[nlri.Family]nlri.AFI` - stores NH AFI per family
- Resolution already at peer level in `peer.go` via ExtendedNextHop checks
- BGP session has ONE local address (IPv4 OR IPv6, not both)
- "Self" = use `settings.LocalAddress`, validated against negotiated capabilities
- Config syntax: `next-hop <ip>;` in static routes, `next-hop self` in API text format
- Dual-field pattern exists in: `RouteSpec`, `NLRIGroup`, `NLRIBatch`, `StaticRoute`

## Design

### RouteNextHop Type

```go
// pkg/plugin/nexthop.go

// NextHopPolicy specifies how next-hop is determined for a route.
type NextHopPolicy uint8

const (
    NextHopUnset    NextHopPolicy = iota // Zero value = invalid
    NextHopExplicit                       // Use configured IP address
    NextHopSelf                           // Use session's local address
)

// RouteNextHop encapsulates next-hop policy for route origination.
// Resolution happens at peer level where negotiated capabilities are known.
type RouteNextHop struct {
    Policy NextHopPolicy
    Addr   netip.Addr // Valid only when Policy == NextHopExplicit
}

// Constructors
func NewNextHopExplicit(addr netip.Addr) RouteNextHop
func NewNextHopSelf() RouteNextHop

// Accessors
func (n RouteNextHop) IsSelf() bool
func (n RouteNextHop) IsExplicit() bool
func (n RouteNextHop) IsValid() bool // True if Self or Explicit with valid addr

// String returns "self" or the IP address string
func (n RouteNextHop) String() string
```

### Design Decisions

1. **Explicit addresses bypass validation**: `resolveNextHop()` returns explicit addresses without checking family compatibility or addr validity. User is responsible for providing valid next-hops. Rationale: explicit = intentional override. If addr is invalid, returns invalid addr without error.

2. **Constructor accepts invalid addr**: `NewNextHopExplicit(netip.Addr{})` is allowed. `IsValid()` returns false for this case. Callers should check `IsValid()` before calling `resolveNextHop()`.

3. **Error types in `pkg/reactor/peer.go`** (follows existing pattern):
   - `ErrNextHopUnset` - policy is zero value
   - `ErrNextHopSelfNoLocal` - Self but no LocalAddress configured
   - `ErrNextHopIncompatible` - Self address incompatible with family (no Extended NH)

### Resolution at Peer Level

```go
// pkg/reactor/peer.go

// resolveNextHop returns the actual IP for a RouteNextHop policy.
// Uses session's LocalAddress for Self, validates against Extended NH capability.
func (p *Peer) resolveNextHop(nh api.RouteNextHop, family nlri.Family) (netip.Addr, error) {
    switch nh.Policy {
    case api.NextHopExplicit:
        return nh.Addr, nil
    case api.NextHopSelf:
        local := p.settings.LocalAddress
        if !local.IsValid() {
            return netip.Addr{}, ErrNextHopSelfNoLocal
        }
        // Validate: can we use this address for this NLRI family?
        if !p.canUseNextHopFor(local, family) {
            return netip.Addr{}, ErrNextHopIncompatible
        }
        return local, nil
    default:
        return netip.Addr{}, ErrNextHopUnset
    }
}

// canUseNextHopFor checks if addr is valid as next-hop for family.
// Natural match (IPv4 for IPv4, IPv6 for IPv6) always allowed.
// Cross-family allowed if Extended NH capability negotiated.
func (p *Peer) canUseNextHopFor(addr netip.Addr, family nlri.Family) bool {
    // Natural match
    if addr.Is4() && family.AFI == nlri.AFIIPv4 { return true }
    if addr.Is6() && family.AFI == nlri.AFIIPv6 { return true }

    // Cross-family via Extended NH (RFC 5549/8950)
    if p.sendCtx != nil {
        nhAFI := p.sendCtx.ExtendedNextHopFor(family)
        if nhAFI != 0 {
            if addr.Is6() && nhAFI == nlri.AFIIPv6 { return true }
            if addr.Is4() && nhAFI == nlri.AFIIPv4 { return true }
        }
    }
    return false
}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestRouteNextHop_Constructors` | `pkg/plugin/nexthop_test.go` | NewNextHopExplicit, NewNextHopSelf |
| `TestRouteNextHop_ZeroValue` | `pkg/plugin/nexthop_test.go` | Zero value is invalid |
| `TestRouteNextHop_IsSelf` | `pkg/plugin/nexthop_test.go` | IsSelf() returns correct bool |
| `TestRouteNextHop_IsExplicit` | `pkg/plugin/nexthop_test.go` | IsExplicit() returns correct bool |
| `TestRouteNextHop_IsValid` | `pkg/plugin/nexthop_test.go` | IsValid(): Self=true, Explicit+valid=true, Explicit+invalid=false, Unset=false |
| `TestRouteNextHop_String` | `pkg/plugin/nexthop_test.go` | String(): Self="self", Explicit=IP, Unset="", Explicit+invalid="" |
| `TestResolveNextHop_Explicit` | `pkg/reactor/peer_test.go` | Explicit returns configured addr |
| `TestResolveNextHop_Self` | `pkg/reactor/peer_test.go` | Self returns LocalAddress |
| `TestResolveNextHop_SelfNoLocal` | `pkg/reactor/peer_test.go` | Self with no LocalAddress errors |
| `TestResolveNextHop_Unset` | `pkg/reactor/peer_test.go` | Unset policy returns ErrNextHopUnset |
| `TestResolveNextHop_ExplicitInvalid` | `pkg/reactor/peer_test.go` | Explicit with invalid addr returns invalid addr (no error) |
| `TestCanUseNextHopFor_IPv4Natural` | `pkg/reactor/peer_test.go` | IPv4 addr for IPv4 family OK |
| `TestCanUseNextHopFor_IPv6Natural` | `pkg/reactor/peer_test.go` | IPv6 addr for IPv6 family OK |
| `TestCanUseNextHopFor_ExtendedNH` | `pkg/reactor/peer_test.go` | IPv6 addr for IPv4 family with cap OK |
| `TestCanUseNextHopFor_CrossFamilyNoCap` | `pkg/reactor/peer_test.go` | Cross-family without cap fails |
| `TestCanUseNextHopFor_NilSendCtx` | `pkg/reactor/peer_test.go` | Nil sendCtx, cross-family fails |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| (none) | - | Existing tests cover route announcement behavior |

## Files to Modify

- `pkg/plugin/nexthop.go` - **NEW** RouteNextHop type and constructors
- `pkg/plugin/nexthop_test.go` - **NEW** Unit tests
- `pkg/plugin/types.go` - Update RouteSpec, NLRIGroup, NLRIBatch to use RouteNextHop
- `pkg/plugin/route.go` - Update parsing to create RouteNextHop
- `pkg/plugin/update_text.go` - Update parsedAttrs and parsing to use RouteNextHop
- `pkg/reactor/peersettings.go` - Update StaticRoute to use RouteNextHop
- `pkg/reactor/peer.go` - Add resolveNextHop(), canUseNextHopFor(), error vars
- `pkg/reactor/peer_test.go` - Add resolution tests
- `pkg/reactor/reactor.go` - Update buildAnnounceUpdateFromStatic()
- `pkg/config/bgp.go` - Update StaticRouteConfig parsing
- `pkg/config/loader.go` - Update route loading

## Implementation Steps

1. **Write tests** - Create `pkg/plugin/nexthop_test.go` and `pkg/reactor/peer_test.go` additions
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Minimal code: nexthop.go, peer.go resolution methods
4. **Run tests** - Verify PASS (paste output)
5. **Migrate** - Update types.go, route.go, peersettings.go, reactor.go, config
6. **Verify all** - `make lint && make test && make functional`
7. **RFC refs** - Add RFC comments to protocol code

## RFC Documentation

- Add `// RFC 4271 Section 5.1.3` comments to NEXT_HOP handling
- Add `// RFC 5549` / `// RFC 8950` comments to Extended NH resolution
- If RFC missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`

## Migration Notes

### Types with dual-field pattern

| Type | Location | Fields |
|------|----------|--------|
| `RouteSpec` | `pkg/plugin/types.go` | `NextHop`, `NextHopSelf` |
| `NLRIGroup` | `pkg/plugin/types.go` | `NextHop`, `NextHopSelf` |
| `NLRIBatch` | `pkg/plugin/types.go` | `NextHop`, `NextHopSelf` |
| `parsedAttrs` | `pkg/plugin/update_text.go` | `NextHop`, `NextHopSelf` |
| `StaticRoute` | `pkg/reactor/peersettings.go` | `NextHop`, `NextHopSelf` |

### Before
```go
type RouteSpec struct {
    Prefix      netip.Prefix
    NextHop     netip.Addr
    NextHopSelf bool
    PathAttributes
}
```

### After
```go
type RouteSpec struct {
    Prefix  netip.Prefix
    NextHop RouteNextHop  // Single field encapsulates policy
    PathAttributes
}

// Resolution centralized at peer level:
nextHop, err := peer.resolveNextHop(route.NextHop, family)
```

## Checklist

### 🧪 TDD
- [x] Tests written
- [ ] Tests FAIL (not captured)
- [x] Implementation complete
- [x] Tests PASS (output below)

> **Note:** Spec created retrospectively to document completed implementation.

### Test Output

```
=== RUN   TestRouteNextHop_Constructors
--- PASS: TestRouteNextHop_Constructors (0.00s)
=== RUN   TestRouteNextHop_ZeroValue
--- PASS: TestRouteNextHop_ZeroValue (0.00s)
=== RUN   TestRouteNextHop_IsSelf
--- PASS: TestRouteNextHop_IsSelf (0.00s)
=== RUN   TestRouteNextHop_IsExplicit
--- PASS: TestRouteNextHop_IsExplicit (0.00s)
=== RUN   TestRouteNextHop_IsValid
--- PASS: TestRouteNextHop_IsValid (0.00s)
=== RUN   TestRouteNextHop_String
--- PASS: TestRouteNextHop_String (0.00s)
=== RUN   TestResolveNextHop_Explicit
--- PASS: TestResolveNextHop_Explicit (0.00s)
=== RUN   TestResolveNextHop_Self
--- PASS: TestResolveNextHop_Self (0.00s)
=== RUN   TestResolveNextHop_SelfNoLocal
--- PASS: TestResolveNextHop_SelfNoLocal (0.00s)
=== RUN   TestResolveNextHop_Unset
--- PASS: TestResolveNextHop_Unset (0.00s)
=== RUN   TestResolveNextHop_ExplicitInvalid
--- PASS: TestResolveNextHop_ExplicitInvalid (0.00s)
=== RUN   TestCanUseNextHopFor_IPv4Natural
--- PASS: TestCanUseNextHopFor_IPv4Natural (0.00s)
=== RUN   TestCanUseNextHopFor_IPv6Natural
--- PASS: TestCanUseNextHopFor_IPv6Natural (0.00s)
=== RUN   TestCanUseNextHopFor_ExtendedNH
--- PASS: TestCanUseNextHopFor_ExtendedNH (0.00s)
=== RUN   TestCanUseNextHopFor_CrossFamilyNoCap
--- PASS: TestCanUseNextHopFor_CrossFamilyNoCap (0.00s)
=== RUN   TestCanUseNextHopFor_NilSendCtx
--- PASS: TestCanUseNextHopFor_NilSendCtx (0.00s)
PASS
```

### Verification
- [x] `make lint` passes (pre-existing deprecation warnings unrelated)
- [x] `make test` passes
- [x] `make functional` passes (18/18)

### Documentation
- [x] Required docs read
- [x] RFC references added
- [x] `.claude/zebgp/api/ARCHITECTURE.md` updated

### Completion
- [x] Spec moved to `docs/plan/done/083-route-nexthop-type.md`
