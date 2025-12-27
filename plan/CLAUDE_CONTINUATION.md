# Claude Continuation State

**Last Updated:** 2025-12-27

---

## CURRENT STATUS

✅ **All P0/P1 items complete.** Remaining work is encode test fixes.

---

## RECENTLY COMPLETED

| Commit | Feature |
|--------|---------|
| `5d8539e` | Process backpressure and respawn limits |
| `af8a705` | BGP collision detection (RFC 4271 §6.8) |
| `2587fdf` | Session-level API commands (status, enable, disable) |
| `777e1f0` | RIB flush/clear API commands |
| `6210707` | API UPDATE receive forwarding to processes |
| `3cf4f92` | EVPN Bytes() encoding for all 5 route types |
| `85bac94` | L3VPN (MPLS VPN) route announcement support |

**Previous P0 items - now complete:**
- ✅ EVPN Bytes() methods
- ✅ Collision detection (RFC 4271 §6.8)
- ✅ FSM bug (ESTABLISHED→IDLE) - fixed, `ae` test passes

**Previous P1 items - now complete:**
- ✅ Process backpressure & respawn limits
- ✅ Session API commands
- ✅ RIB API commands

---

## FUNCTIONAL TEST STATUS

**Passing:** 37/51 tests

**Failing tests:**

| Code | Test | Issue |
|------|------|-------|
| 6 | extended-nexthop | RFC 8950 not implemented |
| 7 | flow-redirect | FlowSpec encoding incomplete |
| L | mvpn | MVPN stub only |
| N | new-v4 | Unknown |
| Q | parity | Unknown |
| R | path-information | ADD-PATH encoding |
| S | prefix-sid | Prefix-SID not implemented |
| T | split | Unknown |
| U | srv6-mup-v3 | MUP stub only |
| V | srv6-mup | MUP stub only |
| Z | vpn | VPN encoding issue |
| a | watchdog | Unknown |
| aj | mup4 | MUP API not implemented |
| ak | mup6 | MUP API not implemented |

---

## PRIORITIES

### 🟢 Low Priority (Encode Tests)

These are edge cases and advanced features:

1. **FlowSpec encoding** - ICMP type/code components
2. **ADD-PATH** - path-information test
3. **Extended nexthop** - RFC 8950
4. **Prefix-SID** - Segment routing
5. **MUP** - Mobile User Plane (stub)
6. **MVPN** - Multicast VPN (stub)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Route handlers | `pkg/api/route.go` |
| UPDATE building | `pkg/reactor/reactor.go` |
| Session handling | `pkg/reactor/session.go` |
| Collision detection | `pkg/reactor/collision_test.go` |
| EVPN encoding | `pkg/bgp/nlri/evpn.go` |
| FSM | `pkg/bgp/fsm/` |

---

## NOTES

- All unit tests pass (`make test`)
- Lint clean (`make lint`)
- 14/51 functional tests fail (mostly advanced features)
- Core BGP functionality complete and tested

---

## Resume Point

**Last worked:** 2025-12-27
**Last commit:** `5d8539e` (process backpressure)
**Session ended:** Clean break

**To resume:**
1. Pick a failing encode test to fix
2. Run `go run ./test/cmd/self-check <code>` to see failure
3. Fix encoding issue
4. Verify with `make test && make lint`
