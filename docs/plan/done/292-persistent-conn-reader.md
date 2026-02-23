# Spec: persistent-conn-reader

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - IPC protocol (5-stage startup, RPC semantics)
4. `pkg/plugin/rpc/conn.go` - current Conn implementation (5 goroutine sites)
5. `pkg/plugin/rpc/mux.go` - MuxConn persistent reader pattern (reference implementation)

## Task

Eliminate all 5 per-RPC goroutines in `pkg/plugin/rpc/conn.go` by:
1. **Persistent reader goroutine** — one long-lived goroutine per Conn (replaces 3 read-side goroutines)
2. **Deadline-based writes** — `SetWriteDeadline` on the underlying `net.Conn` (replaces 2 write-side goroutines)

Currently, every call to `ReadRequest`, `CallRPC`, `CallBatchRPC`, and `WriteWithContext` spawns 1-2 goroutines that race a blocking I/O operation against context cancellation. This violates `rules/goroutine-lifecycle.md` (no per-event goroutines in hot paths) and creates goroutine leak risk documented in the Conn type comment.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/ipc_protocol.md` - IPC protocol, 5-stage startup, RPC semantics
  → Decision: connections are bidirectional over Unix socket pairs; NUL-framed JSON
  → Constraint: must preserve request-response correlation (ID matching)
- [x] `.claude/rules/goroutine-lifecycle.md` - long-lived workers only
  → Constraint: no per-event goroutines in hot paths; channel + worker pattern required

**Key insights:**
- MuxConn (`pkg/plugin/rpc/mux.go`) already demonstrates the persistent reader pattern for Socket A post-startup
- Each Conn instance uses only ONE read pattern at runtime: Socket A → `ReadRequest` loop, Socket B → `CallRPC`/`CallBatchRPC` (serialized by `callMu`). No mixing on same connection
- `net.Conn` (Unix sockets, `net.Pipe()`) supports `SetWriteDeadline`
- All delivery-path contexts have 5s timeouts; startup contexts use server-scoped cancellation

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `pkg/plugin/rpc/conn.go` (319L) - Conn struct with 5 goroutine spawn sites
- [x] `pkg/plugin/rpc/mux.go` (164L) - MuxConn wrapping Conn with persistent reader for concurrent CallRPC
- [x] `internal/ipc/framing.go` (110L) - FrameReader (bufio.Scanner) and FrameWriter (io.Writer)
- [x] `pkg/plugin/rpc/mux_test.go` - MuxConn test coverage (sequential, concurrent, cancel, close, error, unexpected ID)

**Current goroutine spawn sites (5 total):**

| Site | File:Line | Method | Direction | Hot Path? |
|------|-----------|--------|-----------|-----------|
| G1 | `conn.go:92` | `ReadRequest` | Read | Yes (dispatch loop) |
| G2 | `conn.go:183` | `CallRPC` | Read | Yes (event delivery) |
| G3 | `conn.go:221` | `CallBatchRPC` | Write | Yes (batch delivery) |
| G4 | `conn.go:240` | `CallBatchRPC` | Read | Yes (batch delivery) |
| G5 | `conn.go:278` | `WriteWithContext` | Write | Yes (all writes) |

**Behavior to preserve:**
- `callMu` serializes `CallRPC`/`CallBatchRPC` — exactly one outstanding request per Conn at a time
- `mu` protects concurrent writes
- Request/response ID matching in `CallRPC` and `CallBatchRPC`
- `Close()` unblocks all pending operations
- Context cancellation returns `ctx.Err()` promptly
- `MuxConn` wraps `Conn` and owns the reader — after MuxConn creation, `Conn.ReadRequest` must NOT be called
- All existing tests pass unchanged (~~MuxConn bypasses Conn's reader~~ MuxConn now shares Conn's persistent reader via readFrame — discovered during integration that direct reader access races with the persistent reader)

**Behavior to change:**
- Replace per-call goroutines with persistent reader + deadline writes
- Store `writeConn net.Conn` in Conn struct (currently only `readConn` is stored)
- `WriteWithContext` uses `SetWriteDeadline` instead of goroutine bridge
- `ReadRequest`, `CallRPC`, `CallBatchRPC` read from persistent reader channel instead of spawning goroutines

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- RPC frames enter via `net.Conn` (Unix socket pair or `net.Pipe`) → `ipc.FrameReader.Read()` (blocking `bufio.Scanner.Scan()`)
- RPC frames exit via `ipc.FrameWriter.Write()` → `net.Conn.Write()` (blocking)

### Transformation Path

**Read side (current → proposed):**
1. Current: `ReadRequest(ctx)` → `go func() { reader.Read() }` → `select { ctx | ch }`
2. Proposed: `startReader()` [once] → background goroutine: `for { reader.Read(); frameCh <- frame }` → `ReadRequest(ctx)` → `select { ctx | frameCh | readerDone }`

**Write side (current → proposed):**
1. Current: `WriteWithContext(ctx, v)` → `go func() { WriteFrame(v) }` → `select { ctx | ch }`
2. Proposed: `writeWithDeadline(ctx, v)` → `writeConn.SetWriteDeadline(deadline)` → `WriteFrame(v)` → `writeConn.SetWriteDeadline(zero)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Conn → FrameReader | Persistent goroutine reads, pushes to frameCh | [x] |
| Conn → FrameWriter | Deadline-based write, no goroutine | [x] |
| Conn → MuxConn | MuxConn shares Conn's persistent reader via readFrame | [x] |

