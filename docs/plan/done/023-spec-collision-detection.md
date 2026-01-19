# Spec: BGP Connection Collision Detection (RFC 4271 §6.8)

**Status:** ✅ COMPLETE (2025-12-27)

## Task
Implement BGP connection collision detection per RFC 4271 Section 6.8.

## RFC Requirements Summary

### When Collision Occurs
Two parallel connections exist between the same pair of BGP speakers where:
- Source IP of connection A = Destination IP of connection B
- Destination IP of connection A = Source IP of connection B

### When to Detect
1. **MUST** examine OpenConfirm state connections upon receipt of OPEN
2. **MAY** examine OpenSent state connections if BGP Identifier is known
3. **Cannot** detect collision in Idle, Connect, or Active states

### Resolution Algorithm
```
Compare BGP Identifiers as 4-octet unsigned integers (host byte order):

IF local_bgp_id < remote_bgp_id:
    Close EXISTING connection (in OpenConfirm)
    Accept NEW connection (with incoming OPEN)
ELSE:
    Close NEW connection (send NOTIFICATION Cease/Collision)
    Keep EXISTING connection
```

### Special Cases
- Collision with **Established** connection: Always close new connection
- Send NOTIFICATION with Error Code **Cease** (6), Subcode **Connection Collision Detection** (7)

---

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof

### From TDD_ENFORCEMENT.md
- Every test MUST document VALIDATES and PREVENTS
- Write test first → run → MUST FAIL → implement → run → MUST PASS
- Table-driven tests for multiple scenarios

### From ExaBGP Reference
ExaBGP handles collision in `peer.py:handle_connection()`:
- If FSM == ESTABLISHED: reject with NOTIFICATION 6/7
- If FSM == OPENCONFIRM: compare router IDs
  - `remote_id < local_id`: reject incoming, keep outgoing
  - `remote_id >= local_id`: close outgoing, accept incoming

---

## Current Architecture Analysis

### Connection Flow (ZeBGP)
```
Reactor.handleConnection(conn)
    ↓
peer.AcceptConnection(conn)
    ↓
session.Accept(conn)
    ↓
session.connectionEstablished(conn)
    ↓
FSM: EventTCPConnectionConfirmed → OpenSent
    ↓
sendOpen() → wait for peer OPEN
```

### Key Files
| File | Purpose |
|------|---------|
| `internal/reactor/reactor.go` | `handleConnection()` - entry point |
| `internal/reactor/peer.go` | `AcceptConnection()` - peer level |
| `internal/reactor/session.go` | `Accept()` - session level, FSM interaction |
| `internal/bgp/fsm/fsm.go` | FSM states and transitions |

### Current Gap
`session.Accept()` simply rejects if connection exists:
```go
if s.conn != nil {
    return ErrAlreadyConnected
}
```
This doesn't implement RFC 4271 §6.8 collision detection.

---

## Implementation Design

### Architecture Decision: Where to Detect?

**Option A: At Reactor Level** (selected)
- Reactor knows about all peers and their states
- Can compare connections before handing to session
- Matches ExaBGP's approach (handle_connection in peer.py)

**Option B: At Session Level**
- Session would need to know about other sessions
- Requires cross-session communication

**Option C: At FSM Level**
- Too low-level, FSM shouldn't manage connections

**Selected: Option A** - Detection at peer/reactor level

### Detection Points

1. **On incoming connection (`handleConnection`):**
   - Check if peer exists
   - Check peer's current FSM state
   - If OpenConfirm or Established: potential collision

2. **On receiving OPEN message (`handleOpen`):**
   - Now we know remote BGP Identifier
   - Can compare with local BGP Identifier
   - Make collision decision

### Key Insight
The collision detection requires knowing the remote BGP Identifier. We learn this from the OPEN message. So detection happens in two phases:

1. **Phase 1 (early reject):** If peer is already ESTABLISHED, reject immediately
2. **Phase 2 (OPEN-based):** After receiving OPEN on new connection, compare BGP IDs

---

## Implementation Steps

### Step 1: Add Collision Detection Method to Session
```go
// DetectCollision checks if incoming connection causes collision.
// Returns (shouldAccept, shouldCloseExisting, error).
// RFC 4271 §6.8
func (s *Session) DetectCollision(remoteBGPID uint32) (bool, bool, error)
```

**Logic:**
1. Get current FSM state
2. If Established → reject new (shouldAccept=false)
3. If OpenConfirm:
   - Compare local BGP ID vs remote BGP ID
   - If local < remote: accept new, close existing
   - If local >= remote: reject new, keep existing
4. Other states: accept new

### Step 2: Modify Session.Accept
Change from simple reject to collision-aware:
```go
func (s *Session) Accept(conn net.Conn) error {
    s.mu.Lock()
    if s.conn != nil {
        // Potential collision - defer to collision detection
        s.mu.Unlock()
        return ErrCollisionPending // New error
    }
    s.mu.Unlock()
    return s.connectionEstablished(conn)
}
```

### Step 3: Add HandleIncomingOpen to Session
After receiving OPEN on incoming connection, invoke collision detection:
```go
func (s *Session) handleOpen(open *message.Open) error {
    // ... existing validation ...

    // If this is incoming and we have outgoing in OpenConfirm:
    // RFC 4271 §6.8 collision detection
    if collision := s.detectCollision(open.RouterID); collision != nil {
        return collision
    }

    // ... continue with normal OPEN handling ...
}
```

### Step 4: Add Peer-Level Collision Tracking
Peer needs to track both incoming and outgoing session attempts:
```go
type Peer struct {
    // ... existing fields ...

    // For collision detection
    incomingConn  net.Conn     // Pending incoming connection
    incomingState fsm.State    // State when incoming arrived
}
```

