# 079 — WireUpdate Error Returns

## Objective

Change WireUpdate accessor methods to return `(value, error)` so callers can distinguish valid-empty (`nil, nil`) from malformed (`nil, err`).

## Decisions

- Removed `sync.Once` caching from `Attrs()` entirely instead of making it error-aware: `AttributesWire` is cheap to create (just a slice wrapper), and the `sync.Once` pattern has a fundamental bug when errors are involved — the second call after an error returns `nil, nil` instead of re-returning the error.
- Chose `fmt.Errorf("context: %w", baseErr)` with sentinel errors `ErrUpdateTruncated`/`ErrUpdateMalformed`: allows callers to use `errors.Is()` while preserving context.

## Patterns

- Sentinel errors in `internal/component/plugin/errors.go`, wrapped with location context: `return nil, fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)`.
- `nil, nil` is the valid-empty return (empty section), not an error: callers must handle both empty and error explicitly.

## Gotchas

- `sync.Once` with errors: second call after error returns cached `nil, nil` instead of the error. Never use `sync.Once` for operations that can fail if you need error propagation on subsequent calls.
- The O(n²) NLRI splitting issue in `SplitWireUpdate` was discovered during critical review: `ChunkMPNLRI` splits into N chunks, then O(n) loop recombines chunks 1..N back into `remaining`. Fix: return all chunks at once. This was documented for follow-up.

## Files

- `internal/component/plugin/wire_update.go` — changed signatures, removed attrsOnce/attrs fields
- `internal/component/plugin/errors.go` — new: ErrUpdateTruncated, ErrUpdateMalformed
- All callers in `internal/reactor/` and `internal/component/plugin/` updated
