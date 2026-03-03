# 088 — Text Mode Per-Attribute Syntax

## Objective

Fix the text mode parser to use attribute names directly as keywords (`origin set igp`) instead of nested inside `attr set` (`attr set origin igp`), matching the intended grammar from the new-syntax spec.

## Decisions

- `attr set <bytes>` kept for wire mode (hex/b64) only: the nested form is semantically correct for raw bytes that cannot be decomposed into individual attribute names.
- Scalar `del [<value>]` semantics: unconditional delete vs conditional delete (error if current value does not match the specified value).
- Old `attr set` syntax in text mode returns an error, not a silent migration: forces callers to update rather than accepting ambiguous input.

## Gotchas

- Existing functional test `.run` files all used the old syntax and needed re-migration via script.
- `rd` and `label` keywords appear in the grammar as scalars but are actually per-NLRI parameters for VPN/labeled families: they are parsed within the `nlri` section, not as top-level attribute accumulators.

## Files

- `internal/plugin/update_text.go` — per-attribute keyword parsing
- `internal/plugin/update_text_test.go` — 50+ tests updated to new syntax
- `test/data/api/*.run` — re-migrated
