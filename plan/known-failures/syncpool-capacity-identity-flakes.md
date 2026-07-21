### `sync.Pool` capacity/identity unit flakes under full-suite GC pressure

Observed 2026-07-07 in a full `ze-verify` run (stage 07 `ze-unit-test-cached`):
`internal/core/textbuf` `TestPoolPreservesCapacityWithoutString` (`"128" is not
greater than or equal to "300"`) and `internal/core/bufpool`
`TestGetReturnsSameBufferAfterPut`. Both assert a `sync.Pool` preserves a
buffer's capacity/identity across Get/Put, which the GC can invalidate under the
memory pressure of the full parallel suite. textbuf passes 5/5 in isolation
(`go test ./internal/core/textbuf/ -run TestPoolPreservesCapacityWithoutString
-count=5`). Same non-deterministic class as learned 881. Triage in isolation;
not a regression from an unrelated change.
