---
kind: directive
level: MUST
stage:
---
- **Every registered native check MUST declare the rule point it enforces**, with a `// ze point: <rule>/<section>/<slug>` line in its Go doc comment, or `// ze point: none -- <why>` when nothing written binds it. `nativeHookActions` in `internal/le/hookruntime/runtime.go` is the authority for what is registered; `./le rules gate-map-report` refuses a binding on an unwired function and a registered check with no binding.
- **The `Check` and `Enforces` columns of the tables in `.claude/hooks/README.md` MUST name exactly the registered checks and the rule stems their bindings name.** They are a gated mirror rather than a second roster: `hookTableProblems` in `internal/le/rules/hooktable.go` compares them against the Go registry. What each check triggers on and what it does is documentation, and it sits in the remaining column of the same table.
