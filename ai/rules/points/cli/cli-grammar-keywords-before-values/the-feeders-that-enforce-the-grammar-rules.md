---
kind: table
level:
stage:
---
| Feeder | What it checks | Run |
|--------|----------------|-----|
| Static gate | Every built-in command (YANG command tree) against R1-R9 (R9 sibling-collision is static-gate-only, it needs sibling context), plus no `--flag` in any `.yang` | `./le cli-grammar` (NOT a `./le verify worktree` stage -- it is not in `stagesForMode`; the gate reaches CI through `TestTheRealCheckoutPassesAndWasRead` in `internal/le/cligrammar/cligrammar_test.go`, which runs the same checker over the real checkout under the unit stage) |
| Registration | Every plugin `CommandDecl` at registration (`validateCommandName`) | plugin startup in functional/exabgp suites |
| Runtime guard | The runtime built-in assembly (`AllBuiltinRPCs` x `WireMethodToPaths`) re-checked with `ExemptCategory` by wire method; and the `CommandRegistry.Register` boundary rejecting a bad name | `TestRuntimeBuiltinSurfaceGrammar` / `TestRegistrationRejectsBadGrammar` (unit) |
| Root namespace | Every registered root command (`registry.MustRegisterRootHandler` / `RegisterRoot`, enumerated from source) against R9 across surfaces (`grammar.CheckRootNamespace`): a hyphenated root whose left segment names a YANG verb or container is a namespace member masquerading as a compound root. Root handlers never pass through the YANG-tree static gate, so this feeder is the only one that governs them | `./le cli-grammar` (same gate); `TestRootNamespaceGrammar` (unit) |
| Demo call sites | Every `ze <token>` invocation under `demos/terminal/`: the position-1 token must be a YANG verb, a registered root, or the `-` stdin sentinel. `./le cli-grammar` reads the demo sources; `./le terminal-demo check-all` validates the published artifacts |
| `le` surface | `le`'s own command tree, which the first five feeders never reach because it registers outside the YANG tree and outside `registry.RegisterRoot` | `./le cli-grammar` (same gate) |
| Offline flags | The `cmd/ze/` flag surface against "`--flag` or Keyword" below: a root spelled as a flag, a flag a client sends to the daemon, a flag repeating a pipe operator, and a flag the parser and `registry.RegisterCommandFlags` disagree about. What a static scan cannot place is COUNTED and printed, never dropped | `./le cli-grammar` (same gate); the shapes are pinned by `TestTheFlagFeederDrawsARowForEachShape` (unit) |
