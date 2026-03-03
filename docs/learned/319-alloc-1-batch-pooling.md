# 319 — Allocation Reduction: Batch Pooling

## Objective

Replace per-burst slice allocations in five drain/selection functions with per-worker reusable slices. Workers are long-lived goroutines that process serially — per-worker buffer fields are safe without synchronisation.

## Decisions

- Chose caller-provides-buffer pattern (`func drain(buf []T) []T`) over per-struct fields for `deliveryLoop` — simpler, no struct field pollution
- `fwdWorker` struct got `batchBuf` field; `forwardBatch` got `targetBuf` — struct owns the buffer because struct owns the worker
- Reuse with `[:0]` reset before each drain call — preserves backing array, clears logical length

## Patterns

- Per-worker long-lived goroutine = safe reusable buffer: no concurrent access, consumed synchronously before next event
- Forward pool workers exit on idle timeout and restart with `nil` buffer — fresh allocation on first use, avoids stale data
- `unsafe.SliceData` pointer comparison used in tests to prove same backing array (not just same content)

## Gotchas

- `selectForwardTargets` result must not be held across lock boundaries (consumed immediately by `strings.Join`) — safe to reuse buffer only because caller owns the lifecycle
- Forward pool workers can restart; if buffer were on a shared struct, restarted worker could see old data — fresh nil field on struct re-creation prevents this

## Files

- `internal/plugins/bgp/reactor/delivery.go` — `drainDeliveryBatch` signature change
- `internal/plugins/bgp/reactor/forward_pool.go` — `batchBuf` on `fwdWorker`
- `internal/plugin/process_delivery.go` — local vars for both drain and events buffers
- `internal/plugins/bgp-rs/server.go` — `targetBuf` on `forwardBatch`
- `internal/plugins/bgp/reactor/delivery_test.go` — new tests
- `internal/plugin/process_delivery_test.go` — new tests
