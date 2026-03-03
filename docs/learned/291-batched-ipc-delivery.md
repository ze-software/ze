# 291 — Batched IPC Event Delivery

## Objective

Replace per-event IPC writes (1 syscall + 1 ack + 2 goroutines per event) with a drain-and-batch pattern (1 write + 1 ack + 0 goroutines per batch), reducing CPU overhead in the plugin event delivery hot path.

## Decisions

- JSON-RPC `deliver-batch` method over binary offset table: binary uint32 headers can contain NUL bytes (0x00), which break the existing NUL-delimited framing. JSON-RPC reuses the existing framing protocol unchanged.
- `deliveryLoop` drains the channel non-blocking after the first event (select with `default:`), then flushes the batch — channel capacity (64) naturally bounds max batch size. No separate batch size configuration needed.
- Pooled buffer (`sync.Pool`) for batch construction — no per-frame `make([]byte)` allocation.
- `CallBatchRPC` bypasses the per-call goroutine bridge (`WriteWithContext`) entirely — batch write uses direct `conn.Write()` with a deadline, which became the foundation for spec 292.

## Patterns

- Events embedded as raw JSON in the batch frame (no double-encoding) — the JSON is already formatted; wrapping it in another JSON layer would require escaping.
- `FrameReader` was already buffered (uses `bufio.Scanner`) — only the write side needed batching.
- Python SDK (`test/scripts/ze_api.py`) also needed `deliver-batch` handling with a `_pending_events` queue — every new RPC method must be added to all SDK implementations, including the Python test helper.

## Gotchas

- Binary batch format rejected: uint32 values can contain NUL bytes, breaking NUL-delimited framing. Check framing delimiter compatibility before choosing a binary format.
- Python SDK was forgotten initially — functional test #3 failed when plugins received the batch but discarded events. Rule: new RPC → check all SDK implementations (Go + Python).
- AC-7 (zero-copy offset slicing) changed to JSON unmarshal on read side — trade-off accepted since primary goal (syscall/goroutine reduction) was achieved.

## Files

- `internal/ipc/batch.go` — `WriteBatchFrame`, `ParseBatchEvents`, `batchBufPool`
- `internal/ipc/framing.go` — `WriteRaw`, `RawWriter` added; `// Related:` cross-reference to batch.go
- `pkg/plugin/rpc/conn.go` — `CallBatchRPC`, `WriteRawFrame`, `writeBatchFrame`
- `internal/plugin/process.go` — drain-and-batch `deliveryLoop`
- `pkg/plugin/sdk/sdk.go` — `handleDeliverBatch` + `deliver-batch` dispatch case
- `test/scripts/ze_api.py` — `deliver-batch` handler + `_pending_events` queue
- `docs/architecture/api/process-protocol.md` — event delivery section updated
