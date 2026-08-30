---
kind: directive
level: MUST
stage:
---
**Before any design decision, and before proposing a design, ze MUST be grepped for the pattern that already exists: trained instincts about "how software works" are wrong here.** The page for the work MUST be read first: `docs/architecture/core-design.md` for system architecture and data flow, `docs/architecture/module-tiers.md` for placement and compile-out, `docs/architecture/buffer-architecture.md` for the buffer path and pool shapes, `docs/architecture/zefs-format.md` for runtime state, `docs/architecture/web-components.md` for server-rendered markup, and `docs/contributing/ze-go-style.md` for every divergence from standard Go.
