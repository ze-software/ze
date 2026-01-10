# Spec: Process Backpressure and Respawn Limits

## Task
Implement process backpressure queue and respawn limits for API processes

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Check ExaBGP reference in `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Write test FIRST with VALIDATES/PREVENTS documentation
- Run test - MUST FAIL (show failure output)
- Write minimum implementation to pass
- Run test - MUST PASS (show pass output)
- ONE feature at a time

### From PROCESS_PROTOCOL.md
- Write queue backpressure: HIGH_WATER=1000, LOW_WATER=100
- When queue exceeds high water: drop events, log warning
- Resume when queue drains to low water mark
- Respawn limit: maximum 5 respawns per minute
- If exceeded, process disabled until reload

## Codebase Context

### Existing Files
- `pkg/plugin/process.go` - Current Process and ProcessManager
- `pkg/plugin/types.go` - ProcessConfig struct

### ExaBGP Reference
- `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/reactor/api/processes.py`
- Lines 126-132: Queue constants and respawn timemask
- Lines 853-879: Queue stats methods
- Lines 260-361: Respawn tracking logic

### Key ExaBGP Constants
```python
WRITE_QUEUE_HIGH_WATER = 1000  # Pause writes when exceeded
WRITE_QUEUE_LOW_WATER = 100    # Resume when drops below
respawn_timemask = 0xFFFFFF - 0b111111  # ~63 second window
respawn_number = 5  # Max respawns per window
```

## Implementation Steps

### Step 1: Add Write Queue to Process

**Test:** `TestProcessWriteQueueBackpressure`
- VALIDATES: Events dropped when queue full, warning logged
- PREVENTS: Memory exhaustion from slow consumers

**Implementation:**
- Add `writeQueue chan []byte` to Process
- Add `queueDropped atomic.Uint64` counter
- Modify `WriteEvent()` to use non-blocking send

### Step 2: Add Queue Stats Methods

**Test:** `TestProcessQueueStats`
- VALIDATES: Queue size and dropped count accessible
- PREVENTS: Inability to monitor backpressure

**Implementation:**
- `QueueSize() int` - current items in queue
- `QueueDropped() uint64` - total events dropped

### Step 3: Add Respawn Tracking to ProcessManager

**Test:** `TestProcessRespawnLimit`
- VALIDATES: Process disabled after 5 respawns in 60s
- PREVENTS: Infinite respawn loops consuming resources

**Implementation:**
- Add `respawnTimes map[string][]time.Time` to ProcessManager
- Add `disabled map[string]bool` to ProcessManager
- Add `Respawn(name string) error` method

### Step 4: Integrate Respawn with Monitor

**Test:** `TestProcessMonitorRespawn`
- VALIDATES: Crashed process auto-respawns if enabled
- PREVENTS: Silent process failures

**Implementation:**
- Modify `monitor()` goroutine to call respawn on exit
- Add `RespawnEnabled bool` to ProcessConfig

## Verification Checklist
- [ ] Tests written and shown to FAIL first
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Queue stats accessible via API
- [ ] Respawn limit enforced (5 per minute)
- [ ] Dropped events logged as warnings
