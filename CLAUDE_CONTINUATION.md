# ZeBGP Development Continuation Notes

## Last Session: 2025-12-20

---

## Current Test Status

### Tests Exist But Most Fail

**Tests ARE copied from ExaBGP** - infrastructure works. Failures are due to **missing protocol implementation**.

```bash
# Run all tests
go run ./cmd/self-check --timeout=10s --all

# Current status (2025-12-20):
# ~6 pass, ~31 fail out of 37 encoding tests
```

**Test locations:**
```
testdata/encode/   # 37 .ci files + 38 .conf files
testdata/decode/   # 18 .test files
testdata/api/      # 89 files
testdata/parse/    # Config parsing tests
testdata/scripts/  # Helper scripts
```

### What's Missing in ZeBGP (Causes Test Failures)

| Feature | Status | Blocks Tests |
|---------|--------|--------------|
| MP_REACH_NLRI / MP_UNREACH_NLRI | Not implemented | Most IPv6, VPN tests |
| Communities | Not implemented | community tests |
| Extended Communities | Not implemented | ext-community tests |
| Large Communities | Not implemented | large-community tests |
| FlowSpec NLRI | Not implemented | flow-* tests |
| VPN NLRI (VPNv4/v6) | Not implemented | vpn tests |
| EVPN NLRI | Not implemented | l2vpn tests |
| BGP-LS NLRI | Not implemented | bgp-ls-* decode tests |

---

## What Remains To Be Done

### Priority 1: Implement Missing Protocol Features

**To pass existing tests, implement in this order:**

1. **MP_REACH_NLRI / MP_UNREACH_NLRI** (RFC 4760)
   - Required for: IPv6, VPN, FlowSpec, EVPN, all multi-protocol
   - Files: `pkg/bgp/attribute/mp_reach.go`, `pkg/bgp/attribute/mp_unreach.go`

2. **Communities** (RFC 1997)
   - Required for: community tests
   - Files: `pkg/bgp/attribute/communities.go`

3. **Extended Communities** (RFC 4360)
   - Required for: VPN, FlowSpec redirect
   - Files: `pkg/bgp/attribute/extended_communities.go`

4. **Large Communities** (RFC 8092)
   - Required for: large community tests
   - Files: `pkg/bgp/attribute/large_communities.go`

### Priority 2: Config-Based Route Announcements

The infrastructure is in place but **Freeform parsing doesn't extract nested route data**.

**Current Limitation:**
```go
// In pkg/config/bgp.go:77-78
Field("static", Freeform()),  // Stores "route 10.0.0.0/24" as key, loses nested data
```

**Solutions (pick one):**
1. Parse Freeform data manually in `parseNeighborConfig()` - complex regex/tokenizing
2. Create custom schema type that handles both block and inline syntax
3. Only support block syntax - change to `List(TypePrefix, ...)` schema

### Priority 3: Improve Test Coverage

Current coverage:
- `pkg/api` - 42.4%
- `pkg/editor` - 27.4%
- `pkg/reactor` - 64.6%
- `pkg/config` - 70.6%

---

## What Was Accomplished (Previous Sessions)

### 1. Fixed Self-Check Test Infrastructure

The `self-check` command works correctly for running ExaBGP-style integration tests.

**Bugs Fixed:**
- `cmd/self-check/main.go:410` - Changed `"run"` to `"server"` (zebgp command)
- `cmd/self-check/main.go:434-438` - Read server pipes asynchronously before `Wait()` returns
- `cmd/self-check/main.go:448-449` - Kill client before reading its output
- `pkg/testpeer/peer.go:476-479` - Removed KEEPALIVE bypass so expected messages are matched

### 2. Added Static Route Support Infrastructure

Routes can now be configured per-neighbor and sent when session is established.

**New Types:**
- `pkg/reactor/neighbor.go:18-25` - `StaticRoute` struct

**New Functions:**
- `pkg/reactor/peer.go:309-323` - `sendInitialRoutes()`
- `pkg/reactor/peer.go:326-369` - `buildStaticRouteUpdate()`

### 3. Copied ExaBGP Tests

All tests from `../main/qa/encoding/` and `../main/qa/decoding/` copied to `testdata/`.

---

## Key File Locations

| Purpose | File |
|---------|------|
| Self-check runner | `cmd/self-check/main.go` |
| Test peer | `cmd/zebgp-peer/main.go`, `pkg/testpeer/peer.go` |
| BGP session | `pkg/reactor/session.go` |
| Peer with reconnection | `pkg/reactor/peer.go` |
| Neighbor config | `pkg/reactor/neighbor.go` |
| Config schema | `pkg/config/bgp.go` |
| Config loading | `pkg/config/loader.go` |
| Test data | `testdata/encode/*.ci`, `testdata/decode/*.test` |

---

## Useful Commands

```bash
# Run all unit tests
go test ./... -count=1

# Run self-check integration tests
go run ./cmd/self-check --timeout=15s --all

# Run specific self-check test by index
go run ./cmd/self-check --timeout=15s 0

# List available self-check tests
go run ./cmd/self-check --list

# View expected messages in a test file
go run ./cmd/zebgp-peer --view testdata/encode/ebgp.ci

# Run zebgp-peer in sink mode
go run ./cmd/zebgp-peer --port 1790 --sink
```

---

## Known Issues

1. **Freeform schema**: Doesn't preserve nested structure. `GetList("route")` returns empty for Freeform containers.

2. **Test failures**: Most self-check tests fail due to missing protocol features (MP_REACH, Communities, etc.), not infrastructure issues.

---

## Implementation Plan Reference

See `ZE_IMPLEMENTATION_PLAN.md` for the full implementation plan.

Branch: `main`
