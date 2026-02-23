# Spec: Buffered TCP Session Reads

**Spec set:** I/O Syscall Reduction (1 of 2)
- **This spec:** Buffered TCP reads (peer sessions)
- **Companion:** `spec-batched-ipc-delivery.md` (plugin event delivery)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - session read loop
4. `internal/plugins/bgp/reactor/session_read.go` - current read path
5. `internal/plugins/bgp/reactor/session.go` - Session struct, buffer pools

## Task

Wrap the BGP session TCP connection with `bufio.Reader` to reduce read syscalls. Currently every BGP message requires 2 `io.ReadFull` calls directly on `net.Conn` (one for the 19-byte header, one for the body), each triggering a kernel syscall. A `bufio.Reader` batches kernel reads — a single 4KB+ read from the kernel serves multiple `io.ReadFull` calls from userspace.

**Motivation:** Flamegraph profiling shows `(*Session).readAndProcessMessage` → `io.ReadFull` → `net.(*conn).Read` → `rawsyscalln` as a significant CPU consumer. Under load (route reflector with millions of routes), 2 syscalls per message is the dominant read-path cost.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - session read loop, buffer pools
  → Constraint: Pool buffers must be returned or ownership transferred
  → Constraint: Read deadlines used for hold timer enforcement

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/session_read.go` - `readAndProcessMessage`: reads header (19B) and body via 2x `io.ReadFull(conn, ...)` directly on `net.Conn`
- [ ] `internal/plugins/bgp/reactor/session.go` - Session struct: `readBufPool4K`/`readBufPool64K` pools, `getReadBuffer()`/`returnReadBuffer()`, `extendedMessage` flag, `conn` field
- [ ] `internal/plugins/bgp/reactor/session.go` - `Run()` loop: calls `readAndProcessMessage` in a loop with read deadlines

**Behavior to preserve:**
- Pool-based read buffer lifecycle (get → use → return/keep)
- Read deadline management (`SetReadDeadline` per message)
- Extended message negotiation (4K vs 64K buffer selection)
- Buffer ownership transfer to callback when `kept=true`
- Error handling: EOF, connection reset, header validation
- `ReadAndProcess()` public API for tests

**Behavior to change:**
- Wrap `net.Conn` with `bufio.Reader` so `io.ReadFull` reads from userspace buffer instead of direct syscalls

## Data Flow (MANDATORY)

### Entry Point
- TCP bytes arrive from BGP peer on `net.Conn`
- `Session.Run()` loop calls `readAndProcessMessage(conn)`

### Transformation Path
1. **Current:** `io.ReadFull(conn, buf[:19])` → kernel syscall for 19 bytes
2. **Current:** header parse → validate length
3. **Current:** `io.ReadFull(conn, buf[19:length])` → kernel syscall for body
4. **Proposed:** `io.ReadFull(bufReader, buf[:19])` → reads from userspace buffer (may trigger one larger kernel read)
5. **Proposed:** `io.ReadFull(bufReader, buf[19:length])` → reads from userspace buffer (no syscall if data cached)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel ↔ Userspace | `net.Conn.Read` (currently per-field) → `bufio.Reader` (batched) | [ ] |
| Session ↔ Callback | Buffer ownership via `kept` flag — unchanged | [ ] |

### Integration Points
- `Session.Run()` creates connection — add `bufio.Reader` wrap here
- `readAndProcessMessage()` — change `conn net.Conn` param to `reader io.Reader`
- `ReadAndProcess()` (test API) — use same buffered reader
- `SetReadDeadline()` — must still be called on underlying `net.Conn`, not on `bufio.Reader`

### Architectural Verification
- [ ] No bypassed layers — reads still go through same code path
- [ ] No unintended coupling — `bufio.Reader` is a transparent wrapper
- [ ] No duplicated functionality — extends `io.ReadFull`, doesn't recreate
- [ ] Zero-copy preserved — pool buffers unchanged, `bufio.Reader` is read-side only

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session read loop with `bufio.Reader` | Messages parsed correctly (all types: UPDATE, OPEN, KEEPALIVE, NOTIFICATION, ROUTE-REFRESH) |
| AC-2 | Read deadline expiry | `SetReadDeadline` on underlying conn still triggers timeout in `io.ReadFull` through `bufio.Reader` |
| AC-3 | Extended message negotiation | Buffer reader created with standard size; no impact on pool buffer selection (4K vs 64K) |
| AC-4 | Connection close (EOF) | `bufio.Reader` propagates EOF correctly |
| AC-5 | Buffer ownership transfer | `kept=true` from callback still works — pool buffer lifecycle unchanged |
| AC-6 | Existing tests pass | `make ze-unit-test` and `make ze-functional-test` pass unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionReadWithBufio` | `internal/plugins/bgp/reactor/session_read_test.go` | Buffered reader parses messages correctly | |
| `TestSessionReadDeadlineWithBufio` | `internal/plugins/bgp/reactor/session_read_test.go` | Read deadline fires through bufio.Reader | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs. Buffer sizes use existing pool constants.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing BGP functional tests | `test/` | All existing sessions still work with buffered reads | |

### Future
- Benchmark: measure syscall reduction under load (requires profiling infrastructure)

## Files to Modify
- `internal/plugins/bgp/reactor/session.go` - Add `bufReader *bufio.Reader` field to Session, initialize in `Run()` or `setConn()`
- `internal/plugins/bgp/reactor/session_read.go` - Change `readAndProcessMessage` to read from `s.bufReader` instead of raw `conn`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| RPC count in arch docs | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Functional test for new RPC/API | No | |

## Files to Create
None — this is a modification to existing files only.

## Implementation Steps

1. **Add `bufReader` field to Session** — initialize `bufio.NewReaderSize(conn, 8192)` when connection is set
2. **Write unit tests** for buffered read path → Review: deadline propagation, EOF handling
3. **Run tests** → Verify FAIL
4. **Modify `readAndProcessMessage`** — use `s.bufReader` for `io.ReadFull`, keep `conn` for `SetReadDeadline`
5. **Run tests** → Verify PASS
6. **Run `make ze-verify`** → Full suite
7. **Critical Review** → Correctness, Simplicity, Consistency, Completeness, Quality, Tests

### Failure Routing

| Failure | Route To |
|---------|----------|
| Read deadline doesn't fire through bufio | Check: `SetReadDeadline` must be on underlying `net.Conn`, not through `bufio.Reader` |
| EOF not propagated | Check: `bufio.Reader` wraps same conn, should propagate |
| Buffer pool interaction | `bufio.Reader` is independent of pool buffers — they serve different purposes |

## Design Insights

- `bufio.Reader` buffer size doesn't need to match pool buffer size — `bufio` buffers kernel reads, pools buffer parsed messages
- `SetReadDeadline` must be called on the underlying `net.Conn` — `bufio.Reader` doesn't implement `net.Conn` deadline methods
- `bufio.Reader` must be reset or recreated on reconnection (new conn = new reader)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-buffered-tcp-read.md`
- [ ] **Spec included in commit**
