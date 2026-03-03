# 178 — Lint Fixes (No Interface Changes)

## Objective

Resolve all lint issues that do not require changing function or method signatures, bringing `make lint` to zero issues.

## Decisions

- `hugeParam` issues skipped — fixing them requires pointer receivers which change interfaces.
- `errcheck` on `_, _ = fn()` with `check-blank: true` still triggers — must use `//nolint:errcheck` explicitly for best-effort writes, not `_, _ =` suppression.
- `//nolint:errcheck` appropriate for: `Close()` in defer, `Write()` for logging output, `fmt.Fprintf` to stderr for CLI messages.

## Patterns

- Fix order: goimports → misspell → godot → govet shadow → prealloc → gocritic fixable → errcheck.
- `rangeValCopy`: large structs in range loops use `for i := range` + `&slice[i]` instead of `for _, v := range`.
- `typeAssertChain`: sequential type assertions convert to type switch.
- `nestingReduce`: long conditional body inverts to `if !condition { continue }`.

## Gotchas

- None.

## Files

- Widespread changes across many files — no single key file.
