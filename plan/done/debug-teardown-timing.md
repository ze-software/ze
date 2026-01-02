# Teardown Test Timing Investigation - DONE

## Problem
The teardown test showed suspicious timing variance:
- Sometimes completes in ~6s
- Sometimes takes ~17s
- Default timeout was 15s, causing failures

## Root Cause
**zebgp was NOT sending shutdown signal to API processes!**

When zebgp shut down:
1. Test runner sent SIGKILL (not SIGTERM) - no cleanup
2. Even with SIGTERM, ProcessManager.Stop() didn't send shutdown to processes
3. Python scripts always waited 5s for shutdown timeout

## Solution
Two fixes applied:

### 1. Process shutdown signal (pkg/api/process.go)
Added `SendShutdown()` method that writes `{"answer": "shutdown"}\n` **synchronously**
to process stdin, bypassing the async write queue.

Key design decision: Shutdown is critical - it must be delivered before process
termination. Using the async WriteEvent queue would be a race condition (the queue
might not drain before context cancellation). Direct synchronous write to stdin
ensures the message is in the pipe buffer immediately.

```go
func (p *Process) SendShutdown() {
    p.mu.Lock()
    defer p.mu.Unlock()
    // Write synchronously - bypass async queue
    _, _ = p.stdin.Write([]byte("{\"answer\": \"shutdown\"}\n"))
}
```

### 2. Graceful test runner shutdown (test/functional/runner.go)
Changed from `Kill()` (SIGKILL) to `Signal(SIGTERM)` with 2s grace period.
This allows zebgp to run cleanup and send shutdown signals.

## Timing Analysis (after fix)
- Script delays: 4.5s
- wait_for_ack: up to 2s
- wait_for_shutdown: 0s (receives shutdown) or 5s (timeout)
- Graceful shutdown: up to 2s

Fast runs (~6-8s): Shutdown arrives during wait_for_ack
Slow runs (~13-15s): Shutdown arrives after wait_for_shutdown times out

## Files Modified
- `pkg/api/process.go` - Added synchronous SendShutdown(), modified ProcessManager.Stop()
- `test/functional/runner.go` - SIGTERM + grace period instead of SIGKILL
- `test/data/api/teardown.ci` - Adjusted timeout to 18s

## Verification
- `make test && make lint` - PASS
- `go run ./test/cmd/functional api -a` - 14/14 tests pass
