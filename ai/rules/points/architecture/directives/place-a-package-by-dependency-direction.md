---
kind: directive
level: MUST
stage:
---
**Where a package lives under `internal/` is decided by dependency direction, not by size or age: a config-driven engine something else depends on is a component (`internal/component/`), an engine nothing depends on is an edge plugin (`internal/plugins/`), and a package with no config lifecycle and no registry side effect is a core library (`internal/core/`).** A sub-plugin of a subsystem MUST go under that subsystem's namespace (`internal/component/bgp/plugins/<x>`), a non-engine package outside `internal/core/` MUST carry a row in `internal/le/tier/testdata/tier_non_engine_categories.txt` rather than an exception hidden in Go, and a compile-out-able feature MUST be declared once as a `<tag> <pkg>` row in the root `feature-gates.txt` and MUST NOT be imported by always-on code, which reaches it through a registry or a seam. `./le tier check` refuses each violation and `docs/architecture/module-tiers.md` carries the two axes.
