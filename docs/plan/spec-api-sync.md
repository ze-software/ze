# API Process Synchronization

## Problem

When ZeBGP starts, API processes (like `persist`) need time to initialize before BGP sessions begin. Currently:

1. ZeBGP spawns API processes
2. ZeBGP immediately starts BGP peer connections
3. Session becomes Established, EOR is sent
4. API process (e.g., persist) receives "state up", replays routes
5. Routes arrive AFTER EOR → violates RFC 4724

This race causes the `teardown` functional test to be flaky.

## Solution: API Start/Ready Protocol

API processes signal their readiness before BGP negotiation begins.

### Commands

| Command | Direction | Meaning |
|---------|-----------|---------|
| `session api ready` | API → ZeBGP | "I'm initialized and ready" |

### Flow

```
1. ZeBGP starts, parses config
2. ZeBGP counts how many processes are configured (N)
3. ZeBGP spawns API processes
4. ZeBGP waits for N "api ready" signals (with timeout)
5. Each API process initializes and sends "session api ready"
6. ZeBGP receives all ready signals (or timeout expires)
7. ZeBGP starts peer connections
```

### Timeout Behavior

| Scenario | Behavior |
|----------|----------|
| No processes configured | No wait, proceed immediately |
| All processes send ready | Proceed immediately |
| 5s timeout expires | Proceed anyway (log warning) |

**Key Design:** All API processes MUST send `session api ready` after initialization. This is mandatory protocol, not optional.

## Implementation

### Reactor Changes

```go
// pkg/reactor/api_sync.go

type APISyncState struct {
    processCount  int           // number of spawned processes
    readyCount    atomic.Int32  // count of "api ready" received
    readyCh       chan struct{} // closed when all ready
}

// WaitForAPIReady blocks until all API processes are ready or timeout.
func (r *Reactor) WaitForAPIReady() {
    if r.processCount == 0 {
        return // No processes, no wait
    }

    select {
    case <-r.readyCh:
        return
    case <-time.After(5 * time.Second):
        slog.Warn("API timeout", "ready", r.readyCount.Load(), "expected", r.processCount)
    }
}

// SignalAPIReady called when "session api ready" received.
func (r *Reactor) SignalAPIReady() {
    if r.readyCount.Add(1) >= int32(r.processCount) {
        close(r.readyCh)
    }
}
```

### API Process Changes

All API processes must send ready signal after initialization:

```go
func (p *Process) Run() int {
    // Initialize (register commands, load state, etc.)
    p.initialize()

    // Signal ready - MANDATORY
    p.sendCommand("session api ready")

    // Main event loop
    for { ... }
}
```

## Per-Session Synchronization

For routes replayed on reconnect (e.g., persist plugin):

1. Session becomes Established
2. "state up" sent to API processes
3. ZeBGP resets ready count, waits for N "api ready" signals
4. API process replays routes for that peer
5. API process sends `session api ready`
6. All ready signals received → ZeBGP sends EOR

The same mechanism applies globally to all pending sessions.

## Testing

The `teardown` functional test validates this:
- Connection A: route1, EOR, NOTIFY
- Connection B: route1, route2, EOR, NOTIFY
- Connection C: route1, route2, route3, EOR

If API sync works correctly, routes are always sent before EOR.

## Backwards Compatibility

- **Breaking change:** All API processes must now send `session api ready`
- Existing API scripts need updating to send ready signal
- No config changes required - just update the process scripts

## Test Updates Required

### API Script Updates

All `.run` scripts must send `session api ready` after initialization:

```python
# At start of script, after imports
send("session api ready")
```

### Persist Plugin

The persist plugin sends ready signal:
- At startup: register commands → `session api ready`
- On state up: replay routes → `session api ready`

### Test expectations remain unchanged:
- C1: route1, route2, route3, EOR
- Routes MUST arrive before EOR (guaranteed by api ready signal)

### Verification
```bash
# Run test 10 times - should pass 100%
for i in $(seq 1 10); do
    go run ./test/cmd/functional api teardown
done
```

## Implementation Status

### Completed ✅

1. **exabgp_api.py** - Added `ready()` function that sends `session api ready`
2. **All .run scripts** - Call `ready()` after initialization
3. **All .ci files** - EOR reordered to come after routes
4. **peer.go KEEPALIVE skip** - Logic to handle synchronization

### Bug Fixes During Implementation

API tests revealed attribute handling bugs in ZeBGP (not API sync issues):

| Commit | Fix |
|--------|-----|
| `c397f9a` | `buildRIBRouteUpdate`: Use stored LOCAL_PREF instead of hardcoded 100 |
| `238979f` | `AnnounceRoute`: Include all path attributes (MED, LOCAL_PREF, communities) when queuing routes |
| `238979f` | `buildRIBRouteUpdate`: Fix attribute ordering (MED before LOCAL_PREF, MP_REACH_NLRI at end) |

### Test Results

| Tests | Before | After |
|-------|--------|-------|
| API tests | 7/14 | 14/14 |
| Encoding tests | 37/37 | 37/37 |
