# 167 — YANG Schema Refactor

## Objective

Replace hardcoded Go validation and completion logic in the config editor with YANG-driven alternatives, reducing duplicate maintenance of constraints already expressed in the YANG model.

## Decisions

- Completer fully rewritten to navigate the YANG entry tree directly (`yang.Loader.GetEntry()`), removing all dependency on Go `config.Schema` — chose YANG navigation over Go schema to have a single source of truth.
- `internal/yang/schema.go` was created during planning but deleted when the completer used the YANG entry directly — intermediate adapter was unnecessary.
- Parser still depends on Go schema node types (`FlexNode`, `FreeformNode`, `InlineListNode`, `FamilyBlockNode`): YANG describes WHAT data is valid, not HOW the config syntax is parsed — these are syntax-handling patterns with no YANG equivalent. Full parser replacement remains blocked.
- Editor and ConfigValidator no longer store a `schema` field; they call `config.BGPSchema()` inline when creating parsers, avoiding stale references.

## Patterns

- `errors.As` pattern via `AsValidationError()` helper satisfies `errorlint`; don't type-assert directly on error interface.
- `GetEntry()` returns the processed entry tree post-`Resolve()` — always use this, not `yang.Module` directly, for fields like `Mandatory`.

## Gotchas

- `Validator` name collides with `yang.Validator` already in the package — renamed editor validator to `ConfigValidator`.
- Integer bounds check required before `int → uint16` conversion to satisfy `gosec`.

## Files

- `internal/yang/loader.go` — added `GetEntry()` for processed entry access
- `internal/yang/validator.go` — added `findInEntry()`, `AsValidationError()`, use entry tree for mandatory detection
- `internal/config/editor/validator.go` — wired YANG validator, removed hardcoded `validateHoldTime()`
- `internal/config/editor/completer.go` — rewritten to use YANG tree directly
