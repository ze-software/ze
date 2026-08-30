---
kind: directive
level: SHOULD
stage:
---
**The detail behind these directives lives in the pages below, and the relevant page SHOULD be read before allocation or string-building work.**

| Document | Covers |
|---|---|
| `docs/architecture/buffer-architecture.md` | The wire-to-wire buffer path, who owns each buffer, the pools Ze runs, the key wire abstractions, caller-owned buffers, common allocation mistakes |
| `docs/architecture/textbuf-string-building.md` | `textbuf.Buffer` methods, the standalone and append helpers, allocation tiers, and the `fmt`, `strings` and `+` replacement tables |
| `docs/architecture/pool-architecture.md` | Attribute dedup pools, handles, sharding, compaction |
| `docs/architecture/encoding-context.md` | `ContextID` and zero-copy forwarding |
| `docs/architecture/forward-congestion-pool.md` | The two-tier forward pool and per-peer workers |
| `docs/architecture/core-design.md` | System architecture and data flow |
| `ai/rules/architecture.md` | Encapsulation onion, lazy over eager, pool strategy |
| `/ze-find-alloc`, `/ze-fix-alloc` | Audit for a remaining allocation, and fix one |
