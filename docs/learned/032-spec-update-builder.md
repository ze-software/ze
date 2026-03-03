# 032 — UPDATE Message Builder Pattern

## Objective

Design a fluent `UpdateBuilder` for constructing BGP UPDATE messages with automatic attribute ordering and required-attribute validation, replacing 10+ repetitive builder functions.

## Decisions

- Builder stores typed attributes and sorts by type code at `Build()` time, ensuring RFC 4271 Section 5 compliance without requiring callers to get ordering right.
- `Build()` returns `[]*Update` (slice) to handle large NLRI sets that exceed message size limits.
- Address family is auto-detected from NLRI type, not passed explicitly.
- Required-attribute validation (`ORIGIN`, `AS_PATH`, next-hop missing → error) happens at `Build()` time.

## Patterns

- This spec planned the builder; actual implementation of `BuildUnicast`, `BuildVPN`, etc. was delivered incrementally across later specs (030, 034, etc.).

## Gotchas

None — design spec. Key insight: the existing `append()`-based builders scattered across `peer.go` had no central validation point; centralising in a builder fixed ordering bugs for free.

## Files

- `internal/bgp/message/update_build.go` — `UpdateBuilder` and per-family `Build*` methods
