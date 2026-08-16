---
kind: table
level:
stage:
---
| Feeder | What it checks | Run |
|--------|----------------|-----|
| Static gate | Every built-in command (YANG command tree) against R1-R9 (R9 sibling-collision is static-gate-only, it needs sibling context), plus no `--flag` in any `.yang` | `make ze-cli-grammar-check` (NOT a `make ze-precommit-verify` stage -- it is not in `stagesForMode`; the gate reaches CI through `TestCLIGrammarGateStatic` in `scripts/checks/cli_grammar_test.go`, which runs the same checker under the unit stage) |
| Registration | Every plugin `CommandDecl` at registration (`validateCommandName`) | plugin startup in functional/exabgp suites |
| Runtime guard | The runtime built-in assembly (`AllBuiltinRPCs` x `WireMethodToPaths`) re-checked with `ExemptCategory` by wire method; and the `CommandRegistry.Register` boundary rejecting a bad name | `TestRuntimeBuiltinSurfaceGrammar` / `TestRegistrationRejectsBadGrammar` (unit) |
| Root namespace | Every registered root command (`registry.MustRegisterRootHandler` / `RegisterRoot`, enumerated from source) against R9 across surfaces (`grammar.CheckRootNamespace`): a hyphenated root whose left segment names a YANG verb or container is a namespace member masquerading as a compound root. Root handlers never pass through the YANG-tree static gate, so this feeder is the only one that governs them | `make ze-cli-grammar-check` (same gate); `TestRootNamespaceGrammar` (unit) |
| Demo call sites | Every `ze <token>` invocation in `demos/terminal/**/*.sh`: the position-1 token must be a YANG verb, a registered root, or the `-` stdin sentinel. The other feeders check how commands are DECLARED; this one checks the repo's own CALL SITES, which no other gate reaches -- `make ze-precommit-verify` never executes the demos (Docker + VHS, run from `mk/terminal-demo.mk` at release time and by the gh-pages website workflow), so a removed launch form rots there silently -- and since main's own `pages.yml` was deleted, main no longer gets even the after-the-fact signal it once did: `ze <config-file>` stayed in thirteen demo scripts and failed the Deploy website job on every push for four days. This static gate is now the ONLY thing on main that sees a broken demo call site | `make ze-cli-grammar-check` (same gate); `TestCLIGrammarGateStatic` (unit) |
