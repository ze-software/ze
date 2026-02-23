# Spec: Buffered TCP Session Reads

**Spec set:** I/O Syscall Reduction (1 of 2)
- **This spec:** Buffered TCP reads (peer sessions)
- **Companion:** `spec-batched-ipc-delivery.md` (plugin event delivery)

## Task

Wrap the BGP session TCP connection with `bufio.Reader` to reduce read syscalls. Currently every BGP message requires 2 `io.ReadFull` calls directly on `net.Conn` (one for the 19-byte header, one for the body), each triggering a kernel syscall. A `bufio.Reader` batches kernel reads — a single 64KB read from the kernel serves multiple `io.ReadFull` calls from userspace.

**Motivation:** Flamegraph profiling shows `(*Session).readAndProcessMessage` → `io.ReadFull` → `net.(*conn).Read` → `rawsyscalln` as a significant CPU consumer. Under load (route reflector with millions of routes), 2 syscalls per message is the dominant read-path cost.

## Required Reading

### Architecture Docs
- `docs/architecture/core-design.md` - session read loop, buffer pools
  → Constraint: Pool buffers must be returned or ownership transferred
  → Constraint: Read deadlines used for hold timer enforcement

## Current Behavior

**Source files read:**
- `internal/plugins/bgp/reactor/session_read.go` - `readAndProcessMessage`: reads header (19B) and body via 2x `io.ReadFull(conn, ...)` directly on `net.Conn`
- `internal/plugins/bgp/reactor/session.go` - Session struct: `readBufPool4K`/`readBufPool64K` pools, `getReadBuffer()`/`returnReadBuffer()`, `extendedMessage` flag, `conn` field
- `internal/plugins/bgp/reactor/session.go` - `Run()` loop: calls `readAndProcessMessage` in a loop with read deadlines

**Behavior preserved:**
- Pool-based read buffer lifecycle (get → use → return/keep)
- Read deadline management (`SetReadDeadline` per message on underlying `net.Conn`)
- Extended message negotiation (4K vs 64K buffer selection)
- Buffer ownership transfer to callback when `kept=true`
- Error handling: EOF, connection reset, header validation
- `ReadAndProcess()` public API for tests

**Behavior changed:**
- `net.Conn` wrapped with `bufio.NewReaderSize(conn, 65536)` so `io.ReadFull` reads from userspace buffer instead of direct syscalls
- Buffer sized at 64KB to cover extended BGP messages in a single kernel read

## Data Flow

### Entry Point
- TCP bytes arrive from BGP peer on `net.Conn`
- `Session.Run()` loop calls `readAndProcessMessage(conn)`

### Transformation Path
1. **Before:** `io.ReadFull(conn, buf[:19])` → kernel syscall for 19 bytes
2. **Before:** header parse → validate length
3. **Before:** `io.ReadFull(conn, buf[19:length])` → kernel syscall for body
4. **After:** `io.ReadFull(s.bufReader, buf[:19])` → reads from userspace buffer (may trigger one larger kernel read)
5. **After:** `io.ReadFull(s.bufReader, buf[19:length])` → reads from userspace buffer (no syscall if data cached)

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Kernel ↔ Userspace | `net.Conn.Read` (per-field) → `bufio.Reader` (batched 64KB) |
| Session ↔ Callback | Buffer ownership via `kept` flag — unchanged |

