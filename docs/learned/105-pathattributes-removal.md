# 105 — PathAttributes Removal

## Objective

Replace the `plugin.PathAttributes` intermediate struct with `attribute.Builder` for wire-first attribute construction, eliminating the text→struct→wire conversion in favour of text→wire directly.

## Decisions

- Mechanical refactor, no design decisions.

## Patterns

- `PathAttributes` was text → struct → wire bytes. With `Builder`: text → wire bytes (direct). The Builder accumulates wire bytes as text args are parsed.
- MUP-specific keywords must be handled BEFORE `parseCommonAttributeBuilder` — the common parser runs first and consumes `extended-community`, leaving the MUP-specific switch case unreachable.

## Gotchas

- MUP `extended-community` bug: `parseCommonAttributeBuilder` parsed `extended-community` first (stored in builder, returned `consumed > 0`), so the MUP-specific `case "extended-community"` was never reached. Fix: handle MUP keywords before calling the common parser.

## Files

- `internal/bgp/attribute/builder.go` — text parse methods added
- `internal/plugin/types.go` — `PathAttributes` removed, replaced by `Builder`
- `internal/plugin/route.go` — `parseCommonAttributeBuilder`, MUP keyword ordering fix
