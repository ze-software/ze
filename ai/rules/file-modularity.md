# File Modularity

**When:** when a `.go` file grows past ~600 lines, or starts holding more than one concern
**Severity:** advisory

## Directives

Rationale: `ai/rationale/file-modularity.md`

## One Concern Per File

Each `.go` source file contains exactly one concern — a cohesive group of types and functions serving a single responsibility.

The line thresholds exist for **context economy**: any task touching a file
should be able to load that file's whole concern without dragging unrelated
code into context. The corollary (per Thomas): a split is only worth doing
when the separation is RIGHT — a forced mechanical split that scatters one
concern across files is worse than one large cohesive file. The post-edit
size warning is deliberately non-blocking for this reason; treat it as a
prompt to check cohesion, not an order to cut.

| Lines | Action |
|-------|--------|
| < 600 | Fine if single concern |
| 600–1000 | Multiple concerns? Split if yes |
| > 1000 | Almost certainly needs splitting |

Before creating a file: "one concern?" Before adding to one: "belongs to this file's concern?" Past 600 lines: check for multiple concerns.

## Splitting

- **Tool:** `go build -o bin/go_extract ./scripts/dev/go_extract.go && bin/go_extract <source.go> <dest.go> <symbol1> [symbol2 ...]`
  Moves named declarations (with doc comments) to dest, runs `goimports` on both.
  Note: `goimports` cannot resolve aliased imports; add those manually to the new file.
- Zero semantic effect — Go compiles all files in a package together
- File-local types move with their functions
- Shared test helpers stay in base `_test.go`
- `goimports` handles import cleanup
- Name after concern: `reactor_announce.go`, `session_handlers.go`
- New files: copy `// Design:` from original, review topic annotation (`ai/rules/design-doc-references.md`)
- All resulting files: `// Related:` to siblings (`ai/rules/related-refs.md`)

## Exempt: Test Files

`_test.go` files are not subject to line-count thresholds. Tests grow with coverage and table-driven cases; splitting them adds navigation cost without improving production code clarity.

## NOT a Reason to Split

- Large but single coherent concern (capability registry, pool internals)
- CLI file with one-function-per-subcommand
- Dependency chain where dispatcher references all implementations

## Reference

Learned: `plan/learned/363-file-modularity.md`. Prior: `plan/learned/221-file-splitting.md`.
