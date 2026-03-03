# 220 — WriteTo Remaining Allocs

## Objective

Eliminate remaining heap allocations in the BGP UPDATE encoding path by converting the last `make([]byte)` + `append` patterns to `WriteTo(buf, off)` buffer-first writes.

## Decisions

- Final output buffers (`attrBytes`, `inlineNLRI`) MUST remain as `make([]byte)` — they are returned to callers via the `Update` struct and must survive past the next `resetScratch()` call. The buffer-first rule applies to intermediate encoding, not to output values handed to callers.
- Used existing `WriteLabelStack` instead of creating a new `EncodeLabelStackTo` function — reusing existing code is always preferred when the signature already matches the need.
- `WithMaxSize` methods do not need `resetScratch()` because they write into the caller-provided buffer, not the internal scratch space.

## Patterns

- The buffer-first rule has a clear exception: data that must outlive the encoding call (returned to callers, stored in structs) must be its own allocation. Only intermediate/temporary buffers benefit from pooling.

## Gotchas

- Confusing "intermediate buffer" with "output buffer" would cause use-after-free bugs if scratch buffers were reused while their slices were still referenced by the Update struct.

## Files

- `internal/plugins/bgp/wire/` — UPDATE builder and attribute encoding
- `internal/bgp/message/` — attribute WriteTo implementations
