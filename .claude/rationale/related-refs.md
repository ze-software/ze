# Rationale: Related File References

## Problem

When Claude reads a file, it doesn't automatically know which other files in the same package are tightly coupled to it. After splitting `bgp.go` into `bgp.go`, `bgp_peer.go`, `bgp_routes.go`, `bgp_util.go`, reading just `bgp_peer.go` without knowing about the siblings means missing context that affects the work.

Currently, discovering related files requires scanning the package with Glob/Grep — which costs tool calls and context. The `// Design:` pattern solved this for architecture docs. `// Related:` solves it for sibling source files.

## Design

- One `// Related:` line per sibling file, placed after `// Design:`
- Topic annotation on each line (like `// Design:` has) so Claude can decide which siblings to actually load
- Only on files with strong internal coupling — not every file in a package

## Staleness Risk

The main risk is `// Related:` comments becoming stale when files are renamed or deleted. Mitigations:
- Enforcement hook (planned): validates referenced files exist at write time
- Standalone check script: can be run to audit all refs across the repo
- Cultural: same discipline as keeping `// Design:` refs current

## Why Not Use a Centralized Manifest

A per-package `RELATED.md` or similar manifest would be one more file to maintain and wouldn't be visible when reading the source. Inline comments are visible in the file itself, right where they're needed, following the same pattern as `// Design:`.
