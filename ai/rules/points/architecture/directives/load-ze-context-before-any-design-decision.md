---
kind: directive
level: MUST
stage:
---
**Before any design decision (communication mechanism, naming, package placement, platform backend, lifecycle), the reading named below for that artifact and that area MUST be loaded. Trained instincts about "how software works" are wrong here: ze has opinions, and `docs/contributing/ze-go-style.md` names each divergence from standard Go.**
**The "Before Writing Code" checklist MUST be completed before writing any code, tests, or documentation.**
**Before any spec: source MUST be read, current behavior MUST be documented, and existing behavior MUST be preserved by default.**
**Before modifying a file, what else the change obliges MUST be checked. A change to a YANG file, a registration file, Go source, a `.ci` test, a docs page, or a spec has a predictable ripple, and the Impact Analysis directives name it.**
**The full data flow MUST be traced before writing or reviewing a spec.**
**Where a Go package lives under `internal/` is decided by dependency direction, not by size or age: three tiers, two mechanical axes (`docs/architecture/module-tiers.md`). New code MUST land in the correct tier; an engine in the wrong tier fails `./le verify worktree`.**
**Runtime state MUST persist through the managed zefs store, never as a loose file.**
