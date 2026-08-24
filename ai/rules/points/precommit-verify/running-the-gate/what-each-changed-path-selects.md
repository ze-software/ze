---
kind: table
level:
stage:
---
| The change set holds | The scoped stages get |
|----------------------|-----------------------|
| a `.go` file | its package, plus every importer within two levels, with the feature tags on |
| `go.mod`, `go.sum`, or a `vendor/` path | `./...`, and the widening names the path: a dependency moved, so every package that compiles against it is reachable |
| a kind a rule names: `.py`, `rfc/`, `.md` under `ai/`, `plan/` or `docs/`, `Makefile`, `mk/*.mk`, `.github/*.yml`, a `.claude/hooks/` script | the tooling packages whose tests READ that kind, never the whole tree |
| a `.ci`, `.et` or `.wb` body under `test/` | the Go test packages that WALK that corpus. `.ci` selects `./internal/test/runner`, `./scripts/dev`, `./scripts/docvalid` and `./scripts/checks`; `.et` selects `./internal/component/cli/testing` and `./scripts/dev`; `.wb` selects `./scripts/dev` |
| a path under `examples/plugin/go`, matched BEFORE the `.go` rule | no package. It is a separate module, so `go list ./...` never reports it and nothing here compiles or reads it. Ordering is load-bearing: the `.go` rule would seed a directory no package owns and widen the whole run |
| a path under `gokrazy/modcache/` | no package. A third-party module cache every tree walker names in a skip list |
| a `.go` file the unit tag set never compiles, in the module root | `./scripts/dev` and `./scripts/checks`, the tree walkers that read it. `./...` does not compile it either, so widening would buy nothing |
| a `.go` file under `cmd/ze-installer` | `./...`. It was a no-widen row until 2026-08-24 and lost it: `scripts/dev/lint_flavors.py` now lints that package under a `ze_installer` flavor whenever the lint runs over `./...`, so the wide answer is the only one that reports on an edit to the initrd's PID 1 |
| a kind no rule names | the package it sits in when that directory holds Go source, the tooling packages otherwise. The path is NAMED on stderr, which is the evidence for writing it a rule |
| nothing, and `tmp/ze-verify.status` holds no green commit | `./...`, and the widening names the condition. Without a proven commit, every commit in history is unverified, so a clean tree must not select nothing |
