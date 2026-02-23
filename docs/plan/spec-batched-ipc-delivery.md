# Spec: Batched IPC Event Delivery

**Spec set:** I/O Syscall Reduction (2 of 2)
- **Companion:** `spec-buffered-tcp-read.md` (peer TCP session reads)
- **This spec:** Batched plugin event delivery with offset-based zero-copy

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - current IPC framing
4. `internal/ipc/framing.go` - FrameReader/FrameWriter
5. `pkg/plugin/rpc/conn.go` - Conn, CallRPC, WriteFrame
6. `internal/plugin/process.go` - deliveryLoop, Deliver()
7. `internal/plugin/rpc_plugin.go` - SendDeliverEvent

## Task

Replace per-event IPC writes with a batch frame format. Currently each event delivered to a plugin triggers: `json.Marshal` → `make([]byte, len+1)` → `conn.Write()` (1 syscall) + blocking response read (1 syscall) + 2 short-lived goroutines (for context cancellation bridging). Under load, a route reflector with 3 plugins generates 6+ syscalls and 6 goroutine create/destroy cycles per UPDATE.

The batch format writes N events as one frame with an offset table, enabling:
- 1 write syscall per batch (instead of N)
- 1 ack per batch (instead of N)
- Zero-copy slicing on the reader (offsets into shared buffer)
- Elimination of per-event goroutine churn

**Motivation:** Flamegraph profiling shows `deliveryLoop` → `SendDeliverEvent` → `CallRPC` → `WriteWithContext` → `rawsyscalln` as significant CPU. The left side of the flamegraph shows `goexit0`/`schedule` runtime overhead from per-event goroutine creation in `WriteWithContext` and `CallRPC`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - NUL-framed JSON protocol, event format
  → Constraint: Events are JSON objects terminated by NUL byte
  → Decision: No backwards compatibility needed (Ze pre-release)