### Integration Points
- ~~`MuxConn.readLoop()` accesses `m.conn.reader.Read()` directly — does NOT use Conn's ReadRequest. MuxConn is **unaffected** by this change.~~ MuxConn was updated to use `conn.readFrame(context.Background())`, sharing the persistent reader. Direct `reader.Read()` access raced with the persistent reader goroutine when SDK called `ReadRequest` during handshake then wrapped in MuxConn.
- `callMu` continues to serialize CallRPC/CallBatchRPC — the persistent reader reads exactly one frame per call, matching the serial protocol.
- `Close()` closes readConn → unblocks `reader.Read()` → persistent reader exits → `readerDone` channel fires.

### Architectural Verification
- [x] No bypassed layers (persistent reader is internal to Conn, callers unchanged)
- [x] No unintended coupling (MuxConn shares persistent reader via readFrame, same goroutine)
- [x] No duplicated functionality (Conn's persistent reader is simpler than MuxConn's — no ID routing)
- [x] Zero-copy preserved (FrameReader already copies internally; no additional copies)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ReadRequest(ctx)` called on Conn | Returns next frame from persistent reader; no goroutine spawned per call |
| AC-2 | `CallRPC(ctx, method, params)` called | Writes via deadline, reads from persistent reader; no goroutines spawned |
| AC-3 | `CallBatchRPC(ctx, events)` called | Writes via deadline, reads from persistent reader; no goroutines spawned |
| AC-4 | `WriteWithContext(ctx, v)` called | Uses `SetWriteDeadline` on writeConn; no goroutine spawned |
| AC-5 | Context cancelled during `ReadRequest` | Returns `ctx.Err()` promptly; persistent reader continues for future calls |
| AC-6 | Context cancelled during write | `SetWriteDeadline` causes write to fail with timeout error; returns `ctx.Err()` |
| AC-7 | `Close()` called while `ReadRequest` is blocked | `ReadRequest` returns error; persistent reader exits cleanly |
| AC-8 | `Close()` called while `CallRPC` is waiting for response | `CallRPC` returns error; persistent reader exits |
| AC-9 | `MuxConn` wraps Conn | MuxConn still works correctly — bypasses Conn's persistent reader |
| AC-10 | Reader encounters I/O error (broken pipe) | Error stored; all subsequent reads return stored error |
| AC-11 | Multiple sequential `ReadRequest` calls | Each gets the next frame in order; persistent reader stays alive |
| AC-12 | Context without deadline passed to `WriteWithContext` | Uses default 30s safety deadline; write completes normally |
| AC-13 | `CallRPC` with existing `callMu` serialization | Concurrent callers still serialize correctly; no races |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConn_ReadRequest_PersistentReader` | `pkg/plugin/rpc/conn_test.go` | AC-1: ReadRequest uses persistent reader, no per-call goroutine | |
| `TestConn_ReadRequest_ContextCancel` | `pkg/plugin/rpc/conn_test.go` | AC-5: context cancel returns promptly, reader survives | |
| `TestConn_ReadRequest_Sequential` | `pkg/plugin/rpc/conn_test.go` | AC-11: multiple sequential reads get frames in order | |
| `TestConn_ReadRequest_CloseUnblocks` | `pkg/plugin/rpc/conn_test.go` | AC-7: Close() unblocks pending ReadRequest | |
| `TestConn_CallRPC_DeadlineWrite` | `pkg/plugin/rpc/conn_test.go` | AC-2: CallRPC uses deadline write + persistent read | |
| `TestConn_CallBatchRPC_DeadlineWrite` | `pkg/plugin/rpc/conn_test.go` | AC-3: CallBatchRPC uses deadline write + persistent read | |
| `TestConn_WriteWithContext_Deadline` | `pkg/plugin/rpc/conn_test.go` | AC-4, AC-12: uses SetWriteDeadline with ctx deadline or default | |
| `TestConn_WriteWithContext_ContextCancel` | `pkg/plugin/rpc/conn_test.go` | AC-6: cancelled context → deadline-triggered write error | |
| `TestConn_ReaderError_Propagates` | `pkg/plugin/rpc/conn_test.go` | AC-10: broken pipe → error stored, subsequent reads fail | |
| `TestConn_CallRPC_Serialization` | `pkg/plugin/rpc/conn_test.go` | AC-13: concurrent CallRPC callers serialize via callMu | |
| `TestConn_MuxConn_Compatibility` | `pkg/plugin/rpc/conn_test.go` | AC-9: MuxConn still works after Conn changes | |
| `TestConn_NoGoroutineLeak` | `pkg/plugin/rpc/conn_test.go` | Goroutine count stable across many ReadRequest/CallRPC calls | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Default write deadline | 30s | N/A | N/A | N/A |
| frameCh capacity | 1 | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `make ze-functional-test` | `test/plugin/*.ci` | Plugin startup + event delivery still works | |

