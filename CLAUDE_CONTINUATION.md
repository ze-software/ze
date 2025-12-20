# ZeBGP Development Continuation Notes

## Last Session: 2025-12-20

### What Was Accomplished

#### 1. Fixed Self-Check Test Infrastructure
The `self-check` command now works correctly for running ExaBGP-style integration tests.

**Bugs Fixed:**
- `cmd/self-check/main.go:410` - Changed `"run"` to `"server"` (zebgp command)
- `cmd/self-check/main.go:434-438` - Read server pipes asynchronously before `Wait()` returns
- `cmd/self-check/main.go:448-449` - Kill client before reading its output
- `pkg/testpeer/peer.go:476-479` - Removed KEEPALIVE bypass so expected messages are matched

**Test Command:**
```bash
go build ./cmd/... && ./cmd/self-check/self-check --timeout=15s --all
```

#### 2. Added Static Route Support Infrastructure
Routes can now be configured per-neighbor and sent when session is established.

**New Types:**
- `pkg/reactor/neighbor.go:18-25` - `StaticRoute` struct with Prefix, NextHop, Origin, LocalPreference, MED

**New Functions:**
- `pkg/reactor/peer.go:309-323` - `sendInitialRoutes()` sends routes on session establishment
- `pkg/reactor/peer.go:326-369` - `buildStaticRouteUpdate()` builds UPDATE messages

**Config Loader:**
- `pkg/config/loader.go:104-120` - Converts `StaticRouteConfig` to `reactor.StaticRoute`

### What Remains To Be Done

#### Priority 1: Config-Based Route Announcements (Partial)
The infrastructure is in place but **Freeform parsing doesn't extract nested route data**.

**Current Limitation:**
```go
// In pkg/config/bgp.go:77-78
Field("static", Freeform()),  // Stores "route 10.0.0.0/24" as key, loses nested data
```

**What happens:**
- `static { route 10.0.0.0/24 { next-hop 10.0.1.254; } }` is parsed
- `tree.Get("route 10.0.0.0/24")` returns `"true"`
- But `next-hop` value is lost because Freeform flattens nested structures

**Solutions (pick one):**
1. Parse Freeform data manually in `parseNeighborConfig()` - complex regex/tokenizing
2. Create custom schema type that handles both block and inline syntax
3. Only support block syntax - change to `List(TypePrefix, ...)` schema

**Test Config Formats:**
```
# Block syntax (would work with List schema):
static {
    route 10.0.0.0/24 {
        next-hop 10.0.1.254;
    }
}

# Inline syntax (ExaBGP style, requires Freeform):
static {
    route 10.0.0.0/24 next-hop 10.0.1.254 local-preference 200;
}
```

See `etc/zebgp/parse-simple-v4.conf` for examples of both syntaxes.

#### Priority 2: Add ExaBGP Tests
Copy tests from `../main/qa/encoding/` and `../main/qa/decoding/` to `testdata/`.

**ExaBGP test locations:**
```
/Users/thomas/Code/github.com/exa-networks/exabgp/main/qa/encoding/  # 39 tests
/Users/thomas/Code/github.com/exa-networks/exabgp/main/qa/decoding/  # 19 tests
```

**Test format (.ci files):**
```
option:file:config-file.conf
option:asn:65000
1:cmd:announce route 10.0.0.0/24 next-hop 10.0.1.254
1:raw:FFFFFFFF...:002F:02:...  # Expected bytes
1:json:{...}  # Expected JSON (not yet used)
```

#### Priority 3: Improve Test Coverage
Current coverage:
- `pkg/api` - 42.4%
- `pkg/editor` - 27.4%
- `pkg/reactor` - 64.6%
- `pkg/config` - 70.6%

### Key File Locations

| Purpose | File |
|---------|------|
| Self-check runner | `cmd/self-check/main.go` |
| Test peer (BGP server for testing) | `cmd/zebgp-peer/main.go`, `pkg/testpeer/peer.go` |
| BGP session handling | `pkg/reactor/session.go` |
| Peer with reconnection | `pkg/reactor/peer.go` |
| Neighbor config | `pkg/reactor/neighbor.go` |
| Config schema | `pkg/config/bgp.go` |
| Config loading | `pkg/config/loader.go` |
| Test data | `testdata/*.ci`, `testdata/*.conf` |

### Code Flow: Session Establishment

1. `Reactor.Start()` starts peers
2. `Peer.run()` creates Session and connects
3. `Session.Connect()` establishes TCP
4. `Session.connectionEstablished()` starts FSM
5. FSM transitions to Established
6. `Peer.runOnce():251-259` callback fires on state change
7. `Peer.sendInitialRoutes()` sends static routes (line 255)

### Code Flow: Self-Check Test

1. `main()` parses flags, loads tests from `testdata/*.ci`
2. `Runner.Run()` iterates tests
3. `Runner.runTest()`:
   - Writes expect file from test.Options + test.Expects
   - Starts zebgp-peer (server) with expect file
   - Starts zebgp (client) with config file
   - Reads outputs asynchronously
   - Waits for zebgp-peer to exit
   - Checks for "successful" in output

### Environment Variables

```bash
exabgp_tcp_port=1790      # Override BGP port (default 179)
exabgp_tcp_bind=          # Empty = connect mode (not listen)
```

### Useful Commands

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
go run ./cmd/zebgp-peer --view testdata/conf-ebgp.ci

# Run zebgp-peer in sink mode (accept everything)
go run ./cmd/zebgp-peer --port 1790 --sink

# Manual test run
go build -o /tmp/zebgp ./cmd/zebgp
go build -o /tmp/zebgp-peer ./cmd/zebgp-peer
/tmp/zebgp-peer --port 1790 --asn 65000 testdata/conf-ebgp.ci &
exabgp_tcp_port=1790 exabgp_tcp_bind= /tmp/zebgp server testdata/conf-ebgp.conf
```

### Implementation Plan Reference

See `ZE_IMPLEMENTATION_PLAN.md` for the full implementation plan. Key sections:
- Lines 2100-2188: Testing infrastructure (Phase 13)
- Lines 1900-2099: What's been completed (P0-P3)

### Known Issues

1. **Pipe reading in self-check**: Must read stdout/stderr before `cmd.Wait()` returns, otherwise data is lost. Fixed by reading in goroutines started before Wait.

2. **Freeform schema**: Doesn't preserve nested structure. `GetList("route")` returns empty for Freeform containers.

3. **Test config simplified**: Current `testdata/conf-ebgp.ci` only tests KEEPALIVE, not UPDATE messages. Full UPDATE testing requires either:
   - Working config-based announcements, or
   - API-based route injection during test

### Git Status

Latest commit: `7e8ef0d` - "Fix self-check infrastructure and add static route support"

Branch: `main`
