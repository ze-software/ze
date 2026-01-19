# Spec: EOR Tracking - Only Send EOR for Families with Routes

## Task
Fix EOR (End-of-RIB) marker logic to only send EOR for address families where routes were actually sent since session start or last EOR.

## Problem
Current behavior: EOR is sent for ALL negotiated families, regardless of whether routes were sent.

Expected behavior: EOR should only be sent for families where routes were actually advertised.

**Example:**
- Config has: ipv4/unicast, ipv4/mpls-vpn, ipv6/unicast, ipv6/mpls-vpn
- Routes sent: 1 IPv4 unicast, 1 IPv6 unicast
- Current: Sends EOR for all 4 families
- Expected: Sends EOR only for ipv4/unicast and ipv6/unicast

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `docs/plan/CLAUDE_CONTINUATION.md` for current state
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission

### From RFC 4724 Section 2
> "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once
> it completes the initial routing update (including the case when
> there is no update to send) for an address family after the BGP
> session is established."

**Note:** RFC says send EOR even with no updates. However, for test compatibility with ExaBGP encoding tests, we follow the pattern of only sending EOR for families with routes. This is a practical optimization that avoids unnecessary messages.

## Current Code Analysis

### EOR Sending Locations in peer.go

| Line | Family | Condition |
|------|--------|-----------|
| 931-934 | IPv4 Unicast | `nf.IPv4Unicast` |
| 935-938 | IPv6 Unicast | `nf.IPv6Unicast` |
| 1821-1827 | MVPN | `nf.IPv4McastVPN` / `nf.IPv6McastVPN` |
| 2048-2049 | VPLS | Always (after sending VPLS routes) |
| 2233-2244 | FlowSpec | All negotiated FlowSpec families |
| 2445-2450 | MUP | `nf.IPv4MUP` / `nf.IPv6MUP` |

### Problem Areas
1. `sendInitialRoutes()` sends EOR based on negotiated families, not routes sent
2. No tracking of which families had routes sent

## Implementation Steps

### Step 1: Add Family Tracking to sendInitialRoutes

Track which families had routes sent during initial route sending:

```go
// In sendInitialRoutes(), add:
familiesSent := make(map[nlri.Family]bool)

// When sending a route, mark its family:
familiesSent[routeFamily] = true

// At end, only send EOR for families with routes:
for family := range familiesSent {
    _ = p.SendUpdate(message.BuildEOR(family))
}
```

### Step 2: Helper Function for Route Family

```go
// routeFamily returns the NLRI family for a StaticRoute.
func routeFamily(route StaticRoute) nlri.Family {
    if route.IsVPN() {
        if route.Prefix.Addr().Is6() {
            return nlri.Family{AFI: 2, SAFI: 128}
        }
        return nlri.Family{AFI: 1, SAFI: 128}
    }
    if route.Prefix.Addr().Is6() {
        return nlri.IPv6Unicast
    }
    return nlri.IPv4Unicast
}
```

### Step 3: Update Each Route-Sending Section

For each section that sends routes:
1. Track the family when route is sent
2. Only send EOR for tracked families at the end

### Step 4: Handle Special Route Types

| Route Type | Family Logic |
|------------|--------------|
| Static unicast | IPv4/IPv6 based on prefix |
| VPN | AFI from prefix, SAFI=128 |
| MVPN | AFI from prefix, SAFI=5 |
| VPLS | AFI=25, SAFI=65 |
| FlowSpec | AFI from config, SAFI=133/134 |
| MUP | AFI from config, SAFI=85 |

## Files to Modify

| File | Change |
|------|--------|
| `internal/reactor/peer.go` | Track families sent, conditional EOR |

## Test Plan

### Functional Test
Test 6 (extended-nexthop) should continue passing - it already expects EOR only for families with routes.

### Unit Test
Add test verifying EOR is only sent for families with routes:

```go
// TestEOROnlyForFamiliesWithRoutes verifies EOR is sent only for
// families where routes were advertised.
//
// VALIDATES: EOR sent only for IPv4 unicast when only IPv4 routes sent.
//
// PREVENTS: Spurious EOR for negotiated but unused families.
func TestEOROnlyForFamiliesWithRoutes(t *testing.T) {
    // Setup peer with IPv4+IPv6 negotiated
    // Send only IPv4 routes
    // Verify only IPv4 EOR sent
}
```

## Verification Checklist

- [ ] Unit test written and shown to FAIL first
- [ ] Implementation tracks families when sending routes
- [ ] EOR only sent for families with routes sent
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Functional test 6 (extended-nexthop) passes
