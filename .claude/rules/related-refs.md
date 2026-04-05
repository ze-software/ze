---
paths:
  - "**/*.go"
---

# File Cross-References

Rationale: `.claude/rationale/related-refs.md`

## Purpose

Cross-reference comments let Claude load only needed files without scanning the whole package. Complements `// Design:` (architecture docs) by pointing to sibling source files.

## Keywords

Three directional keywords express the relationship between files:

| Keyword | Direction | Meaning | Example |
|---------|-----------|---------|---------|
| `// Detail:` | Hub → Leaf | "details of this topic are in X" | `reactor.go` → `reactor_api.go` |
| `// Overview:` | Leaf → Hub | "broader context is in X" | `reactor_api.go` → `reactor.go` |
| `// Related:` | Peer ↔ Peer | "sibling at same level" | `reactor_api_batch.go` ↔ `reactor_api_forward.go` |

**Hub file** = orchestrator, core types, dispatch (typically shortest name: `server.go`, `decode.go`, `peer.go`).
**Leaf file** = specific concern split from hub (has suffix: `_text`, `_routes`, `_batch`, or prefix: `cmd_`).
**Peer files** = siblings at same abstraction level, neither contains the other.

## Bidirectionality (BLOCKING)

Every cross-reference MUST have a back-reference. If A references B, B must reference A.

| A says | B must say |
|--------|-----------|
| `// Detail: B.go — topic` | `// Overview: A.go — topic` |
| `// Overview: B.go — topic` | `// Detail: A.go — topic` |
| `// Related: B.go — topic` | `// Related: A.go — topic` |

## Format

Place after `// Design:` at file top. One line per reference with topic annotation:

| Line | Content |
|------|---------|
| 1 | `// Design: docs/architecture/config/syntax.md — BGP config types` |
| 2 | `// Detail: bgp_routes.go — route extraction and NLRI parsers` |
| 3 | `// Related: bgp_peer.go — peer parsing and process bindings` |

## When to Add

| Situation | Action |
|-----------|--------|
| Splitting a file | Hub gets `// Detail:` to leaves, leaves get `// Overview:` to hub |
| Tightly coupled new file | Add reference + matching back-reference |
| Touching file with stale refs | Fix (remove deleted, add missing, fix direction) |

## When NOT to Add

- Standalone in package (no strong coupling to siblings)
- Only related through package's public API

## Maintenance

When renaming/deleting a `.go` file, search for `// Detail:`, `// Overview:`, and `// Related:` references to that filename and update/remove.

## Exempt

Same as `// Design:`: `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.

## Reference

Learned: `plan/learned/363-file-modularity.md`. Enforcement hook: `require-related-refs.sh` (exit 2, blocking).