- [ ] `docs/architecture/core-design.md` - engine↔plugin event delivery
  → Constraint: Events pre-formatted once per format mode, reused across plugins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/ipc/framing.go` - `FrameWriter.Write()`: allocates `make([]byte, len(msg)+1)` per frame, appends NUL, calls `conn.Write()`. `FrameReader`: uses `bufio.Scanner` (already buffered reads)
- [ ] `pkg/plugin/rpc/conn.go` - `CallRPC()`: serialized by `callMu`, marshals JSON, calls `WriteWithContext` (spawns goroutine), reads response (spawns goroutine). `WriteWithContext()`: spawns `go func()` per write for context cancellation
- [ ] `internal/plugin/process.go` - `deliveryLoop()`: reads from `eventChan` (cap 64), calls `connB.SendDeliverEvent()` per event with 5s timeout. One long-lived goroutine per process
- [ ] `internal/plugin/rpc_plugin.go` - `SendDeliverEvent()`: wraps event in `DeliverEventInput`, calls `CallRPC("ze-plugin-callback:deliver-event", ...)`, checks response
- [ ] `pkg/plugin/sdk/sdk.go` - Plugin SDK event loop: receives events via RPC, dispatches to handler

**Behavior to preserve:**
- Event JSON format (unchanged — batch wraps existing events)
- Per-process delivery goroutine (long-lived, channel-based)
- Event pre-formatting (one marshal per format mode, reused)
- Delivery timeout (5s default, configurable via env)
- Error handling on delivery failure
- Cache consumer ack semantics
- Plugin SDK event dispatch to handler callbacks

**Behavior to change:**
- `deliveryLoop`: drain channel, batch events, single write+ack instead of per-event RPC
- `FrameWriter`: new batch write method using pooled buffer with offset table
- `FrameReader`: new batch read method returning slices into shared buffer
- `CallRPC` path: bypass per-event goroutines for batch delivery
- New RPC method: `ze-plugin-callback:deliver-batch` accepting multiple events

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received by reactor → formatted as JSON event → enqueued to `Process.eventChan`

### Transformation Path (Current — per event)
1. `deliveryLoop` reads one event from `eventChan`
2. `SendDeliverEvent(ctx, eventJSON)` wraps in RPC request
3. `CallRPC` → `json.Marshal(request)` → `WriteWithContext` → `go func(){ WriteFrame() }` → `FrameWriter.Write()` → `make([]byte, N+1)` → `conn.Write()` (syscall)
4. `CallRPC` → `go func(){ reader.Read() }` → blocks on response (syscall)
5. Response received → goroutines exit → next event

### Transformation Path (Proposed — batched)
1. `deliveryLoop` drains all available events from `eventChan` (non-blocking after first)
2. Writes batch header (total length, count, offsets) + concatenated event payloads into pooled buffer
3. Single `conn.Write(buf[:totalLen])` (1 syscall)
4. Single response read (1 syscall) — batch ack
5. Return buffer to pool → next batch

### Batch Frame Format

| Field | Size | Description |
|-------|------|-------------|
| total_len | 4 bytes (uint32 BE) | Total frame size including header |
| count | 2 bytes (uint16 BE) | Number of events in batch |
| offsets | count × 4 bytes (uint32 BE each) | Byte offset of each event within payload section |
| payload | variable | Concatenated event JSON strings |
| terminator | 1 byte | NUL byte (0x00) — frame delimiter |

Offsets are relative to the start of the payload section. End of event `i` = `offsets[i+1]` (or `payloadLen` for last event). Reader extracts event `i` as `payload[offsets[i]:offsets[i+1]]` — zero allocation, pure slice.

**Max batch size:** bounded by pool buffer (4KB initially, could use 64KB IPC pool). Events that exceed remaining space trigger a flush.

**Single-event fast path:** count=1 is a valid batch — no special case needed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin (write) | Batch frame over Unix socket | [ ] |
| Plugin → Engine (ack) | Single batch ack response | [ ] |
| Userspace → Kernel | 1 write syscall per batch vs N | [ ] |

### Integration Points
- `internal/ipc/framing.go` — new `BatchFrameWriter` and `BatchFrameReader`
- `pkg/plugin/rpc/conn.go` — new `WriteBatch` / `ReadBatch` methods (bypasses per-call goroutines)
- `internal/plugin/rpc_plugin.go` — new `SendDeliverBatch` method
- `internal/plugin/process.go` — `deliveryLoop` drain-and-flush logic
- `pkg/plugin/sdk/sdk.go` — SDK handles `deliver-batch` RPC, dispatches each event
- `pkg/plugin/rpc/types.go` — new `DeliverBatchInput` type

### Architectural Verification
- [ ] No bypassed layers — batch delivery uses same socket, same direction
- [ ] No unintended coupling — batch format is IPC-internal, plugins see individual events
- [ ] No duplicated functionality — replaces per-event delivery, doesn't layer on top
- [ ] Zero-copy preserved — offset-based slicing avoids allocation on reader side

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Single event in channel | Batch of 1 delivered and acked correctly |
| AC-2 | N events queued (burst) | All N delivered in one batch write, one ack |
| AC-3 | Batch exceeds buffer size | Flush partial batch, start new batch for remaining |
| AC-4 | Plugin receives batch | SDK dispatches each event individually to handler |
| AC-5 | Delivery timeout | Batch write respects context deadline |
| AC-6 | Plugin error response | Error propagated to delivery loop |
| AC-7 | Offset extraction on reader | Events extracted via slice (zero-copy), no allocation per event |
| AC-8 | Empty channel after first event | First event triggers batch, non-blocking drain finds no more, flush batch of 1 |
| AC-9 | Existing plugins work | All functional tests pass (bgp-rib, bgp-rr, bgp-gr) |
| AC-10 | FrameWriter allocation eliminated | Batch write uses pooled buffer, no per-frame `make([]byte)` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBatchFrameWriteRead` | `internal/ipc/framing_test.go` | Round-trip: write batch → read batch, verify events match | |
| `TestBatchFrameSingleEvent` | `internal/ipc/framing_test.go` | Batch of 1 works correctly | |
| `TestBatchFrameOffsetSlicing` | `internal/ipc/framing_test.go` | Reader returns slices into shared buffer (no copy) | |
| `TestBatchFrameMaxSize` | `internal/ipc/framing_test.go` | Batch respects buffer size limit | |
| `TestDeliveryLoopBatching` | `internal/plugin/process_test.go` | Multiple queued events delivered in single batch | |
| `TestDeliveryLoopSingleEvent` | `internal/plugin/process_test.go` | Single event delivered as batch of 1 | |
| `TestSendDeliverBatch` | `internal/plugin/rpc_plugin_test.go` | Batch RPC sends correctly, ack received | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count | 1-65535 | 65535 | 0 (empty batch) | N/A (uint16 max) |
| total_len | 7+ (header min) | MaxMessageSize | 6 (header too small) | MaxMessageSize+1 |
| offset | 0 to payload_len | payload_len-1 | N/A | payload_len (out of bounds) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin functional tests | `test/plugin/` | Events delivered to plugins correctly with batch framing | |

