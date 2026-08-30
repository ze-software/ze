---
kind: directive
level: SHOULD
stage:
---
**The detail behind these directives lives in the pages below, and the relevant page SHOULD be read before the design work it covers.**

| Page | Covers |
|---|---|
| `docs/architecture/core-design.md` | System architecture, negotiated capabilities, the UPDATE container, RIB storage, plugin API communication, data flow, component boundaries |
| `docs/architecture/module-tiers.md` | The three tiers, the two placement axes, the non-engine category manifest, compile-out, and what `./le tier check` enforces |
| `docs/architecture/buffer-architecture.md` | The buffer path, pool shapes, the wire abstractions, caller-owned buffers |
| `docs/architecture/zefs-format.md` | The zefs store and runtime state through `statestore` |
| `docs/architecture/web-components.md` | Server-rendered markup and the guards that own each property |
| `docs/contributing/ze-go-style.md` | Where Ze diverges from standard Go, in each of seven areas |
