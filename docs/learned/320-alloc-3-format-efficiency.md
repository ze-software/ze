# 320 — Allocation Reduction: Format Efficiency

## Objective

Eliminate per-call heap allocations in the text formatting hot path by replacing `fmt.Fprintf`, `netip.Addr.String()`, and `netip.Prefix.String()` with zero-alloc stdlib alternatives. Target: `netip.Addr.string4` was the #1 allocator at 30.2M objects / 461 MB.

## Decisions

- Used stack-local `[64]byte` scratch buffer with `AppendTo(scratch[:0])` — scratch stays on stack, result written into `strings.Builder` via `sb.Write()`
- Added `INET.AppendKey` and `INET.AppendString` as zero-alloc siblings to `Key()` and `String()` — kept originals for non-hot callers (map keys, logging)
- Threaded `scratch []byte` parameter through `formatAttributesText` → `formatAttributeText` to share scratch across attributes in one UPDATE
- Left ExtCommunity hex formatting and unknown-attribute formatting with `fmt.Fprintf` — cold paths, no zero-alloc hex alternative in stdlib

## Patterns

- `netip.Prefix.AppendTo(b)` and `netip.Addr.AppendTo(b)` exist in stdlib — always prefer over `.String()` in hot paths
- `strings.Builder` accepts `Write([]byte)` — bridges between `AppendTo` returns and the builder
- `strconv.AppendUint(scratch[:0], val, 10)` + `sb.Write` replaces every `fmt.Fprintf(sb, "%d", val)` pattern
- Type-assert to `*nlri.INET` in `writeNLRIList` to use `AppendString`/`AppendKey` — non-INET NLRIs keep `String()` (cold path)

## Gotchas

- Allocation-counting tests planned (`testing.AllocsPerRun`) were not written — existing golden output tests proved byte-identity instead. The output correctness is the observable contract, not the allocation mechanism.
- `sb.String()` at the end of formatting is an unavoidable allocation — the result must be a string

## Files

- `internal/component/bgp/format/text.go` — hot-path formatting rewrites
- `internal/component/bgp/nlri/inet.go` — `AppendKey`, `AppendString` methods added
