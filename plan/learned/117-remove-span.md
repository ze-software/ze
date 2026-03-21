# 117 — Remove Span Type

## Objective

Remove the `bgp.Span` type introduced in spec 102 and replace all usages with native Go `[]byte` slices in `AttrIterator`.

## Decisions

- Mechanical refactor, no design decisions.

## Patterns

- Native Go slices provide all benefits of `Span` without custom types. `[]byte` is already a zero-copy view into the original buffer.

## Gotchas

- `Span` was over-engineered from the start: designed for compact offset storage (uint16 vs 24-byte slice header), but the use case (caching parsed attribute offsets) was never implemented. Only 1 of 4 planned features was built before removal.
- `docs/architecture/buffer-architecture.md` contained hypothetical code examples for features (ParseUpdateOffsets, WireUpdate.*Span) that were never implemented. These examples were cleaned up.
- The `attrIndex` struct already used raw `uint16` for actual compact offset storage — Span was solving a problem that already had a solution.

## Files

- `internal/bgp/span.go` — deleted
- `internal/bgp/span_test.go` — deleted
- `internal/bgp/attribute/iterator.go` — `Next()` and `Find()` return `[]byte` instead of `bgp.Span`
- `docs/architecture/buffer-architecture.md` — hypothetical examples removed
