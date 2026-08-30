---
kind: directive
level: MUST
stage:
---
**A native hook check MUST declare the point it enforces with a `// ze point: <rule>/<section>/<slug>` line in its Go function's doc comment.** The function MUST be a top-level function named in `nativeHookActions`.
**A check that enforces no written point MUST say `// ze point: none -- <why>`, and the reason is REQUIRED.** Without it, "nobody bound this yet" and "there is nothing to bind" look the same.
**`./le rules gate-map-report` refuses a dangling, regressed, or bare binding.** What each of its sets means is in `docs/contributing/rule-authoring.md`.
