# Spec: exabgp-bridge-muxconn

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Task

Fix the ExaBGP bridge runtime I/O to speak MuxConn wire format after the 5-stage plugin protocol. Currently both directions silently fail: commands are dropped by ze's MuxConn (missing `#<id>` prefix), and events are dropped by the bridge (can't parse `#<id>` prefix as JSON).

After this fix, the bridge can send commands (including `peer <addr> flush`) and receive events. The forward-barrier spec's Phase 4 (transparent flush in bridge) depends on this.

**Discovery context:** Found during implementation of `spec-forward-barrier.md` Phase 4. The bridge was written before MuxConn adoption and never updated. ExaBGP compatibility tests only cover encoding, not bridge runtime I/O.

## Required Reading

- [ ] `pkg/plugin/rpc/mux.go` -- MuxConn readLoop, wire format `#<id> method json`
- [ ] `pkg/plugin/rpc/framing.go` -- FrameReader/FrameWriter (newline-delimited)
- [ ] `pkg/plugin/rpc/conn.go` -- Conn wraps FrameReader/FrameWriter, takes `net.Conn`
- [ ] `internal/exabgp/bridge/bridge.go` -- bridge runtime, 5-stage protocol, three goroutines
- [ ] `internal/exabgp/bridge/bridge_command.go` -- ExaBGP to ze command translation
- [ ] `internal/exabgp/bridge/bridge_event.go` -- ze to ExaBGP event translation
- [ ] `internal/component/plugin/process/process.go` -- InitConns creates MuxConn after stage 5
- [ ] `internal/component/plugin/process/delivery.go` -- deliverViaConn sends events via SendDeliverBatch
- [ ] `internal/component/plugin/ipc/rpc.go` -- SendDeliverBatch uses CallBatchRPC

## Current Behavior (MANDATORY)

**Commands (bridge to ze):** Bridge writes raw text (`peer 10.0.0.1 update text ...`) to os.Stdout. Ze's MuxConn readLoop checks `strings.HasPrefix(line, "#")`, fails, logs warning, increments consecutiveBad counter, drops the line. After 100 consecutive bad lines, connection is closed.

**Events (ze to bridge):** Ze sends `#<id> ze-plugin-callback:deliver-batch {"events":["..."]}` via MuxConn. Bridge reads line from os.Stdin, calls `json.Unmarshal()` on the full line including `#<id>` prefix. Parse fails. Event logged as warning and dropped.

**Both directions are silently non-functional.** The bridge only works for the 5-stage protocol (raw text, before MuxConn is created). After stage 5, no data flows in either direction.

**Behavior to preserve:**
- 5-stage startup protocol (raw text, before MuxConn)
- ExaBGP to ze command translation (`ExabgpToZebgpCommand`)
- Ze to ExaBGP event translation (`ZebgpToExabgpJSON`)
- Existing bridge unit tests (translation functions)

**Behavior to change:**
- After stage 5, bridge speaks MuxConn wire format for both commands and events
- Commands sent via `#<id> ze-system:dispatch {"command":"..."}` format
- Events received as `#<id> ze-plugin-callback:deliver-batch {"events":[...]}` and parsed
- Flush support: `#<id> ze-bgp:peer-flush {"selector":"<addr>"}` after route commands

## Design Considerations

**bufio.Scanner buffering:** The 5-stage protocol uses a `bufio.Scanner` on os.Stdin. After stage 5, creating a new reader on os.Stdin risks losing data buffered by the scanner. The bridge code already handles this carefully (reuses the same scanner for events). The fix must either reuse the existing scanner or ensure no data is buffered at the transition point.

**os.Stdin is not net.Conn:** `rpc.NewConn` takes `net.Conn`. os.Stdin is `*os.File`. Options: (a) use `net.FileConn()`, (b) create a lightweight MuxConn-compatible wrapper that accepts `io.Reader`/`io.Writer`, (c) implement MuxConn wire format parsing inline without the library.

**Flush integration:** Once the bridge speaks MuxConn, flush becomes natural: `CallRPC("ze-bgp:peer-flush", ...)` returns when the barrier completes. No synthetic events needed. The response arrives through the same MuxConn channel.

## Data Flow (MANDATORY)

To be completed during design phase.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ExaBGP plugin sends route command | -> | bridge formats MuxConn, ze dispatches | TBD |
| Ze sends BGP event | -> | bridge parses MuxConn, translates, forwards to ExaBGP plugin | TBD |
| ExaBGP plugin sends route | -> | bridge injects flush via MuxConn CallRPC | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ExaBGP plugin writes `neighbor 10.0.0.1 announce route ...` | Bridge translates, sends via MuxConn, ze processes command |
| AC-2 | Ze sends BGP UPDATE event via MuxConn | Bridge receives, translates to ExaBGP JSON, forwards to plugin stdin |
| AC-3 | ExaBGP plugin sends route command | Bridge injects `peer <addr> flush` via MuxConn, blocks until response |
| AC-4 | Bridge startup 5-stage protocol | Works as before (raw text, no MuxConn) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | `internal/exabgp/bridge/bridge_test.go` | MuxConn command dispatch | |
| TBD | `internal/exabgp/bridge/bridge_test.go` | MuxConn event reception | |
| TBD | `internal/exabgp/bridge/bridge_test.go` | Flush injection after route commands | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TBD | `test/exabgp/` or existing exabgp-compat tests | End-to-end bridge runtime | |

## Files to Modify
- `internal/exabgp/bridge/bridge.go` -- switch to MuxConn after stage 5
- `internal/exabgp/bridge/bridge_command.go` -- format commands in MuxConn wire format
- `internal/exabgp/bridge/bridge_event.go` -- parse events from MuxConn wire format
- Possibly `pkg/plugin/rpc/conn.go` -- accept `io.ReadWriteCloser` not just `net.Conn`

## Files to Create
- TBD during design

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
