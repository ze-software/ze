# File Cross-References

**When:** splitting a file, or adding a file tightly coupled to a sibling
**Severity:** blocking

## Directives

Rationale: `ai/rationale/related-refs.md`

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
- Relationship is obvious from filename alone (see "Not a Directory Listing" below)

## Not a Directory Listing

`// Detail:` lines should point to files with **non-obvious relationships**, not
enumerate every file in the package. If the relationship is self-evident from the
filename (e.g., `config.go` has config, `validators.go` has validators), omit it.

Good hub header (reactor.go, 15 files in package, lists 5 with non-trivial roles):
```
// Detail: reactor_wire.go — zero-allocation wire UPDATE builders
// Detail: reactor_connection.go — TCP accept, collision detection (RFC 4271 §6.8)
// Detail: forward_pool.go — per-peer forward worker pool
```

Bad hub header (lists every file, duplicating `ls`):
```
// Detail: config.go — config parsing
// Detail: validators.go — validation
// Detail: logger.go — logging
// Detail: types.go — type definitions
```

**Rule of thumb:** if removing the `// Detail:` line would leave a reader unable
to find important code, keep it. If they would find it anyway by scanning filenames,
drop it. Aim for 3-5 references maximum per hub file.

## Maintenance

When renaming/deleting a `.go` file, search for `// Detail:`, `// Overview:`, and `// Related:` references to that filename and update/remove.

## Exempt

Same as `// Design:`: `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.

## Reference

Learned: `plan/learned/363-file-modularity.md`.
