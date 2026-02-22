# Related File References

Rationale: `.claude/rationale/related-refs.md`

## Purpose

`// Related:` comments let Claude load only needed files without scanning the whole package. Complements `// Design:` (architecture docs) by pointing to sibling source files.

## Format

Place after `// Design:` at file top. One line per related file with topic annotation:

| Line | Content |
|------|---------|
| 1 | `// Design: docs/architecture/config/syntax.md — BGP config types` |
| 2 | `// Related: bgp_peer.go — peer parsing and process bindings` |
| 3 | `// Related: bgp_routes.go — route extraction and NLRI parsers` |

## When to Add

| Situation | Action |
|-----------|--------|
| Splitting a file | All resulting files get `// Related:` to siblings |
| Tightly coupled new file | Add to new file + update siblings |
| Touching file with stale refs | Fix (remove deleted, add missing) |

## When NOT to Add

- Standalone in package (no strong coupling to siblings)
- Only related through package's public API

## Maintenance

When renaming/deleting a `.go` file, search for `// Related:` references to that filename and update/remove.

## Exempt

Same as `// Design:`: `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.

## Reference

Spec: `docs/plan/spec-file-modularity.md`. Enforcement hook: planned (see spec AC-3 through AC-6).
