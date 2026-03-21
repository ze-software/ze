# 103 — WriteTo Error Return (CheckedWriteTo)

## Objective

Add `CheckedWriteTo` variants to all wire-encoding types, providing buffer overflow detection and invalid-state detection without changing the existing unchecked `WriteTo` interface.

## Decisions

- Additive change: `WriteTo` unchanged (fast path, caller guarantees capacity); `CheckedWriteTo` added as opt-in safe path returning `(int, error)`.
- Single error type `wire.ErrBufferTooSmall` — deliberate simplicity; `CheckedWriteTo` validates then delegates to `WriteTo`.
- `CheckedWriteToWithContext` added for context-dependent types (ASPath, Aggregator) that require transcoding context.
- ~70 methods added across the type hierarchy.

## Patterns

- Pattern: check `len(buf) < off + x.Len()` → return `0, wire.ErrBufferTooSmall`; otherwise `return x.WriteTo(buf, off), nil`. Composite types validate total capacity once, then call unchecked child `WriteTo` calls.

## Gotchas

- `WireNLRI` needed refactoring: `LenWithContext` calculates length, `WriteTo` writes directly, `Pack` was calling `WriteTo` — ordering mattered.

## Files

- `internal/bgp/wire/errors.go` — `ErrBufferTooSmall`
- All NLRI types, attribute types, message types — `CheckedWriteTo` added
- `docs/architecture/wire/buffer-writer.md` — updated with `CheckedWriteTo` documentation
