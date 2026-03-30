# 496 -- cli-dispatch

## Context

Ze had 14 static `usage()` functions hardcoding command lists across `cmd/ze/*/main.go`, with a 20-case switch for top-level dispatch. Adding a command required: writing the handler, adding a switch case, updating the static help string, and adding it to `ze help --ai`. Meanwhile, daemon commands already had dynamic dispatch via `RegisterRPCs()` + YANG + `BuildCommandTree()`. The goal was to extend that YANG-driven pattern to all commands so both `ze <verb> <noun>` and `ze cli` share the same grammar.

## Decisions

- Unified verb dispatch over per-package static switches: `ze show`, `ze set`, `ze del`, `ze validate`, `ze monitor` are now top-level verbs dispatched through the YANG command tree, not hardcoded subcommand packages.
- YANG descriptions as authoritative source over `RPCRegistration.Help` field: removed the `Help` field from both `RPCInfo` (command tree building) and `RPCRegistration` (handler registration). Descriptions come from YANG `description` fields, looked up by walking the YANG command tree.
- `BuildTree()` no longer sets `Description` from `RPCInfo`: the YANG command tree (`yang.BuildCommandTree()`) provides descriptions independently. `BuildTree()` only builds the structural tree for dispatch and completion.
- `ze cli -c` flag over `--run` for non-interactive one-shot commands (mirrors ssh -c convention).
- `ze validate config <file>` as the canonical path over `ze config validate <file>`: the verb-first grammar matches the unified tree, though both paths still work.

## Consequences

- New commands only need a YANG module (with `ze:command` extension) and an `init()` handler registration. No static help strings, no switch cases.
- Help text at any level is generated from YANG: `ze help`, `ze show help`, `ze show bgp help` all walk the same tree.
- `RPCInfo` struct is now just `CLICommand` + `ReadOnly` -- minimal data for tree construction.
- The `yangDescription()` function in `cmd/ze/cli/main.go` was removed as dead code after `Help` field removal.
- The YANG command tree (`yangCmdTree`) and `YANGCommandTree()` export remain for help generation in `help_ai.go` and `main.go`.

## Gotchas

- Removing `Help` from `RPCInfo` required removing it in three places: the struct field, `BuildTree()` usage in `node.go`, and two call sites in `cmd/ze/cli/main.go` (`BuildCommandTree` and `buildRuntimeTree`).
- The `yangDescription()` helper became dead code after removing the `Help:` field from `RPCInfo` literals, requiring its removal to pass lint.
- The `.ci` test framework requires a `stdin=config` block even when the command reads from a `tmpfs` file, not stdin.

## Files

- `internal/component/command/node.go` -- removed `rpc.Help` assignment from `BuildTree()`
- `cmd/ze/cli/main.go` -- removed `Help:` from RPCInfo literals, removed dead `yangDescription()` function
- `test/parse/cli-validate-config.ci` -- new functional test for `ze validate config <file>` verb dispatch
