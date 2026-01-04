# Test C (Teardown) Fix - Analysis and Proposed Solution

## Status: INCOMPLETE - Connection Lifecycle Issue

**Current Test Results:**
- Encoding tests: 37/37 (100%) ✓
- API tests: 13/14 (92.9%) - Only test C failing

## Latest Session Progress (2026-01-04)

### Key Discovery: sendingInitialRoutes Flag Race

When connection A tears down while sendInitialRoutes() is sleeping:
1. `sendInitialRoutes()` for A sets flag to 1, sleeps 500ms
2. Teardown arrives, closes connection A
3. `session.Run()` returns, `runOnce()` returns
4. Connection B establishes immediately
5. `sendInitialRoutes()` for B tries `CompareAndSwap(0, 1)` - **FAILS** because A's goroutine still holds flag
6. B's sendInitialRoutes() is **SKIPPED** - no EOR sent
7. A's goroutine eventually finishes, sets flag to 0

### Fixes Applied This Session

1. **sendingInitialRoutes flag reset in runOnce defer** (`pkg/reactor/peer.go`):
   ```go
   defer func() {
       p.sendingInitialRoutes.Store(0)  // Reset for next session
       // ... rest of cleanup
   }()
   ```
   This ensures the flag is cleared when the session ends, not when sendInitialRoutes finishes.

2. **Queue teardown when sendInitialRoutes running** (`pkg/reactor/peer.go`):
   ```go
   func (p *Peer) Teardown(subcode uint8) {
       if p.sendingInitialRoutes.Load() == 1 {
           // Queue teardown for sendInitialRoutes to process
           p.opQueue = append(p.opQueue, PeerOp{Type: PeerOpTeardown, ...})
           return
       }
       // Otherwise execute immediately
   }
   ```
   This ensures EOR is sent before NOTIFICATION (proper BGP sequencing).

3. **Restored original test script timing** (`test/data/api/teardown.run`):
   - Previous session incorrectly changed timing from 1.5s to 0.3s
   - Restored to original timing with proper comments

### Current Behavior After Fixes
- Connection A: route1, EOR, NOTIFICATION ✓ (fixed by teardown queuing)
- Connection B: route1 only, then closes (no EOR, no route2) ✗
- Connection C: route1 only, then closes (no EOR, no route2/route3) ✗

### Remaining Issue: Route Order Mismatch

The test expects specific message ordering per connection:
```
B1: route1, route2, EOR  (then B2: notification)
C1: route1, route2, route3, EOR
```

But ZeBGP sends:
```
B: route1 (persist replay at T+0), EOR (at T+500ms), route2 (at T+2000ms from script)
```

**The 500ms sendInitialRoutes delay ends BEFORE route2 arrives from the test script.**

Test script timing:
- T=0.0: route1
- T=0.5: teardown 1 → connection A closes
- T=2.0: route2 → arrives 1.5s after B establishes, but EOR was sent at 500ms!

The test peer receives route1, then EOR (wrong order), then sees mismatch.

### Hypotheses for Remaining Failure
1. **Route order expectation** - CI expects route2 BEFORE EOR, but EOR is sent first
2. **Test peer sequence logic** - May end sequence after seeing "wrong" message
3. **Persist not storing route2** - Route2 might not be stored for replay on C

### Next Steps
1. **Adjust test timing** - Send route2 within 500ms of connection B establishing
2. **Or extend sendInitialRoutes delay** - Wait longer for script routes
3. **Or use proper API sync** - Wait for explicit "ready" signal before EOR
4. **Verify persist storage** - Ensure route2 is stored when sent on connection B

## Problem Summary

Test C tests BGP session teardown and reconnection with route persistence. After teardown, connections B and C only receive route1 (from persist replay) and then close immediately without sending EOR or receiving route2/route3.

## Root Cause Analysis

The issue is **session object reuse after teardown**. When connection A tears down:
1. `session.Teardown()` sends NOTIFY, closes conn, fires ManualStop
2. `ErrTeardown` is sent to `errChan`
3. Connection B arrives BEFORE `session.Run()` exits
4. `Accept()` is called on the SAME session object
5. The session has stale state (errChan may have ErrTeardown, FSM may be in wrong state)
6. Connection B establishes but closes prematurely

### Timing Race

