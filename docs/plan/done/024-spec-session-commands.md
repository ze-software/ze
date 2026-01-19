# Spec: Session API Commands

**Status:** ✅ COMPLETE (2025-12-27)

## Task
Implement session API commands: ack enable/disable/silence, sync enable/disable, reset, ping, bye

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Every test MUST document VALIDATES and PREVENTS
- Write test first → run → MUST FAIL → implement → run → MUST PASS
- Table-driven tests for multiple scenarios
- One feature at a time, no batching

### From CODING_STANDARDS.md
- Error handling: NEVER ignore errors, wrap with fmt.Errorf + %w
- No panic for error handling
- testify assertions (require for setup, assert for verification)

### From ExaBGP Reference
Session commands are per-connection state (not BGP session state):
- **ack enable**: Send "done" after commands (default)
- **ack disable**: Send "done" for this command, then stop
- **ack silence**: Stop immediately, no response for this command
- **sync enable**: Wait for wire transmission before ACK
- **sync disable**: ACK immediately after RIB update (default)
- **reset**: Clear async queue for this connection
- **ping**: Health check, returns "pong <uuid>"
- **bye**: Client disconnect cleanup

## Codebase Context

**Existing patterns to follow:**
- `internal/plugin/handler.go` - command registration, handler signature
- `internal/plugin/types.go` - ProcessConfig struct
- `internal/plugin/process.go` - Process struct

**Key observation:**
ZeBGP uses Process-based API (stdin/stdout), not socket connections.
Session state (ack, sync) should be per-process, not per-command.

**Current state:**
- Process struct has no ack/sync state
- CommandContext has Process pointer

## Implementation Steps

### Step 1: Add Session State to Process
Add fields to Process struct:
- `ackEnabled bool` (default true)
- `syncEnabled bool` (default false)

Add methods:
- `SetAck(enabled bool)`
- `SetSync(enabled bool)`
- `AckEnabled() bool`
- `SyncEnabled() bool`

### Step 2: Add Session State to CommandContext
CommandContext needs access to current process session state.
Already has `Process *Process` field (from commit context).

### Step 3: Implement Session Command Handlers
```go
func handleSessionAckEnable(ctx *CommandContext, args []string) (*Response, error)
func handleSessionAckDisable(ctx *CommandContext, args []string) (*Response, error)
func handleSessionAckSilence(ctx *CommandContext, args []string) (*Response, error)
func handleSessionSyncEnable(ctx *CommandContext, args []string) (*Response, error)
func handleSessionSyncDisable(ctx *CommandContext, args []string) (*Response, error)
func handleSessionReset(ctx *CommandContext, args []string) (*Response, error)
func handleSessionPing(ctx *CommandContext, args []string) (*Response, error)
func handleSessionBye(ctx *CommandContext, args []string) (*Response, error)
```

### Step 4: Register Commands
In `RegisterSessionHandlers()`:
- `session ack enable`
- `session ack disable`
- `session ack silence`
- `session sync enable`
- `session sync disable`
- `session reset`
- `session ping`
- `session bye`

### Step 5: Wire ACK Behavior
Modify response sending to check `AckEnabled()`:
- If ack disabled, suppress "done" response
- silence = no response for current command either

### Step 6: Update Help Text
Add session commands to `handleSystemHelp`

## Verification Checklist
- [ ] Tests written for Process.SetAck/SetSync methods
- [ ] Tests written for each session command handler
- [ ] Tests shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Help text updated
- [ ] ExaBGP compatibility verified (command syntax matches)

## Test Specification

**Test file:** `internal/plugin/session_test.go`

**Test cases:**
```go
// TestProcessAckState verifies ack state management on Process.
// VALIDATES: Process tracks ack enabled/disabled state
// PREVENTS: Missing ack state, always-on or always-off behavior
func TestProcessAckState(t *testing.T)

// TestProcessSyncState verifies sync state management on Process.
// VALIDATES: Process tracks sync enabled/disabled state
// PREVENTS: Missing sync state, incorrect default
func TestProcessSyncState(t *testing.T)

// TestSessionAckEnable verifies the session ack enable command.
// VALIDATES: Command enables ack and returns success
// PREVENTS: Ack remaining disabled after enable command
func TestSessionAckEnable(t *testing.T)

// TestSessionAckDisable verifies the session ack disable command.
// VALIDATES: Command disables ack after response
// PREVENTS: Ack disabled before response is sent
func TestSessionAckDisable(t *testing.T)

// TestSessionAckSilence verifies the session ack silence command.
// VALIDATES: Command disables ack immediately (no response)
// PREVENTS: Response sent for silence command
func TestSessionAckSilence(t *testing.T)

// TestSessionPing verifies the session ping command.
// VALIDATES: Returns pong with daemon UUID
// PREVENTS: Missing health check endpoint
func TestSessionPing(t *testing.T)
```

---

**Created:** 2025-12-27
