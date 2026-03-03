# 138 — Config BGP Block

## Objective

Wrap all BGP-specific config in a `bgp {}` top-level block, enabling future multi-protocol support. Update template syntax to use `peer <pattern>` with optional `inherit-name` instead of separate `group`/`match` blocks.

## Decisions

- Template design: `template { bgp { peer <pattern> { [inherit-name <name>;] ... } } }`. Without `inherit-name`, template auto-applies to matching peers. With `inherit-name`, it's a named template that peers explicitly `inherit`.
- Pattern `*` means any peer can inherit; specific patterns (e.g., `10.0.0.*`) restrict which peers can reference that template.
- Functionality was integrated into existing files rather than creating separate `internal/config/template.go` and `internal/config/validate.go`.

## Patterns

- Migration framework extended with `wrap-bgp-block` and `template->new-format` transformations to handle the 169 files that needed updating.

## Gotchas

- ExaBGP test input files (`test/exabgp/*/input.conf`) were accidentally migrated by the bulk migration — had to be restored to original ExaBGP format (same issue as spec 135).

## Files

- `internal/config/bgp.go` — `bgp {}` container added to schema, template schema updated
- `internal/config/loader.go` — template application with pattern matching and `inherit` resolution
- 169 files migrated (91 config + 42 encoding + 24 plugin + 12 parsing tests)
