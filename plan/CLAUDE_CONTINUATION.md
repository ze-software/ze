# Claude Continuation State

**Last Updated:** 2025-12-27 (security review completed)

---

## CURRENT STATUS

✅ **Completed:** Self-check rewrite (ExaBGP-style functional testing)

**New runner:** `go run ./test/cmd/functional <encoding|api> [options] [tests...]`

---

## RECENTLY COMPLETED

### Self-Check Rewrite (This Session)

Implemented ExaBGP-style functional test runner with:

| Component | File | Lines |
|-----------|------|-------|
| State machine | `test/pkg/state.go` | 110 |
| Record tracking | `test/pkg/record.go` | 127 |
| Process exec | `test/pkg/exec.go` | 188 |
| Test container | `test/pkg/tests.go` | 176 |
| Timing cache | `test/pkg/timing.go` | 155 |
| Encoding tests | `test/pkg/encoding.go` | 383 |
| API tests | `test/pkg/api.go` | 345 |
| CLI parsing | `test/pkg/cli.go` | 124 |
| Main runner | `test/cmd/functional/main.go` | 267 |

**Features:**
- State machine for test lifecycle (none → starting → running → success/fail/timeout)
- Concurrent test execution with configurable parallelism
- Timing cache for ETA estimation
- Colorized live progress display
- Nick-based test selection (0-9, A-Z, a-z)
- Edit mode (`--edit`)
- Verbose/quiet modes

**Security Review (completed):**
- ✅ Path traversal protection for `option:file:` directive
- ✅ Path traversal protection for `.run` scripts
- ✅ Process isolation via Setpgid
- ✅ Context timeouts on all execution
- ✅ Proper file permissions (0600/0750)

**Usage:**
```bash
# List tests
go run ./test/cmd/functional encoding --list

# Run specific tests
go run ./test/cmd/functional encoding 4 5 6

# Run all tests
go run ./test/cmd/functional encoding --all

# Makefile targets
make functional           # Run all (encoding + api)
make functional-encoding  # Run encoding tests only
make functional-api       # Run API tests only
```

### Previous Completions

| Commit | Feature |
|--------|---------|
| `5d8539e` | Process backpressure and respawn limits |
| `af8a705` | BGP collision detection (RFC 4271 §6.8) |
| `2587fdf` | Session-level API commands |
| `777e1f0` | RIB flush/clear API commands |

---

## FUNCTIONAL TEST STATUS

**Passing:** 37/51 tests (legacy self-check)

**Failing tests:** (edge cases/advanced features)

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
| New functional runner | `test/cmd/functional/main.go` |
| Test infrastructure | `test/pkg/*.go` |
| Legacy self-check | `test/cmd/self-check/main.go` |
| Route handlers | `pkg/api/route.go` |
| UPDATE building | `pkg/reactor/reactor.go` |

---

## NOTES

- `make test`: ✅ All unit tests pass
- `make lint`: ✅ 0 issues
- New `test/pkg/` package provides reusable test infrastructure
- Legacy `self-check` still works, new `functional` runner available

---

## Resume Point

**Last worked:** 2025-12-27
**Last commit:** `d20b97c` (feat: implement ExaBGP-style functional test runner)
**Session ended:** Clean break

**To resume:**
1. Pick a failing encode test to fix
2. Run `go run ./test/cmd/functional encoding <code>` to test
3. Verify with `make test && make lint`
