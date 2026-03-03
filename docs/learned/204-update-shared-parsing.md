# 204 — Update Shared Parsing

## Objective

Extract `wire.UpdateSections` to share UPDATE section boundary parsing across the decode, encode, and RIB paths without duplicating the offset logic.

## Decisions

- `UpdateSections` stores offset fields (integers), not data slices — callers slice into the original buffer using the offsets, preserving zero-copy; no copies made at parse time.
- `int` instead of `uint32` for offsets — naturally eliminates gosec G115 integer conversion warnings without casts or suppressions.
- `sections.Valid()` replaces a scattered `parsed bool` field — single method for validity check, consistent across callers.
- Benign race on first accessor call (lazy init pattern) documented explicitly — multiple goroutines calling the first accessor simultaneously both compute the same result; Go memory model makes this safe for this pattern.

## Patterns

- Offset-based parsing: parse returns `UpdateSections{withdrawnEnd, attrEnd, nlriEnd int}`; callers slice `buf[start:end]` — consumer accesses data lazily, no intermediate struct.

## Gotchas

- None.

## Files

- `internal/bgp/wire/update_sections.go` — UpdateSections type, Valid()
- `internal/bgp/wire/decode.go` — updated to return UpdateSections
- `internal/plugins/bgp/reactor/` — updated consumers
