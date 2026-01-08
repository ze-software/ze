# Spec: api-receive-update

**Status:** ✅ COMPLETE (2025-12-27)

## Task
Implement API receive update forwarding to processes

## Problem
The `check` test times out because ZeBGP doesn't forward received UPDATEs to API processes.

**Config requirement:**
```
api {
    receive { parsed; update; }
}
```

**Expected output to process stdin:**
```
neighbor 127.0.0.1 receive update start
neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100
neighbor 127.0.0.1 receive update end
```

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- Run functional test FIRST to verify feature status
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Test documentation: VALIDATES + PREVENTS comments mandatory
- Show test failure output BEFORE implementation
- One feature at a time, no batching

### From COMMANDS.md
- Text format: `neighbor <ip> receive update announced <nlri> next-hop <nh> <attrs>`
- Uses ExaBGP text encoder format (not JSON for this test)

## Codebase Context

| File | Purpose |
|------|---------|
| `pkg/reactor/session.go` | `handleUpdate()` - hook point for forwarding |
| `pkg/api/process.go` | `WriteEvent()` - sends to process stdin |
| `pkg/api/types.go` | `ProcessConfig` - needs receive config |
| `pkg/reactor/reactor.go` | `APIProcessConfig` - needs receive config |
| `pkg/config/loader.go` | Parses api block |

### ExaBGP Reference
- `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/reactor/api/response/text.py`
- Format: `neighbor {ip} {direction} update announced {nlri.extensive()} next-hop {nh}{attrs}`

## Implementation Steps

### Phase 1: Config Parsing

1. **Add receive config to APIProcessConfig**
   - Add `ReceiveUpdate bool` field to `pkg/reactor/reactor.go:APIProcessConfig`
   - Add `ReceiveUpdate bool` field to `pkg/api/types.go:ProcessConfig`

2. **Parse receive block in config**
   - In `pkg/config/loader.go`, parse `api { receive { update; } }`
   - Set `ReceiveUpdate = true` when present

### Phase 2: Update Forwarding

3. **Add UpdateReceiver interface to reactor**
   - Interface for forwarding received updates to interested parties
   - Reactor stores list of receivers

4. **Implement text formatter for received updates**
   - Format: `neighbor <ip> receive update start\n`
   - Format: `neighbor <ip> receive update announced <prefix> next-hop <nh> <attrs>\n`
   - Format: `neighbor <ip> receive update end\n`

5. **Hook into handleUpdate() in session.go**
   - After validation, parse UPDATE into routes
   - Call reactor's ForwardUpdate() method
   - Reactor forwards to all processes with ReceiveUpdate=true

### Phase 3: Integration

6. **Wire API server to receive updates**
   - ProcessManager implements UpdateReceiver
   - Routes updates to processes with ReceiveUpdate=true

## Test Strategy

Since this requires full integration (session + process), use functional test:
```bash
go run ./test/cmd/self-check ae
```

Unit tests for:
- Text formatter function (TestFormatReceivedUpdate)
- Config parsing (TestAPIReceiveConfig)

## Verification Checklist

- [ ] `TestFormatReceivedUpdate` written and shown to FAIL first
- [ ] Text formatter implemented and test passes
- [ ] `TestAPIReceiveConfig` written and shown to FAIL first
- [ ] Config parsing implemented and test passes
- [ ] `handleUpdate()` forwards to processes
- [ ] `go run ./test/cmd/self-check ae` passes
- [ ] `make test` passes
- [ ] `make lint` passes
