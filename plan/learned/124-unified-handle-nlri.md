# 124 — Unified Handle Encoding (Pool Handle Bit Layout)

## Objective

Extend `pool.Handle` to encode pool index, flags, and slot in a single `uint32`, enabling 63 pools × 16M entries each and eliminating redundant struct fields in plugin RIB storage.

## Decisions

- Bit layout: `poolIdx(6) | flags(2) | slot(24)` — 63 pools (idx=63 reserved for `InvalidHandle`), 2 flag bits (bit 0 = hasPathID for ADD-PATH), 16M slots.
- `idx=63` and `slot=0xFFFFFF` reserved as sentinel values for `InvalidHandle`.
- `NewWithIdx(idx, capacity)` — Pool now requires an idx at construction; idx=63 panics.

## Patterns

- Handle encodes metadata directly in the value so consumers need no extra struct fields to track which pool a handle came from.
- `WithFlags()` returns a new handle with modified flag bits, preserving poolIdx and slot — immutable-style handle manipulation.

## Gotchas

None.

## Files

- `internal/pool/handle.go` — `Handle` type, `NewHandle()`, `PoolIdx()`, `Flags()`, `Slot()`, `Valid()`, `WithFlags()`
- `internal/pool/pool.go` — `idx` field, `NewWithIdx()`, slot extraction in all methods