No new functional tests needed — this is an internal optimization. Existing functional tests exercise the full plugin startup + event delivery path and will verify correctness.

### Future (if deferring any tests)
- Benchmark comparing goroutine-per-call vs persistent reader throughput (deferred — optimization evidence, not correctness)

## Files to Modify
- `pkg/plugin/rpc/conn.go` - add persistent reader, deadline writes, store writeConn

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | Existing tests cover |

## Files to Create
- `pkg/plugin/rpc/conn_test.go` - unit tests for Conn (currently no dedicated test file)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Persistent Reader

1. **Write unit tests** for persistent reader behavior (`TestConn_ReadRequest_PersistentReader`, `TestConn_ReadRequest_Sequential`, `TestConn_ReadRequest_ContextCancel`, `TestConn_ReadRequest_CloseUnblocks`, `TestConn_ReaderError_Propagates`, `TestConn_NoGoroutineLeak`) → Review: edge cases? Boundary tests?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement persistent reader** — add to Conn struct:
   - `frameResult` type: `{ data []byte, err error }`
   - `frameCh chan frameResult` (capacity 1)
   - `readerDone chan struct{}`
   - `readerOnce sync.Once`
   - `startReader()` method: goroutine reads frames in loop, sends to frameCh, closes readerDone on exit
   - Modify `ReadRequest()`: call `startReader()`, select on frameCh/ctx.Done()/readerDone
   - Modify `CallRPC()`: call `startReader()`, replace read goroutine with frameCh select
   - Modify `CallBatchRPC()`: same as CallRPC read side
   - Update `Close()` to wait for reader exit (non-blocking — closing readConn already unblocks it)
4. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
5. **Run existing MuxConn tests** → Verify PASS. MuxConn unaffected.

### Phase 2: Deadline-Based Writes

6. **Write unit tests** for deadline writes (`TestConn_WriteWithContext_Deadline`, `TestConn_WriteWithContext_ContextCancel`, `TestConn_CallRPC_DeadlineWrite`, `TestConn_CallBatchRPC_DeadlineWrite`)
7. **Run tests** → Verify FAIL.
8. **Implement deadline writes:**
   - Add `writeConn net.Conn` field to Conn struct
   - Store writeConn in `NewConn()`
   - Replace `WriteWithContext()`: extract deadline from ctx (or use 30s default) → `writeConn.SetWriteDeadline(deadline)` → `WriteFrame(v)` → `writeConn.SetWriteDeadline(time.Time{})` (clear)
   - Replace `CallBatchRPC()` write goroutine: same deadline pattern for `writeBatchFrame()`
9. **Run tests** → Verify PASS.

### Phase 3: Integration + Serialization

10. **Write `TestConn_CallRPC_Serialization`** — verify callMu still works
11. **Write `TestConn_MuxConn_Compatibility`** — verify MuxConn wrapping Conn still works
12. **Run all tests** → Verify PASS.
13. **Verify all** → `make ze-lint && make ze-unit-test && make ze-functional-test`
14. **Critical Review** → All 6 checks from `rules/quality.md` must pass.
15. **Complete spec** → Fill audit tables, move spec to `done/`.

### Design Decisions

**Why frameCh capacity 1 (not 0 or larger)?**
Capacity 1 lets the reader goroutine push one frame without blocking, then attempt the next read. This provides natural back-pressure: if the consumer hasn't read the frame yet, the reader blocks on `frameCh <-` (not on I/O), which is fine since each Conn has at most one consumer. Capacity 0 would force lock-step (reader blocks until consumer ready), losing the pipeline benefit. Larger capacity is unnecessary since reads are serial.

**Why send error through frameCh (not atomic pointer)?**
MuxConn uses an atomic error pointer + separate done channel because it has multiple concurrent callers who need to observe the terminal error independently. Conn has serial access (one ReadRequest OR one CallRPC at a time), so sending the error as a `frameResult{nil, err}` through the same channel is simpler — the consumer gets the error in order, no race with pending frames.

**Why 30s default write deadline?**
The delivery path always has a 5s context timeout. Startup RPCs use server-scoped contexts without deadlines. A 30s safety net prevents writes from blocking indefinitely if the peer hangs without closing the socket, while being generous enough to never trigger during normal operation.

**Why not change MuxConn?**
MuxConn already has its own persistent reader (`readLoop`) that routes by ID. It accesses `conn.reader` directly, bypassing `ReadRequest`. Adding a persistent reader to Conn doesn't conflict — MuxConn simply never calls `startReader()`. This avoids dual-reader conflicts and keeps MuxConn's proven concurrent dispatch logic untouched.

**Why `startReader()` with `sync.Once` instead of starting in `NewConn`?**
MuxConn creates Conn then immediately takes over the reader. If Conn started a reader in `NewConn`, MuxConn would race with it. Lazy start via `sync.Once` means the reader only activates when `ReadRequest` or `CallRPC` is first called — never for MuxConn-wrapped connections.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3/8 (fix syntax/types) |
| Test fails wrong reason | Step 1/6 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| MuxConn test fails | Verify MuxConn doesn't call startReader(); check reader ownership |
| Goroutine leak detected | Check frameCh drain on Close(); check readerDone signaling |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| MuxConn can access `conn.reader` directly without conflict | MuxConn races with persistent reader if SDK calls ReadRequest during handshake then wraps in MuxConn | DATA RACE in SDK tests (22 failures) | Changed MuxConn to use `conn.readFrame()` |
| `SetWriteDeadline` timeout returns `context.DeadlineExceeded` | Returns `net.Error` i/o timeout, not context error | `TestRPCDeliverBatchTimeout` and `TestSlowPluginFatal` failures | Added timeout→context error translation |
| `string(rune(i+1))` produces valid JSON | For i=0, produces `\x01` (control character, invalid JSON) | Test unmarshal failures | Used `fmt.Sprintf("%d", i+1)` |
| `frameCh` with bare `default:` drains safely | `block-silent-ignore.sh` hook rejects bare `default:` | Hook exit 2 | Redesigned with `atomic.Pointer[error]` pattern from MuxConn |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| `frameResult{data, err}` through single channel | Hook blocks bare `default:` case needed for channel drain | Separate `frameCh` (data only) + `atomic.Pointer[error]` + `readerDone` channel |
| MuxConn accessing `conn.reader` directly | DATA RACE with persistent reader goroutine | MuxConn calls `conn.readFrame(context.Background())` |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Assuming net.Error matches context errors | First time | N/A — niche issue | None |

