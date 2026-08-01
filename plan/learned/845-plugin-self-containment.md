# 845 -- plugin self-containment + finishing the command-grammar refactor

Follow-up to [844](844-command-grammar-ownership-first.md). The partial refactor
left in the tree (handover/16) generalized the shared dispatcher to typed,
ArgDef-driven selector extraction and moved command schemas to typed-selector
grammar, but left the internal callers and several handlers on the old grammar,
so `make ze-verify` was red.

## The principle (now a rule)

Remove a plugin and ALL its features (commands, schema, help, doctor checks)
disappear; every OTHER plugin and the core keep working. This is the full
user-facing version of the Proximity Principle's "delete the folder" test.
Written up as `ai/rules/plugin-self-containment.md`, wired into
`ai/INSTRUCTIONS.md` (synced to CLAUDE.md/AGENTS.md), `ai/INDEX.md`, and
cross-referenced from `ai/rules/plugin-design.md`.

## Decisions

- **Shared dispatch carries selector scope, never command spelling.** The
  generic matcher (`internal/component/plugin/server/command.go`) extracts a
  typed selector because a YANG `ArgDef` declares it; it contains no `peer`/
  `bgp`/`bfd` words. The deleted `extractPeerSelector`/`looksLikeIPOrGlob`
  heuristics were the BGP leak 844 warned about.
- **Grammar is replace-outright (cli-grammar.md "Backward Compatibility").**
  Internal `.ci` / `ze_api.py` callers were migrated to the documented forms:
  `show bgp peer <sel> detail|...`, `show bfd session address <addr>`,
  `show l2tp {tunnel,session} id <id>`, `show subscriber id <id> detail`,
  `show pki certificate name <name>`, `show vpn ipsec peer name <name>`.
- **Peer selector is positional (the cli-grammar exception); trailing
  positional selectors are handler-parsed.** `show policy {test,chain} peer
  <name> ...` can't use the dispatcher's mid-path extraction (the selector
  follows the last path token), and a mandatory `selector` leaf collides with
  other ArgString leaves in `validateCommandArgs`. Fix: drop the leaves +
  `RequiresSelector`, let the handler consume `args[0]` as the peer.
- **Typed-selector handlers must read `ctx.Selector(<leaf>)`.** The refactor
  updated bfd/subscriber/iface but missed l2tp/pki/ipsec (still read `args[0]`,
  which is now the action token, not the id/name). Completed those with a
  `ctx.Selector` + positional fallback.
- **`show bgp rib ...` moved to the rib plugin schema** (handlers were already
  there). Only the offline `show bgp decode/encode` diagnostics remain in
  central `cmd/show` -- their handlers live in `cmd/ze/bgp` (offline command
  surface), so relocating them is the larger offline-command-ownership spec
  ([spec-command-surface-ownership]).

## Gotchas

- A missing **mandatory** YANG leaf is rejected by `validateCommandArgs` as a
  non-nil Dispatch error, which `handleDispatchCommandRPC` turns into an RPC
  error -- `ze_api.py`'s `_call_engine` raises on that, crashing the test
  script (looks like a timeout). Handler operational errors return
  `(Response{status:error}, nil)` and are graceful. Tests that probe
  "missing id" must tolerate the RPC-level rejection.
- The pki export `.ci` was passing **falsely**: a broad `try/except` swallowed
  the "unknown command" from the old grammar so the real assertions never ran.
  Migrating the grammar + fixing the handler made it genuinely exercise.
- `pki`/`ipsec` use a typed `name <name>` sub-container (value in
  `ctx.Selector("name")`); `crashes`/`flow-export` keep an optional `name`
  leaf on the command container and the handler parses the `name` keyword from
  trailing args. Both are valid; don't confuse them.

## Verification

- `bin/ze-test bgp plugin -a` 400/400; changed-package unit tests green;
  `TestShowSchemaHasNoBGPPluginCommands` is the first removal-compliance gate.

## Files

- `ai/rules/plugin-self-containment.md`, `ai/INSTRUCTIONS.md`, `ai/INDEX.md`,
  `ai/rules/plugin-design.md`
- `internal/component/cmd/show/{show_policy.go,show_policy_test_cmd.go,ipsec.go}`,
  `internal/component/pki/show.go`, `internal/component/l2tp/cmd/l2tp.go`
- `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang`,
  `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`,
  `internal/component/cmd/show/yang/self_containment_test.go`
- many `test/plugin/*.ci`, `test/ipsec/ipsec-show-peer.ci`
