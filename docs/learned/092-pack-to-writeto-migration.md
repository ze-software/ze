# 092 — Pack() to WriteTo() Migration

## Objective

Complete the migration from allocating `Pack()` methods to zero-allocation `WriteTo(buf, off)` across all packages — attributes, NLRI, UPDATE builders, RIB, reactor, session.

## Decisions

- Mechanical refactor, no design decisions. Architecture defined in specs 073 and 075.

## Patterns

- `AttributesSizeWithContext()` added for accurate buffer pre-allocation before a `WriteTo` loop over attributes.
- Extended-length boundary (255 → 256 bytes) requires the `Extended Length` flag change per RFC 4271 Section 4.3 — verified with comprehensive boundary tests.
- All WriteTo methods accept `off int` parameter to write into the middle of pooled buffers.

## Gotchas

- Large migration (31 files, +2623/-131 lines): cbor/base64/hex encoders also needed WriteTo helpers to avoid allocating intermediate buffers.

## Files

- `internal/bgp/attribute/` — Len() added, WriteTo on all types
- `internal/bgp/message/update_build.go`, `update_split.go` — builders migrated
- `internal/rib/commit.go`, `grouping.go`, `outgoing.go`, `update.go` — RIB path migrated
- `internal/reactor/peer.go`, `reactor.go`, `session.go` — session buffer integration
