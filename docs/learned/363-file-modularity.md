# 363 — File Modularity

## Objective

Split all files over 1000 lines into single-concern files, add `// Related:` cross-reference comments for context-window-efficient navigation, and enforce both with hooks.

## Decisions

- All 19 originally-flagged files (1000-5439 lines) split to under 1000 lines across multiple refactoring passes
- `// Related:` / `// Detail:` / `// Overview:` three-keyword system adopted for hub-leaf-peer relationships
- Bidirectional requirement: if A references B, B must reference A
- Enforcement via `require-related-refs.sh` hook (exit 2, blocking)
- Two files remain over 1000 lines: `chaos/web/viz.go` (1351L, rendering functions) and `research/cmd/attribute-analyser/main.go` (1432L, research tool) — both acceptable single-concern exceptions

## Patterns

- Go compiles all files in a package together — splitting is semantically transparent
- Name after concern: `reactor_announce.go`, `session_handlers.go`
- Shared test helpers stay in base `_test.go` file
- `goimports` (auto-linter hook) handles import cleanup after splits
- 95+ production files now carry `// Related:` cross-references

## Gotchas

- The spec was written when files were 1000-5439 lines. Subsequent feature work (watchdog extraction, config separation, chaos dashboard, etc.) performed much of the splitting organically — the spec captures work that was done incrementally across many specs
- File-local types must move with their functions when splitting
- `// Related:` is only for strong coupling — don't add to files related only through package API

## Files

- `.claude/rules/file-modularity.md` — rule codifying the pattern
- `.claude/rules/related-refs.md` — cross-reference format and rules
- `.claude/hooks/require-related-refs.sh` — enforcement hook
- `docs/learned/221-file-splitting.md` — predecessor summary
