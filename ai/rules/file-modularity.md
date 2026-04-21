# File Modularity

Rationale: `ai/rationale/file-modularity.md`

## One Concern Per File

Each `.go` source file contains exactly one concern — a cohesive group of types and functions serving a single responsibility.

| Lines | Action |
|-------|--------|
| < 600 | Fine if single concern |
| 600–1000 | Multiple concerns? Split if yes |
| > 1000 | Almost certainly needs splitting |

Before creating a file: "one concern?" Before adding to one: "belongs to this file's concern?" Past 600 lines: check for multiple concerns.

## Splitting

- Zero semantic effect — Go compiles all files in a package together
- File-local types move with their functions
- Shared test helpers stay in base `_test.go`
- `goimports` handles import cleanup
- Name after concern: `reactor_announce.go`, `session_handlers.go`
- New files: copy `// Design:` from original, review topic annotation (`rules/design-doc-references.md`)
- All resulting files: `// Related:` to siblings (`rules/related-refs.md`)

## NOT a Reason to Split

- Large but single coherent concern (capability registry, pool internals)
- CLI file with one-function-per-subcommand
- Dependency chain where dispatcher references all implementations

## Reference

Learned: `plan/learned/363-file-modularity.md`. Prior: `plan/learned/221-file-splitting.md`.
