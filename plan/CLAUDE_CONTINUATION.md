# Claude Continuation State

**Last Updated:** 2025-12-28

---

## CURRENT STATUS

✅ **Completed:** ADD-PATH encoding support (test R passes)

---

## RECENTLY COMPLETED

### ADD-PATH Encoding (This Session)

Implemented full ADD-PATH (RFC 7911) support for static routes:

| Component | Change |
|-----------|--------|
| Config parsing | Duplicate prefixes via `#N` suffix in `AddListEntry` |
| NLRI interface | `Pack(ctx *PackContext)` method for capability-aware encoding |
| NegotiatedFamilies | Added `IPv4UnicastAddPath`, `IPv6UnicastAddPath` flags |
| Static route sending | `buildStaticRouteUpdate` and `buildGroupedUpdate` use `Pack(ctx)` |
| CommitService | Uses `Pack(ctx)` for API-announced routes |

**Files modified:**
- `pkg/config/parser.go` - Duplicate key handling
- `pkg/config/bgp.go` - Strip `#N` suffix, add-path constants
- `pkg/config/serialize.go` - Strip suffix on output
- `pkg/config/loader.go` - ADD-PATH capability creation
- `pkg/bgp/nlri/*.go` - Pack(ctx) methods
- `pkg/reactor/peer.go` - NegotiatedFamilies ADD-PATH flags, capability-aware packing
- `pkg/reactor/session.go` - Populate Negotiated.AddPath
- `pkg/rib/commit.go` - packContext helper

### Previous Completions

| Commit | Feature |
|--------|---------|
| `d20b97c` | ExaBGP-style functional test runner |
| `5d8539e` | Process backpressure and respawn limits |
| `af8a705` | BGP collision detection (RFC 4271 §6.8) |

---

## FUNCTIONAL TEST STATUS

**Passing:** 38/51 tests (test R now passes)

**Failing tests:** (edge cases/advanced features)

| Code | Test | Issue |
|------|------|-------|
| 6 | extended-nexthop | RFC 8950 not implemented |
| 7 | flow-redirect | FlowSpec encoding incomplete |
| L | mvpn | MVPN stub only |
| N | new-v4 | Unknown |
| Q | parity | Unknown |
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

### Low Priority (Encode Tests)

These are edge cases and advanced features:

1. **FlowSpec encoding** - ICMP type/code components
2. **Extended nexthop** - RFC 8950
3. **Prefix-SID** - Segment routing
4. **MUP** - Mobile User Plane (stub)
5. **MVPN** - Multicast VPN (stub)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Functional runner | `test/cmd/functional/main.go` |
| NLRI packing | `pkg/bgp/nlri/pack.go` |
| Static route sending | `pkg/reactor/peer.go` |
| Route handlers | `pkg/api/route.go` |
| UPDATE building | `pkg/rib/commit.go` |

---

## Resume Point

**Last worked:** 2025-12-28
**Last commit:** (pending - ADD-PATH encoding)
**Session ended:** Clean break

**To resume:**
1. Pick a failing encode test to fix
2. Run `go run ./test/cmd/functional encoding <code>` to test
3. Verify with `make test && make lint`
