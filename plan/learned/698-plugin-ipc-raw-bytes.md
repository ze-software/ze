# 698 -- plugin-ipc-raw-bytes

## Context

After spec-fmt-0-append eliminated all internal string conversions on the
filter-dispatch hot path, one allocation remained: `string(scratch)` at each
of the 3 reactor call sites that build filter text from a stack-local scratch
buffer. This conversion existed because `PolicyFilterChain` and
`FilterUpdateInput.Update` take `string`. Goal: drop that last allocation
without changing signatures, wire format, or plugin handlers.

## Decisions

- **`unsafe.String`, not type change.** The original skeleton spec proposed
  changing `FilterUpdateInput.Update` from `string` to `[]byte`. Analysis
  showed this does not help: storing the `[]byte` in the heap-escaping
  `&FilterUpdateInput{}` struct causes Go's escape analysis to move
  `scratchArr` to the heap. The allocation moves but does not disappear.
  `unsafe.String` breaks escape analysis intentionally: the compiler does not
  know the string references the stack array, so the array stays on the stack.
- **65536B scratch for standard + extended messages.** The predecessor spec
  used 4096B, sufficient for typical UPDATEs. Bumped to 65536B so extended
  BGP messages (RFC 8654, up to 65535 bytes wire) also stay on the stack.
  The `peer_initial_sync.go` synthetic single-prefix path stays at 256B.
- **No signature changes.** `PolicyFilterChain` and `PolicyFilterFunc` keep
  their `string` parameters. The fix is local to the 3 call sites. The
  chain's internal string operations (`strings.Fields`, `applyFilterDelta`)
  are untouched.
- **No wire format change.** `FilterUpdateInput.Update` stays `string` in
  JSON. The `unsafe.String` value is consumed by `json.Marshal` (which copies
  bytes to its output buffer) within the synchronous `CallRPC` call. No
  external-facing change.

## Consequences

- **Filter dispatch is 0 allocs/op end-to-end (up to the IPC boundary).**
  `BenchmarkFilterDispatch_ZeroAlloc` confirms: AppendUpdateForFilter +
  unsafe.String + PolicyFilterChain (mock accept) = 0 B/op, 0 allocs/op.
  The predecessor's `BenchmarkFormat_Boundary_StringConvert` continues to
  show 1 alloc/op as a reference measurement of the old path.
- **JSON IPC still allocates.** `json.Marshal` and `json.Unmarshal` in the
  bridge/pipe path allocate their own buffers. Those are IPC-layer
  allocations, not filter-dispatch allocations. A future spec could bypass
  JSON for bridge plugins to eliminate those too.

## Gotchas

- **GC traces string data pointers.** Go's internal string representation
  uses `unsafe.Pointer` (not `uintptr`) for the data field. The GC traces
  this pointer. So even if `scratch` (the slice) goes dead after
  `unsafe.String`, the string's liveness keeps the backing array alive.
  `runtime.KeepAlive` is not needed.
- **Heap-spill case is safe.** When `append` grows scratch beyond the
  stack array, the new backing array is on the heap. `unsafe.String`
  creates a string from that heap array. The GC traces the string's data
  pointer and keeps the heap array alive. No dangling reference.
- **Synchronous CallRPC is the safety invariant.** The `unsafe.String` is
  safe because `CallRPC` (both bridge and pipe paths) blocks until the
  plugin responds. `json.Marshal` copies the string bytes before the
  function returns. If `CallRPC` were ever made asynchronous, this
  invariant would break. The `//nolint:gosec // audited:` comment
  documents this.

## Files

- `internal/component/bgp/reactor/reactor_notify.go` (import filter call
  site: `string(scratch)` replaced with `unsafe.String`, scratch bumped
  to 65536B)
- `internal/component/bgp/reactor/reactor_api_forward.go` (export filter
  call site: same changes)
- `internal/component/bgp/reactor/peer_initial_sync.go` (default-originate
  call site: `string(scratch)` replaced with `unsafe.String`, 256B kept)
- `internal/component/bgp/reactor/filter_dispatch_alloc_test.go` (new:
  `BenchmarkFilterDispatch_ZeroAlloc`)
