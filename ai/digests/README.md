# Subsystem flow digests

<!-- Living docs, maintained by hand (NOT generated). -->

Each file here is a "current flow" digest for one subsystem: what it is, how data
flows through it (entry to exit, with `file:line`), the load-bearing files, and the
invariants and gotchas. Read the digest to orient before diving into a subsystem, then
open the files it names.

These are **living** documents. When you change a subsystem's flow, update its digest
in the same work. They are hand-maintained, not generated, so treat any detail as a
strong hint and verify `file:line` before relying on it. The historical record lives in
`plan/learned/`, the canonical design in `docs/architecture/`; a digest is the
fast-orientation layer between them, and `ai/PACKAGE-MAP.md` is the per-package index
below it.

| Digest | Subsystem |
|--------|-----------|
| `bgp-reactor.md` | BGP reactor / session / FSM (the peer event loop) |
| `wire-and-pools.md` | Wire encoding + buffer/pools (buffer-first, zero-copy) |
| `rib.md` | RIB: route storage + best-path selection |
| `config-pipeline.md` | YANG config: File to Tree to ResolveBGPTree to live peers |
| `plugin-transport.md` | Engine to plugin: registry, DirectBridge, EventBus |
| `cli-editor.md` | CLI / config editor (SSH, YANG editor, completion) |

To add a subsystem: trace it from real code, write `<name>.md` in the same shape, and
add a row here.
