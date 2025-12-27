# Spec: RIB Flush/Clear API Commands

## Task

Implement RIB flush/clear API commands:
- `rib flush out` - Re-send all routes in Adj-RIB-Out to peers
- `rib clear in` - Clear Adj-RIB-In (received routes)
- `rib clear out` - Withdraw all routes from Adj-RIB-Out (sends withdrawals to peers)

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Write test FIRST with VALIDATES/PREVENTS documentation
- Run test → MUST FAIL (paste failure output)
- Write minimum implementation to pass
- Run test → MUST PASS (paste pass output)
- ONE feature at a time

### From CODING_STANDARDS.md
- Never ignore errors (no `_` for errors)
- Use `fmt.Errorf` with `%w` for wrapping
- Table-driven tests with testify

## ExaBGP Reference

**File:** `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/reactor/api/command/rib.py`

**Behavior:**
- `flush_adj_rib_out` - Calls `reactor.neighbor_rib_resend()` to re-send routes
- `clear_adj_rib` (in) - Calls `reactor.neighbor_rib_in_clear()` to clear received routes
- `clear_adj_rib` (out) - Calls `reactor.neighbor_rib_out_withdraw()` to withdraw routes

## Codebase Context

**Existing files:**
- `pkg/api/handler.go` - Already has `handleRIBShowIn`, `handleRIBShowOut`
- `pkg/rib/incoming.go` - `IncomingRIB` with `ClearPeer()` method
- `pkg/rib/outgoing.go` - `OutgoingRIB` with `FlushAllPending()`, `GetSentRoutes()`
- `pkg/reactor/reactor.go` - Has `ribIn` and `ribOut` fields

**Pattern to follow:**
```go
// From handler.go - existing RIB show handlers
func handleRIBShowIn(ctx *CommandContext, args []string) (*Response, error) {
    stats := ctx.Reactor.RibInStats()
    // ...
}
```

**RIB methods available:**
- `IncomingRIB.ClearPeer(peerID string) []*Route` - Clear routes for a peer
- `OutgoingRIB.FlushAllPending() []*Route` - Get and clear pending routes
- `OutgoingRIB.GetSentRoutes() []*Route` - Get routes already sent

## Implementation Steps

### Step 1: Add Reactor methods for RIB operations

**File:** `pkg/reactor/reactor.go`

Add methods:
```go
// ClearRibIn clears all routes in Adj-RIB-In.
// Returns count of routes cleared.
func (r *Reactor) ClearRibIn() int

// ClearRibOut withdraws all routes from Adj-RIB-Out.
// Queues withdrawals to be sent to peers.
// Returns count of routes withdrawn.
func (r *Reactor) ClearRibOut() int

// FlushRibOut re-queues all sent routes for re-announcement.
// Used to force resend of all routes to peers.
// Returns count of routes flushed.
func (r *Reactor) FlushRibOut() int
```

### Step 2: Add API handlers

**File:** `pkg/api/handler.go`

Add handlers:
```go
func handleRIBFlushOut(ctx *CommandContext, _ []string) (*Response, error)
func handleRIBClearIn(ctx *CommandContext, _ []string) (*Response, error)
func handleRIBClearOut(ctx *CommandContext, _ []string) (*Response, error)
```

### Step 3: Register commands

**File:** `pkg/api/handler.go`

Add to `RegisterHandlers`:
```go
d.Register("rib flush out", handleRIBFlushOut, "Re-send all routes to peers")
d.Register("rib clear in", handleRIBClearIn, "Clear Adj-RIB-In")
d.Register("rib clear out", handleRIBClearOut, "Withdraw all routes from peers")
```

## Test Specification

**Test file:** `pkg/api/handler_test.go`

### Test Cases

```go
// TestRIBClearIn verifies clearing Adj-RIB-In removes all received routes.
//
// VALIDATES: API command correctly clears incoming route storage.
//
// PREVENTS: Memory leaks from accumulated routes, stale route data.
func TestRIBClearIn(t *testing.T)

// TestRIBClearOut verifies clearing Adj-RIB-Out queues withdrawals.
//
// VALIDATES: Withdrawing all sent routes generates proper cleanup.
//
// PREVENTS: Orphaned routes in peer tables after clear.
func TestRIBClearOut(t *testing.T)

// TestRIBFlushOut verifies flushing re-queues routes for resend.
//
// VALIDATES: All previously sent routes are queued for re-announcement.
//
// PREVENTS: Route sync failures after peer reconnection.
func TestRIBFlushOut(t *testing.T)
```

## Verification Checklist

- [ ] Tests written for all 3 commands
- [ ] Tests FAIL before implementation
- [ ] Reactor methods added
- [ ] API handlers added
- [ ] Commands registered
- [ ] Tests PASS after implementation
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Response format matches existing handlers (JSON with counts)

## Response Format

Match existing `handleRIBShowIn` pattern:
```json
{
  "status": "ok",
  "routes_cleared": 42
}
```

---

**Created:** 2025-12-27