### Future (if deferring any tests)
- Benchmark: measure syscall and goroutine reduction under load
- Property test: random batch sizes round-trip correctly

## Files to Modify
- `internal/ipc/framing.go` - Add `BatchFrameWriter` and `BatchFrameReader` with offset-based format
- `pkg/plugin/rpc/conn.go` - Add `WriteBatch()` (direct write, no per-call goroutine) and `ReadBatch()` methods
- `internal/plugin/rpc_plugin.go` - Add `SendDeliverBatch()` method using new batch RPC
- `internal/plugin/process.go` - Modify `deliveryLoop()` to drain channel and batch events
- `pkg/plugin/sdk/sdk.go` - Handle `deliver-batch` RPC, dispatch individual events to handler
- `pkg/plugin/rpc/types.go` - Add `DeliverBatchInput`/`DeliverBatchOutput` types

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| RPC count in arch docs | Yes | `docs/architecture/api/ipc_protocol.md` — document batch delivery RPC |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | Yes | `.claude/rules/plugin-design.md` — mention batch delivery |
| Functional test for new RPC/API | Yes | Existing plugin tests cover delivery |

## Files to Create
- `internal/ipc/batch.go` - Batch frame writer/reader implementation
- `internal/ipc/batch_test.go` - Batch frame round-trip and boundary tests

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write batch frame unit tests** (`internal/ipc/batch_test.go`) — round-trip, single event, offset slicing, boundary tests → Review: edge cases? Boundary tests?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement `BatchFrameWriter` and `BatchFrameReader`** in `internal/ipc/batch.go` — pooled buffer, offset table, NUL terminator → Review: buffer-first? No `make([]byte)` per write?
4. **Run tests** → Verify PASS (paste output)
5. **Write delivery batch tests** (`internal/plugin/rpc_plugin_test.go`, `process_test.go`) — batch RPC, drain-and-flush
6. **Run tests** → Verify FAIL
7. **Implement `SendDeliverBatch`** in `rpc_plugin.go` — direct write (no per-call goroutine)
8. **Modify `deliveryLoop`** in `process.go` — drain channel non-blocking, batch, flush
9. **Update SDK** in `pkg/plugin/sdk/sdk.go` — handle `deliver-batch` RPC
10. **Run tests** → Verify PASS
11. **Update `docs/architecture/api/ipc_protocol.md`** — document batch delivery format
12. **Verify all** → `make ze-lint && make ze-unit-test && make ze-functional-test`
13. **Critical Review** → All 6 checks from `rules/quality.md`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Batch frame parse error | Step 3 — check offset encoding, endianness |
| Plugin doesn't receive events | Step 9 — verify SDK dispatches `deliver-batch` to handler |
| Delivery timeout | Check: batch write must use connection deadline, not goroutine bridge |
| Existing tests fail | Check: old `deliver-event` RPC still handled for non-batch plugins |
| Offset out of bounds on reader | Step 3 — validate offsets against payload length |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- `FrameReader` already uses `bufio.Scanner` (read side is buffered) — only write side needs batching
- `FrameWriter.Write()` allocates `make([]byte, len+1)` per frame — buffer-first violation to fix
- `CallRPC`/`WriteWithContext` spawn per-call goroutines — batch bypasses this entirely
- Delivery channel capacity (64) naturally bounds max batch size
- Flush trigger: drain channel non-blocking; when empty or buffer full, flush
- Single-event batches are common (low-rate events) — must be efficient

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
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
- [ ] Spec moved to `docs/plan/done/NNN-batched-ipc-delivery.md`
- [ ] **Spec included in commit**