```
Connection A:
  sendInitialRoutes sleeps 500ms
  processes teardown, sends EOR, NOTIFY
  session.Teardown() fires ManualStop
  errChan <- ErrTeardown

Connection B (arrives during A's teardown):
  Accept() called on OLD session
  errChan drained (may miss ErrTeardown if sent after drain)
  OPEN exchange happens
  FSM transitions to Established
  persist replays route1
  Run() loop eventually sees ErrTeardown or stale state
  Connection closes without EOR
```

## Attempted Fixes (All Partial)

1. **errChan draining in Accept()** - Drains before and after connectionEstablished, but race with concurrent Teardown
2. **tearingDown flag** - Set in Teardown, checked in Accept, but timing window exists
3. **FSM state check** - Too strict, blocks valid first connection
4. **Peer state ordering** - setState(Connecting) before/after Teardown timing issues

## Proper Solution

Don't reuse session objects. Each connection should use a fresh session:

### Option A: Session per Connection (Recommended)
Create new session in Accept() when accepting on a torn-down session:
```go
func (s *Session) Accept(conn net.Conn) error {
    if s.tearingDown.Load() {
        // Don't accept on torn-down session
        return ErrSessionTearingDown
    }
    // ... rest of Accept
}
```

Combined with proper handling in handleConnection:
```go
if err := peer.AcceptConnection(conn); err != nil {
    if errors.Is(err, ErrSessionTearingDown) {
        // Session is torn down, wait and retry or close
        // The new session will be available soon
    }
    conn.Close()
}
```

### Option B: Fresh errChan per Connection
Reset errChan when accepting new connection:
```go
func (s *Session) Accept(conn net.Conn) error {
    // Create fresh errChan for new connection
    s.errChan = make(chan error, 2)
    // ... rest of Accept
}
```

### Option C: Separate Read/Write Loops
Use separate goroutines for reading and writing, with proper synchronization. This allows sendInitialRoutes to complete even if Read encounters issues.

## Current Test Status

- Connection A: route1, EOR, NOTIFY ✓
- Connection B: route1 only, closes without EOR ✗
- Connection C: route1 only, closes without EOR ✗

## Test Timing Requirements

The test script sends route2/route3 AFTER teardown. For the test to pass:
- Route2/3 must arrive BEFORE sendInitialRoutes sends EOR
- Currently 500ms delay is used, but timing is unreliable

## Changes Made Across Sessions

### Previous Sessions
1. **Per-peer API sync infrastructure** (pkg/reactor/peer.go):
   - Added `ResetAPISync()`, `SignalAPIReady()`, `waitForAPISync()` methods
   - Added `apiSyncExpected`, `apiSyncReady`, `apiSyncCount` fields

2. **Session race condition mitigations** (pkg/reactor/session.go):
   - Added `tearingDown` atomic flag
   - Added errChan draining in Accept()
   - Added double-drain after connectionEstablished()

3. **Persist plugin update** (pkg/api/persist/persist.go):
   - Changed to send peer-specific ready signal: `peer <addr> session api ready`

4. **Report fix** (test/functional/report.go):
   - Fixed report to not use Nick-based connection offset for multi-connection tests

### This Session (2026-01-04)
5. **sendInitialRoutes flag reset in runOnce defer** (pkg/reactor/peer.go:709-719):
   - Added `p.sendingInitialRoutes.Store(0)` in defer block
   - Ensures new session can run sendInitialRoutes even if old goroutine still running

6. **Queue teardown when sendInitialRoutes running** (pkg/reactor/peer.go:576-591):
   - Modified `Teardown()` to check `sendingInitialRoutes.Load() == 1`
   - If running, queue teardown to opQueue instead of executing immediately
   - Ensures EOR is sent before NOTIFICATION

7. **Restored original test timing** (test/data/api/teardown.run):
   - Reverted incorrect 0.3s timing back to original 1.5s delays
   - Added documentation comments explaining test flow

8. **Updated architecture docs** (.claude/zebgp/api/ARCHITECTURE.md):
   - Added persist plugin documentation
   - Added API sync protocol description

## Recommended Next Steps

1. **Implement proper session isolation** - Create new session per connection instead of reusing
2. **Implement API sync mechanism** - Proper handshake between persist plugin and sendInitialRoutes
3. **Update test design** - Make test C more robust with explicit sync points