## Design Insights

MuxConn sharing the persistent reader via `readFrame()` is simpler than the original plan of keeping two independent readers. The `sync.Once` ensures exactly one reader goroutine regardless of whether Conn is used directly or wrapped in MuxConn. This eliminates a whole class of ownership bugs.

## Implementation Summary

### What Was Implemented
- Persistent reader goroutine in Conn: `startReader()`, `readLoop()`, `readFrame()` with `frameCh`, `readerDone`, `readerErr`
- Deadline-based writes: `writeConn` field, `writeDeadline()` helper, `writeBatchWithDeadline()`
- `WriteWithContext` rewritten to use `SetWriteDeadline` with timeout→context error translation
- MuxConn updated to use `conn.readFrame()` instead of direct `conn.reader.Read()`
- 13 new unit tests in `conn_test.go`
- `// Related:` cross-references between `conn.go` and `mux.go`

### Bugs Found/Fixed
- DATA RACE between Conn persistent reader and MuxConn readLoop — fixed by having MuxConn use `readFrame()`
- Timeout error semantics: `SetWriteDeadline` returns `i/o timeout` but callers expect `context.DeadlineExceeded` — fixed with translation

### Documentation Updates
- None needed — internal optimization, no protocol or API changes

### Deviations from Plan
- Spec said "MuxConn bypasses Conn's reader (unaffected)" — MuxConn was updated to share the persistent reader via `readFrame()` due to DATA RACE
- Spec proposed `frameResult{data, err}` type — replaced with `atomic.Pointer[error]` pattern (hook rejected bare `default:` case)
- `mux.go` was modified (not in original Files to Modify) — necessary to fix the reader ownership race
- Phases 1-3 were implemented together rather than sequentially — all tests written first, then all implementation

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Persistent reader goroutine (replaces 3 read-side goroutines) | ✅ Done | `conn.go:78-129` | `startReader`, `readLoop`, `readFrame` |
| Deadline-based writes (replaces 2 write-side goroutines) | ✅ Done | `conn.go:287-338` | `writeBatchWithDeadline`, `WriteWithContext`, `writeDeadline` |
| Store `writeConn` in Conn struct | ✅ Done | `conn.go:47` | Used by `SetWriteDeadline` calls |
| `callMu` serialization preserved | ✅ Done | `conn.go:207,256` | Unchanged |
| `Close()` unblocks persistent reader | ✅ Done | `conn.go:74-76` | Closes `readConn` → reader exits |
| MuxConn compatibility | ✅ Done | `mux.go:130` | Changed to use `readFrame` (deviation) |
| All existing tests pass unchanged | ✅ Done | `make ze-verify` exit 0 | All pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestConn_ReadRequest_PersistentReader` | 10 sequential reads, no per-call goroutines |
| AC-2 | ✅ Done | `TestConn_CallRPC_DeadlineWrite` | Deadline write + persistent read |
| AC-3 | ✅ Done | `TestConn_CallBatchRPC_DeadlineWrite` | Deadline write + persistent read |
| AC-4 | ✅ Done | `TestConn_WriteWithContext_Deadline` | `SetWriteDeadline` on writeConn |
| AC-5 | ✅ Done | `TestConn_ReadRequest_ContextCancel` | Cancel returns promptly, reader survives |
| AC-6 | ✅ Done | `TestConn_WriteWithContext_ContextCancel` | Pre-canceled context returns error |
| AC-7 | ✅ Done | `TestConn_ReadRequest_CloseUnblocks` | Close unblocks pending ReadRequest |
| AC-8 | ✅ Done | `TestConn_CallRPC_CloseUnblocks` | Close unblocks pending CallRPC |
| AC-9 | ✅ Done | `TestConn_MuxConn_Compatibility` | MuxConn wrapping Conn works via readFrame |
| AC-10 | ✅ Done | `TestConn_ReaderError_Propagates` | Error stored, subsequent reads fail |
| AC-11 | ✅ Done | `TestConn_ReadRequest_Sequential` | 4 frames in order |
| AC-12 | ✅ Done | `TestConn_WriteWithContext_Deadline` | Default 30s deadline when no ctx deadline |
| AC-13 | ✅ Done | `TestConn_CallRPC_Serialization` | 10 concurrent callers serialize correctly |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestConn_ReadRequest_PersistentReader` | ✅ Done | `conn_test.go:42` | AC-1 |
| `TestConn_ReadRequest_ContextCancel` | ✅ Done | `conn_test.go:122` | AC-5 |
| `TestConn_ReadRequest_Sequential` | ✅ Done | `conn_test.go:85` | AC-11 |
| `TestConn_ReadRequest_CloseUnblocks` | ✅ Done | `conn_test.go:158` | AC-7 |
| `TestConn_CallRPC_DeadlineWrite` | ✅ Done | `conn_test.go:283` | AC-2 |
| `TestConn_CallBatchRPC_DeadlineWrite` | ✅ Done | `conn_test.go:318` | AC-3 |
| `TestConn_WriteWithContext_Deadline` | ✅ Done | `conn_test.go:366` | AC-4, AC-12 |
| `TestConn_WriteWithContext_ContextCancel` | ✅ Done | `conn_test.go:412` | AC-6 |
| `TestConn_ReaderError_Propagates` | ✅ Done | `conn_test.go:193` | AC-10 |
| `TestConn_CallRPC_Serialization` | ✅ Done | `conn_test.go:474` | AC-13 |
| `TestConn_MuxConn_Compatibility` | ✅ Done | `conn_test.go:534` | AC-9 |
| `TestConn_NoGoroutineLeak` | ✅ Done | `conn_test.go:228` | AC-1, AC-11 |
| `TestConn_CallRPC_CloseUnblocks` | ✅ Done | `conn_test.go:434` | AC-8 (added beyond plan) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/rpc/conn.go` (modify) | ✅ Done | Persistent reader + deadline writes |
| `pkg/plugin/rpc/conn_test.go` (create) | ✅ Done | 13 new tests |
| `pkg/plugin/rpc/mux.go` (modify) | 🔄 Changed | Not in plan — updated to use `readFrame()` to fix DATA RACE |

### Audit Summary
- **Total items:** 33
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (MuxConn updated — documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-13 all demonstrated
- [x] `make ze-unit-test` passes
- [x] `make ze-functional-test` passes
- [x] Feature code integrated (`pkg/plugin/rpc/conn.go`)
- [x] Integration completeness proven end-to-end
- [x] Architecture docs updated (none needed — internal optimization)
- [x] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [x] `make ze-lint` passes
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?)
- [x] No speculative features (needed NOW?)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written → FAIL → implement → PASS
- [x] Tests FAIL before implementation (DATA RACE in context cancel test — exactly the bug)
- [x] Tests PASS after implementation (22/22 pass with race detector)
- [x] Functional tests for end-to-end behavior (`make ze-functional-test` passes)

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [x] Partial/Skipped items have user approval (none — 0 partial/skipped)
- [x] Implementation Summary filled
- [x] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
