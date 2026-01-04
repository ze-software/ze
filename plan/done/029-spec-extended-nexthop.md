# Spec: extended-nexthop RFC 8950 implementation

## Task
Implement RFC 8950 extended next-hop encoding to allow IPv4 NLRI with IPv6 next-hops.

## Problem
Test 6 (extended-nexthop) fails because:
- **Expected:** `MP_REACH_NLRI` with AFI=1, SAFI=1, IPv6 next-hop, IPv4 NLRI
- **Received:** Traditional UPDATE with NEXT_HOP attribute and inline IPv4 NLRI

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

### From RFC 8950

1. **Capability Code 5** (Extended Next Hop Encoding):
   - Each tuple: NLRI AFI (2) + Reserved (1) + NLRI SAFI (1) + NextHop AFI (2) = 6 bytes
   - Both peers MUST advertise same tuple for it to be negotiated

2. **MP_REACH_NLRI encoding for AFI=1/SAFI=1 with IPv6 next-hop:**
   - AFI = 1 (IPv4)
   - SAFI = 1, 2, or 4
   - Length of Next Hop = 16 or 32 (IPv6 or IPv6 + link-local)
   - Next Hop = IPv6 address
   - NLRI = IPv4 prefixes as normal

3. **When to use extended next-hop:**
   - IPv4 prefix with IPv6 next-hop
   - Extended next-hop negotiated for (AFI=1, SAFI=1, NextHopAFI=2)

### From ExaBGP

- `nexthop.py`: Capability is list of `(AFI, SAFI, NextHopAFI)` tuples
- `negotiated.py`: Intersection of local and remote tuples stored in `self.nexthop`
- Route encoding checks negotiated list before using extended next-hop

## Codebase Context

### Files to Modify

| File | Change |
|------|--------|
| `pkg/bgp/capability/negotiated.go` | Add `ExtendedNextHop` negotiation |
| `pkg/reactor/peer.go` | Add extended NH to `NegotiatedFamilies`, modify route building |

### Current State

1. `capability.go` already has `ExtendedNextHop` struct and parsing
2. `Negotiate()` in `negotiated.go` does NOT process `*ExtendedNextHop`
3. `buildStaticRouteUpdate()` checks `route.Prefix.Addr().Is4()` → uses inline NLRI
4. No way to tell route builder that extended next-hop is negotiated

## Implementation Steps

### Step 1: Add ExtendedNextHop to Negotiated struct

```go
// In negotiated.go, add to Negotiated struct:
extendedNextHop map[extendedNHKey]AFI  // (nlriAFI, nlriSAFI) -> nexthopAFI

// In Negotiate(), add processing for *ExtendedNextHop
```

### Step 2: Add ExtendedNextHop check to NegotiatedFamilies

```go
// In peer.go NegotiatedFamilies:
IPv4UnicastExtNH AFI  // 0 = not negotiated, 2 = IPv6 next-hop allowed
```

### Step 3: Modify buildStaticRouteUpdate

```go
// Change the switch statement to check for extended next-hop:
case route.Prefix.Addr().Is4():
    if route.NextHop.Is6() && extNHAllowed {
        // Use MP_REACH_NLRI with AFI=1, IPv6 next-hop
        attrBytes = append(attrBytes, buildMPReachNLRIExtNH(route)...)
    } else {
        // Traditional inline NLRI
        nlriBytes = inet.Bytes()
    }
```

### Step 4: Add buildMPReachNLRIExtNH function

Build MP_REACH_NLRI for IPv4 NLRI with IPv6 next-hop per RFC 8950.

## Wire Format (RFC 8950 Section 3)

```
MP_REACH_NLRI for IPv4/IPv6-nexthop:
  AFI = 1 (2 octets)
  SAFI = 1 (1 octet)
  Next Hop Length = 16 (1 octet)
  Next Hop = IPv6 address (16 octets)
  Reserved = 0 (1 octet)
  NLRI = IPv4 prefixes (variable)
```

## Test Plan

### Unit Tests

1. `TestNegotiateExtendedNextHop` - verify capability negotiation
2. `TestBuildMPReachNLRIExtNH` - verify wire format

### Functional Test

- Run `go run ./test/cmd/functional encoding 6`

## Verification Checklist

- [ ] Unit tests written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Functional test 6 (extended-nexthop) passes
- [ ] RFC 8950 wire format verified against expected output