### Step 5: Wire into Reactor.handleConnection
```go
func (r *Reactor) handleConnection(conn net.Conn) {
    // ... existing peer lookup ...

    // Check for collision
    state := peer.State()
    switch state {
    case fsm.StateEstablished:
        // Always reject - send NOTIFICATION 6/7
        r.rejectConnection(conn, 6, 7, "already established")
        return

    case fsm.StateOpenConfirm:
        // Collision possible - need to compare BGP IDs after OPEN
        // Queue connection for collision resolution
        peer.SetPendingIncoming(conn)
        return

    default:
        // Accept normally
        peer.AcceptConnection(conn)
    }
}
```

### Step 6: Handle Collision Resolution After OPEN
In `handleOpen()`, if we're in collision state:
```go
func (s *Session) handleOpen(open *message.Open) error {
    // Check for pending collision
    if s.hasPendingIncoming() {
        localID := s.settings.LocalBGPID
        remoteID := open.RouterID

        if localID < remoteID {
            // Close this (outgoing) connection, accept incoming
            s.sendNotification(6, 7, nil)
            s.close()
            return s.acceptPendingIncoming()
        } else {
            // Reject incoming, keep this connection
            s.rejectPendingIncoming(6, 7)
        }
    }
    // ... continue normal processing ...
}
```

---

## Codebase Context

### Existing Error Types
```go
var ErrAlreadyConnected = errors.New("already connected")
var ErrNotConnected = errors.New("not connected")
```

### NOTIFICATION Subcodes (internal/bgp/message/notification.go)
```go
CeaseConnectionCollisionResolution = 7
```
Already defined - good.

### FSM States Available
- `fsm.StateIdle`
- `fsm.StateConnect`
- `fsm.StateActive`
- `fsm.StateOpenSent`
- `fsm.StateOpenConfirm`
- `fsm.StateEstablished`

---

## Test Specification

**Test file:** `internal/reactor/collision_test.go`

### Test Cases

```go
// TestCollisionEstablished verifies collision with established session.
// RFC 4271 §6.8: "collision with existing BGP connection that is in
// the Established state causes closing of the newly created connection"
//
// VALIDATES: Incoming connection rejected when peer is ESTABLISHED.
// PREVENTS: Established sessions being disrupted by new connections.
func TestCollisionEstablished(t *testing.T)

// TestCollisionOpenConfirmLocalWins verifies local BGP ID wins.
// RFC 4271 §6.8: "local system closes the newly created BGP connection"
//
// VALIDATES: When local_id > remote_id, incoming is rejected.
// PREVENTS: Wrong connection being kept when local ID is higher.
func TestCollisionOpenConfirmLocalWins(t *testing.T)

// TestCollisionOpenConfirmRemoteWins verifies remote BGP ID wins.
// RFC 4271 §6.8: "local system closes the BGP connection that already
// exists and accepts the BGP connection initiated by the remote system"
//
// VALIDATES: When local_id < remote_id, existing is closed, incoming accepted.
// PREVENTS: Wrong connection being kept when remote ID is higher.
func TestCollisionOpenConfirmRemoteWins(t *testing.T)

// TestCollisionOpenSentNoCollision verifies OpenSent allows new connections.
// RFC 4271 §6.8: "cannot be detected with connections in Idle, Connect, or Active"
//
// VALIDATES: Connections in OpenSent can accept incoming (no collision).
// PREVENTS: Over-aggressive collision detection.
func TestCollisionOpenSentNoCollision(t *testing.T)

// TestCollisionNotificationSent verifies NOTIFICATION is sent.
// RFC 4271 §6.8: "sending NOTIFICATION message with Error Code Cease"
//
// VALIDATES: Rejected connection receives NOTIFICATION 6/7.
// PREVENTS: Silent connection drops without proper BGP signaling.
func TestCollisionNotificationSent(t *testing.T)

// TestCollisionBGPIDComparison verifies ID comparison as uint32.
// RFC 4271 §6.8: "converting them to host byte order and treating
// them as 4-octet unsigned integers"
//
// VALIDATES: BGP IDs compared correctly as unsigned integers.
// PREVENTS: Byte order or signed comparison bugs.
func TestCollisionBGPIDComparison(t *testing.T)
```

---

## Verification Checklist

- [x] Tests written for each collision scenario
- [x] Tests shown to FAIL first
- [x] Implementation makes tests pass
- [x] NOTIFICATION 6/7 sent on collision
- [x] BGP ID comparison correct (uint32, host byte order)
- [x] OpenConfirm collision resolution works both ways
- [x] Established connections always reject new
- [x] `make test` passes
- [x] `make lint` passes
- [x] RFC 4271 §6.8 fully implemented

---

## Estimated Effort

| Component | Lines | Complexity |
|-----------|-------|------------|
| Session collision methods | ~50 | Medium |
| Peer pending connection | ~30 | Low |
| Reactor collision check | ~40 | Medium |
| Tests | ~200 | Medium |
| **Total** | ~320 | Medium |

---

## Open Questions

1. **OpenSent collision:** RFC says MAY examine OpenSent if BGP ID known by other means. Should we implement this?
   - **Recommendation:** No. Only implement MUST (OpenConfirm). Simplifies implementation.

2. **Multiple incoming connections:** What if two incoming connections arrive before OPEN?
   - **Recommendation:** Accept first, reject subsequent with NOTIFICATION 6/7.

3. **Race conditions:** Outgoing connects while processing incoming?
   - **Recommendation:** Use mutex to serialize connection acceptance.

---

**Created:** 2025-12-27
