# 102 — Buffer-First Migration

## Objective

Migrate ZeBGP to a buffer-first architecture by adding iterator types and direct buffer formatting, eliminating duplication between wire and parsed representations.

## Decisions

- Six layers of UPDATE representation reduced by introducing iterators (`NLRIIterator`, `AttrIterator`, `ASPathIterator`) as zero-allocation traversal over raw wire bytes.
- `RouteJSON` with `MarshalJSON()` added for zero-copy JSON output; replaced `plugin.RIBRoute` which duplicated data.
- `PathAttributes` struct and `rr.UpdateInfo` kept in place (marked deprecated) — removal deferred to spec-105 after verifying no remaining dependencies.
- Parsed attribute storage in `Route` kept for this spec; iterators added additively.

## Patterns

- `WriteTo(buf, off) int` pattern in buffer: iterators yield slices directly from the wire buffer without allocating.
- All new iterator types expose `Next()` returning one element at a time — never collect to slice.

## Gotchas

- `Span` type was introduced in this spec (`internal/bgp/span.go`) but was later found to be over-engineered and removed in spec 117. Custom Span for compact offset storage was never used — native `[]byte` slices suffice.

## Files

- `internal/bgp/nlri/iterator.go` — `NLRIIterator`
- `internal/bgp/attribute/iterator.go` — `AttrIterator`
- `internal/bgp/attribute/aspath_iter.go` — `ASPathIterator`
- `internal/component/plugin/format_buffer.go` — direct buffer formatting functions
- `internal/component/plugin/wire_update.go` — iterator methods on `WireUpdate`
