# 1317 -- wire-edit-1-base-index

## Context

The attribute TLV sequence of one received UPDATE was walked three times: by the
RFC 7606 validator, by a second `AttrFind` pass, and by whichever consumer first
touched `Get`/`Has`/`GetRaw` and triggered a lazy index build under a write lock.
The lock existed only to guard that lazy build and the parsed-value cache, so
every forward-path read of an object nobody mutates paid for a mutex. Child 1 of
the wire-edit umbrella builds the index once, eagerly, and publishes it as part
of an immutable base. It is the substrate children 2 to 5 stand on, and it
changes no emitted byte.

## Decisions

- **Build the index in `attribute.NewAttributesWire`, not inside the RFC 7606
  walk**, over emitting it as a by-product of that walk. The walk reads bytes
  that two later branches still change: the Section 3.g keep-first strip wraps a
  NEW `WireUpdate`, and `ApplyAttrDiscard` overwrites a type-code byte in place.
- **Resolve that ordering hazard by ORDERING, not by rebuilding.**
  `enforceRFC7606` calls `Attrs` last, from `publishBase`, so both mutation
  branches complete before any index exists. `StripAttrRanges` and
  `ApplyAttrDiscard` therefore needed no change at all.
- **No new base type**, over the planned `wireu/base.go`. A parallel type
  duplicating `WireUpdate` would be a second read surface over the same bytes
  (`ai/rules/no-layering.md`); nothing in this child needs a field the two
  existing types lack.
- **A 6-byte span with parsed values in a side table**, over the 24-byte
  `attrIndex` with its interface field. One cache line covers a typical UPDATE
  instead of three, and no heap pointer is retained for the life of a cache
  entry whose ceiling is a million entries.
- **A 32-byte presence bitset**, over a 512-byte per-code table. At the cache
  ceiling that is 32 MB against 512 MB, and O(1) buys nothing when the whole
  index is one cache line.
- **Section-relative span offsets**, over absolute payload offsets, so
  `Snapshot()` and any future re-basing need no arithmetic.

## Consequences

- `AttributesWire` is shared-immutable. `Has`, `GetRaw`, `Packed`, `Count` and
  `Spilled` are lock-free; only `Get`, `All` and `ForEach` lock, and only to fill
  the parsed-value cache the show and JSON paths use.
- `AttributesWire.CarryOver` lets a byte-identical rebuild reuse an index, and
  falls back to a full rebuild on a length mismatch rather than publishing an
  index that does not describe its bytes.
- `ze_bgp_update_span_spill_total` makes the inline-capacity question answerable
  from a running daemon instead of from a static census.
- Any future edit that calls `Attrs` earlier in `enforceRFC7606` reintroduces the
  hazard. `TestStripRebuildIndexMatchesPublished` and
  `TestInPlaceDiscardPrecedesIndexBuild` are the tripwires.

## Gotchas

- **A pooled index must keep its spill.** `SpanIndex.reset` discarded the spill
  slice on every rebuild, so a spilled UPDATE re-allocated on each reuse -- the
  pool made it worse, not better. Found by review, fixed in `f1f746fb6`.
- **`before == after` over ZERO borrows is a vacuous leak test.** Every
  read-buffer assertion in the reactor package was that shape, one of them
  saying so in its own comment, while `adoptFwdHandle` stayed live at two
  production sites. Reaching a real borrow needs an IBGP destination on a 2-byte
  send context; an eBGP one folds the width change into the edit set and borrows
  nothing.
- **The umbrella's AC-6 saving does not exist on iBGP sessions.** The second
  PrefixSID walk was always gated on `!isIBGP && !AcceptSRv6PrefixSID`, and it
  had already been deleted by `8e67a9b03` before this child started.
- **Comparing a new index against the lazy one is impossible once the lazy one is
  deleted.** The oracle has to share no code with the builder, so the test
  compares against an independent `AttrIterator` walk instead.

## Files

- `internal/core/bgp/attribute/span.go`, `span_test.go` (new)
- `internal/core/bgp/attribute/wire.go`, `iterator.go`
- `internal/component/bgp/wireu/wire_update.go`, `base_test.go` (new)
- `internal/component/bgp/reactor/session_validation.go`, `session_span_index_test.go` (new)
- `internal/component/bgp/reactor/reactor_metrics.go`
- `docs/architecture/wire/attributes.md`, `docs/architecture/memory/lifetime-contracts.md`