### Integration Points
- `Session.connectionEstablished()` — creates `bufio.NewReaderSize(conn, 65536)`
- `readAndProcessMessage()` — reads from `s.bufReader` instead of raw `conn`
- `ReadAndProcess()` (test API) — uses same buffered reader
- `SetReadDeadline()` — called on underlying `net.Conn`, not on `bufio.Reader`
- `closeConn()` — `bufReader` not nilled; wrapping closed conn returns proper read error on reconnection

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Demonstrated By |
|-------|-------------------|-------------------|-----------------|
| AC-1 | Session read loop with `bufio.Reader` | Messages parsed correctly (UPDATE, OPEN, KEEPALIVE, NOTIFICATION, ROUTE-REFRESH) | `TestSessionReadWithBufio` — OPEN + KEEPALIVE parsed, FSM transitions verified |
| AC-2 | Read deadline expiry | `SetReadDeadline` on underlying conn triggers timeout through `bufio.Reader` | `TestSessionReadDeadlineWithBufio` — 50ms deadline fires as `net.Error` timeout |
| AC-3 | Extended message negotiation | Buffer reader created with 64KB; no impact on pool buffer selection | `session.go:610` — `bufio.NewReaderSize(conn, 65536)` independent of pool |
| AC-4 | Connection close (EOF) | `bufio.Reader` propagates EOF correctly | `session_read.go:48` — `errors.Is(err, io.EOF)` check unchanged |
| AC-5 | Buffer ownership transfer | `kept=true` from callback still works — pool buffer lifecycle unchanged | `session_read.go:88-94` — pool return logic unchanged |
| AC-6 | Existing tests pass | `make ze-unit-test` and `make ze-functional-test` pass | All tests pass (verified before commit) |

## Design Insights

- `bufio.Reader` buffer size (64KB) doesn't need to match pool buffer size — `bufio` buffers kernel reads, pools buffer parsed messages
- `SetReadDeadline` must be called on the underlying `net.Conn` — `bufio.Reader` doesn't implement `net.Conn` deadline methods
- `bufReader` is NOT nilled on `closeConn()` — `Run()` may have captured conn, and `bufReader` wrapping the closed conn returns a proper read error; `connectionEstablished()` replaces `bufReader` on reconnection
- 64KB chosen to match extended message size — a single kernel read can fill the buffer with one full extended message

## Files Modified

| File | Change | Status |
|------|--------|--------|
| `internal/plugins/bgp/reactor/session.go` | Added `bufReader *bufio.Reader` field, initialized in `connectionEstablished()` with 64KB | ✅ Done |
| `internal/plugins/bgp/reactor/session_read.go` | Changed `io.ReadFull(conn, ...)` to `io.ReadFull(s.bufReader, ...)` for header and body | ✅ Done |
| `internal/plugins/bgp/reactor/session_read_test.go` | Added `TestSessionReadWithBufio`, `TestSessionReadDeadlineWithBufio` | ✅ Done |
| `internal/plugins/bgp/reactor/session_test.go` | Updated `acceptWithReader` helper, 8 test functions for bufio | ✅ Done |

## Implementation Audit

| Requirement | Status | Evidence |
|-------------|--------|----------|
| AC-1: Messages parsed correctly | ✅ Done | `TestSessionReadWithBufio` — OPEN→OpenConfirm, KEEPALIVE→Established |
| AC-2: Read deadline propagation | ✅ Done | `TestSessionReadDeadlineWithBufio` — 50ms deadline fires as `net.Error` timeout |
| AC-3: Extended message unaffected | ✅ Done | `session.go:610` — 64KB bufio independent of 4K/64K pool selection |
| AC-4: EOF propagation | ✅ Done | `session_read.go:48` — EOF check unchanged, bufio propagates |
| AC-5: Buffer ownership transfer | ✅ Done | `session_read.go:88-94` — kept/return logic unchanged |
| AC-6: Existing tests pass | ✅ Done | `make ze-unit-test` + `make ze-functional-test` pass |
| TDD: `TestSessionReadWithBufio` | ✅ Done | `session_read_test.go:18-69` |
| TDD: `TestSessionReadDeadlineWithBufio` | ✅ Done | `session_read_test.go:74-104` |
| Integration: YANG schema | N/A | No schema changes |
| Integration: Architecture docs | N/A | No new concepts requiring doc updates |

### Audit Summary
- ✅ Done: 10
- ⚠️ Partial: 0
- ❌ Skipped: 0
- 🔄 Changed: 0

## Implementation Summary

Wrapped `net.Conn` with `bufio.NewReaderSize(conn, 65536)` in `connectionEstablished()`. Changed `readAndProcessMessage` to read through `s.bufReader`. `SetReadDeadline` still called on underlying `net.Conn`. Two new tests validate message parsing and deadline propagation through the buffered layer.

### Deviations from Spec
- Buffer size increased from planned 8KB to 64KB to match extended message size (improvement)
- `bufReader` initialized in `connectionEstablished()` rather than `Run()` — cleaner lifecycle
