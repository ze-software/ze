# 846 -- move show bgp decode/encode into the BGP command surface

Closes the deferral [845](845-plugin-self-containment.md) lines 40-44 left open:
the offline `show bgp decode` / `show bgp encode` diagnostics still had their
YANG command nodes in the central `cmd/show` schema, which fails the
self-containment removal test (a `show bgp ...` branch in a central verb
package). Relocated them to a BGP-owned schema package.

## What moved

- New package `cmd/ze/bgp/schema` (embed + `yang.RegisterModule` in `init()`)
  with `ze-bgp-tools-cmd.yang` declaring `show bgp decode` / `show bgp encode`
  (same `ze:command "ze-show:bgp-decode"` / `...-encode`, same descriptions).
- Blank-imported from `cmd/ze/bgp/register.go`, which already owns the offline
  handlers (`Run`) and the `cmdregistry.MustRegisterLocal("show bgp decode"...)`
  local dispatch. So all three -- schema node, local registration, handler --
  now live in one removable package.
- Removed the whole `container bgp { decode; encode }` from
  `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`. The central show
  schema now declares no `container bgp` at all.

## Why this is safe (behavior-preserving)

The command tree is the MERGE of every registered YANG module. Before, the
`show > bgp` subtree came from peer + rib + central (decode/encode); now it
comes from peer + rib + the new tools module. The merged tree is structurally
identical, so completion, help, validation, and dispatch are unchanged.
`show bgp decode/encode` already executed via the local handler (no RPC handler
exists for `ze-show:bgp-decode`), and that path is untouched.

## Decisions

- **Wire-method strings kept (`ze-show:bgp-decode/encode`).** They are labels,
  never dispatched as RPCs (execution short-circuits to the local handler in
  `cmdutil.RunCommand`). Renaming would ripple into the `help_test.go` fixture
  and docs for zero functional gain. Ownership is established by WHICH package
  declares the node, which is what the self-containment test enforces -- not by
  the label namespace.
- **Schema lives under `cmd/` (`cmd/ze/bgp/schema`), not `internal/`.** Offline
  command + handler ownership is `cmd/ze/bgp`; the schema belongs next to it
  (plugin-self-containment.md "Where a Plugin's Surface Lives" row: offline/root
  command + handler -> owner package). `cmd -> internal` import is legal; the
  daemon CLI tree includes decode/encode only because `cmd/ze` links the
  package -- verified via `ze show bgp help`.

## Gotcha -- the schema->binary link was unguarded

The new module reaches the tree ONLY through the blank import in
`cmd/ze/bgp/register.go`. Drop it and decode/encode silently vanish from help
and completion while `ze show bgp decode <hex>` keeps working (separate local
handler), so nothing caught it: the existing `cli-show-bgp-encode.ci` asserts on
`ze show bgp encode --help`, which prints the LOCAL handler usage, not the tree.
`TestBGPToolsSchemaOwnsDecodeEncode` checks YANG content, not registration.
Closed with `test/parse/cli-show-bgp-tools-help.ci`: it asserts `ze show bgp
help` (group help, enumerated from the merged tree) contains the decode/encode
YANG descriptions, which exist nowhere but `ze-bgp-tools-cmd.yang`.

## Gate update

`TestShowSchemaHasNoBGPPluginCommands` flipped decode/encode from a carve-out
("assert still present in central") to banned tokens ("assert absent"). The
owner half is the new `cmd/ze/bgp/schema` test.

## Verification

- `make ze-lint-changed` 0 issues; unit green for `cmd/ze/bgp{,/schema}`,
  `internal/component/cmd/show/schema`, `config/yang`, `command`, `cli`,
  `cli/testing`.
- `bin/ze-test bgp parse -pattern cli-show-bgp` 3/3 (decode, encode, new
  tools-help guard).
- `ze show bgp help` lists decode/encode/peer/rib merged from their owner
  modules.

## Follow-up candidate

`show bgp-health` (`container bgp-health`, handler in central `cmd/show/show.go`)
is BGP-specific but a separate hyphenated command, not part of `show bgp ...`.
Same self-containment question, out of scope here.

## Files

- `internal/component/bgp/cli/yang/{ze-bgp-tools-cmd.yang,embed.go,register.go,schema_test.go}`,
  `internal/component/bgp/cli/register.go`
- `internal/component/cmd/show/yang/{ze-cli-show-cmd.yang,self_containment_test.go}`
- `test/parse/cli-show-bgp-tools-help.ci`
- `ai/rules/plugin-self-containment.md`, `docs/contributing/documentation-testing.md`
