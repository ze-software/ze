# Claude Continuation State

**Last Updated:** 2025-12-28

---

## CURRENT STATUS

✅ **Completed:** RFC 8950 Extended Next-Hop (test 6 passes)

### Implementation Summary

| Component | File | Description |
|-----------|------|-------------|
| Capability negotiation | `pkg/bgp/capability/negotiated.go` | ExtendedNextHop map + ExtendedNextHopAFI() |
| Negotiation tests | `pkg/bgp/capability/negotiated_test.go` | 3 tests for ExtNH negotiation |
| Config parsing | `pkg/config/bgp.go` | NexthopFamilyConfig, parseNexthopFamilies() |
| Capability building | `pkg/config/loader.go` | Build ExtendedNextHop capability |
| Route encoding | `pkg/reactor/peer.go` | IPv4UnicastExtNH, IPv4MPLSVPNExtNH flags |
| | | buildMPReachNLRIExtNHUnicast() function |
| | | useExtNHVPN check for VPN routes |

### Files Modified (uncommitted)
- `pkg/bgp/capability/negotiated.go`
- `pkg/bgp/capability/negotiated_test.go`
- `pkg/config/bgp.go`
- `pkg/config/loader.go`
- `pkg/reactor/peer.go`
- `pkg/reactor/peer_test.go`
- `test/data/encode/extended-nexthop.ci`
- `test/data/encode/extended-nexthop.conf`

---

## PREVIOUS STATUS

✅ **Completed:** Protocol updates and plan-to-spec conversion (`215aaa8`)

---

## RECENTLY COMPLETED

### Protocol & Spec Updates (`215aaa8`)

- Added QUALITY MANDATE to ESSENTIAL_PROTOCOLS.md
- Updated prep.md with FIRST rules (git status, continuation, protocols)
- Converted 6 implementation plans to spec format
- Added spec-extended-nexthop.md for RFC 8950
- Moved superseded plans to done/
- Added rfc/rfc8950.txt

### NegotiatedFamilies Refactor (`26ce539`)

Implemented ExaBGP-style pre-computed negotiated families for O(1) access:

| Component | Description |
|-----------|-------------|
| `NegotiatedFamilies` struct | Pre-computed flags for all family types |
| `computeNegotiatedFamilies()` | Extracts flags from capability intersection |
| `Peer.families` | Atomic pointer for lock-free access |
| `safiMUP` constant | Replaces magic number 85 |

**Functions refactored:**
- `sendInitialRoutes()` - Fixed EOR bug (was using local caps, now uses negotiated)
- `sendFlowSpecRoutes()` - Uses `nf.IPv4FlowSpec`, etc.
- `sendVPLSRoutes()` - Uses `nf.L2VPNVPLS`
- `sendMVPNRoutes()` - Uses `nf.IPv4McastVPN`, `nf.IPv6McastVPN`
- `sendMUPRoutes()` - Uses `nf.IPv4MUP`, `nf.IPv6MUP`

**Bug fixed:** EOR was being sent based on LOCAL configured capabilities instead of NEGOTIATED families. Could send EOR for families the peer doesn't support.

**Tests added:**
- `TestComputeNegotiatedFamiliesNil`
- `TestComputeNegotiatedFamiliesBasic` (verifies intersection semantics)
- `TestComputeNegotiatedFamiliesFlowSpecVPN`
- `TestComputeNegotiatedFamiliesVPLS`
- `TestComputeNegotiatedFamiliesMVPN`
- `TestComputeNegotiatedFamiliesMUP`

### Previous Session: FlowSpec Test 7 Fix

| Commit | Feature |
|--------|---------|
| `0af99b6` | Filter routes by negotiated families |
| `3befe4f` | Fix test peer timeout handling |
| `f2578c6` | AS_PATH validation + RFC 5065 constant fix |
| `d20b97c` | ExaBGP-style functional test runner |

---

## FUNCTIONAL TEST STATUS

**Passing:** 27/37 encoding tests (73%)

**Failing tests:**

| Code | Test | Issue |
|------|------|-------|
| 6 | extended-nexthop | RFC 8950 not implemented |
| N | new-v4 | Unknown |
| Q | parity | Unknown |
| R | path-information | ADD-PATH encoding |
| S | prefix-sid | Prefix-SID not implemented |
| T | split | Unknown |
| U | srv6-mup-v3 | MUP timeout (stub) |
| V | srv6-mup | MUP timeout (stub) |
| Z | vpn | Extended-community parsing |
| a | watchdog | Socket permission denied |

---

## KEY FILES

| Purpose | File |
|---------|------|
| NegotiatedFamilies | `pkg/reactor/peer.go` (lines 24-107) |
| Route sending | `pkg/reactor/peer.go:send*Routes()` |
| Tests | `pkg/reactor/peer_test.go` |

---

## NOTES

- `make test`: ✅ All unit tests pass
- `make lint`: ✅ 0 issues
- Atomic pointer provides lock-free O(1) family checks
- Families cleared on session teardown (3 places for robustness)

---

## Resume Point

**Last worked:** 2025-12-28
**Last commit:** `215aaa8` (docs: add quality mandate and convert plans to spec format)
**Session ended:** Clean break

**To resume:**
1. Implement extended-nexthop (RFC 8950) per `plan/spec-extended-nexthop.md`
2. Run `go run ./test/cmd/functional encoding 6` to test
3. Verify with `make test && make lint`
