# Spec: yang-command-arguments-by-leaf-name

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped command handlers pick arguments out of `ValidatedArgs.Tokens()` or `args[]` by positional index instead of reading declared YANG leaves by name, and 25 commands declare the same argument leaf twice on both the `-cmd` container and the `-api` RPC `input`. As a result, CLI usage, the web admin form, MCP tools, and `ValidateArgs` disagree with what handlers accept. These are defects in shipped command surfaces, so they belong before the first release. |
| RFC gaps | - |
| Depends | Split on 2026-11-26 from `plan/release/fixes/spec-yang-single-source-of-truth.md` per owner decision T-1 (2026-11-24: Piece D lands independently of Piece E). Siblings: Piece E `plan/release/fixes/spec-yang-single-source-of-truth.md`, Piece F `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`. Related: `plan/release/fixes/spec-declared-commands-without-leaves.md` (the 13 commands whose leaves sit on RPC input alone), `plan/post-release/features/spec-plugin-command-named-arguments.md` (typed plugin SDK command argument transport). |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Problem.** Two command-argument defects remain across the YANG command tree and BGP command handlers:

1. **Double declaration on `-cmd` containers and `-api` RPC inputs:** Twenty-five commands declare the same argument leaf both on the `ze:command` container in `*-cmd.yang` and on the linked RPC `input` in `*-api.yang`. `cmd/ze/hub/command_meta.go` `buildParamMeta` already prefers `PathToArgDefs(schema)` (`-cmd` container leaves) over `rpc.Input`, falling back to `rpc.Input` only when the `-cmd` container declares no leaves. However, `internal/component/config/yang/rpc_publish.go` `PublishedRPCs` currently publishes `ExtractRPCs`'s `-api` RPC `Input` directly—which feeds `ze help ai --json` and MCP `ze_reference` (`internal/component/aihelp/aihelp.go`), `ze yang doc` (`internal/component/config/yang/cli/tree.go`), and `ze schema` (`internal/component/config/schema/cli/main.go`). Deleting duplicate `-api` RPC `input` blocks without teaching `PublishedRPCs` to populate `RPCMeta.Input` from matching `-cmd` `ArgDefs` would strip input leaves from those three surfaces. Meanwhile, nothing refuses a duplicate leaf across the two modules, so descriptions, types, and `mandatory` flags drift silently.
2. **Positional index reads instead of leaf-name binding:** Compiled-in BGP handlers (`handleTeardown`, `handlePeerDisable`, `handleRaw`, `handleUpdate`, `handleCommit`) and plugin-backed BGP handlers (`injectRoute`, `rpfLookup`, `showIRRPrefix`, `showIRRCheck`, `handleShow` in `healthcheck`, `aspaCommand` and `roaCommand` in `rpki`) still read `validated.Tokens()` or `args[i]` by position or lack declared leaves on their `-cmd` containers. Furthermore, `healthcheck` and `rpki` have no `-cmd` YANG modules or builtin proxy forwarders (`pluginserver.RegisterRPCs`), and `request commit` registers one catch-all `ze-bgp:commit` handler over an empty container while still accepting the retired reversed `<name> <action>` grammar. Consequently, keyword-value invocations emitted by `command.WriteInvocation` (web admin form and MCP tools), such as `teardown cease-subcode 2`, `disable message "going down"`, `raw encoding hex data ...`, or `request bgp rib inject peer ... family ... prefix ...`, fail or misparse while CLI usage and `ValidateArgs` omit or under-constrain their arguments.

**Goal.** Enforce one YANG declaration per command argument (the `-cmd` container leaf is the single source of truth; schema build refuses any leaf duplicated on the linked `-api` RPC `input`, and `PublishedRPCs` populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` so `aihelp`, `yang/cli`, and `schema/cli` keep complete input metadata), move every owned compiled-in BGP handler onto `ValidatedArgs` leaf-name accessors (`Value`, `Keyed`, `PositionalTokens`, following `handlePeerNotify`, including splitting `request commit` into 7 typed `ze-bgp:commit-*` sub-actions and retiring `<name> <action>`), and declare typed `-cmd` leaves with builtin proxy forwarders (following `filter_irr/cmd_irr.go`) and keyword-value handling on the owned plugin-backed BGP commands so CLI text, web admin forms, and MCP tools behave identically.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md`, "Every route validates" and "YANG-Typed Command Arguments"
  → Decision: every command route runs `ValidateArgs` against `-cmd` container leaves; handlers read bound leaves through `ValidatedArgs.Value`, `Keyed`, `Flag`, and `PositionalTokens`, while `ze:rest-of-line` captures trailing free-text words cleanly
  → Constraint: `handlePeerNotify` (`internal/component/bgp/plugins/cmd/peer/notify.go`) is the reference pattern for `ze:rest-of-line` and mixed positional/keyword binding; `send bgp <sel> update <encoding> ...` keeps the recursive update-text route expression as opaque trailing data passed to `ParseUpdateText`/`ParseUpdateWire`
- [ ] `docs/architecture/config/yang-config-design.md`, "2. YANG Module Architecture"
  → Decision: departs from `ze:rpc` documenting input leaves on `-api` RPCs when `-cmd` declares leaves; `-cmd` container leaves are the single input declaration, `PublishedRPCs` populates `RPCMeta.Input` from `-cmd` `ArgDefs`, and section 2 of `yang-config-design.md` is updated in the same change
  → Constraint: `-cmd` containers own CLI/API/web/MCP/reference command argument leaves (`extractArgDefs`, `PathToArgDefs`, `PublishedRPCs`, `buildParamMeta`); a linked `-api` RPC `input` must not duplicate those leaves
- [ ] `docs/architecture/api/architecture.md`, "Command parameter values"
  → Constraint: `command.WriteInvocation` (`internal/component/command/arguments.go`) writes structured parameters in keyword-value form (with anchored selectors bare); every owned handler must accept those exact named tokens
- [ ] `docs/architecture/cli/command-verbs.md`, "Everything is a selector the operator types", "Adding a command", and T-5
  → Constraint: every argument a handler reads MUST be declared in the schema; T-5 (`peer raw` arguments) is retired once `handleRaw` reads its declared leaves via `ValidatedArgs`
- [ ] `ai/rules/cli.md`, "CLI Grammar: Keywords Before Values"
  → Constraint: peer commands address a peer positionally (`request peer <selector> ...`); every other free-form value follows a declared keyword; Ze is unreleased, so the deprecated `request commit <name> <action>` grammar is replaced outright by the 7 typed `request commit <action>` commands and every `.ci`/unit/fixture sender is updated together
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/hub-engine.md`, `ai/doors/forwarding.md`, `ai/doors/core.md`, `ai/doors/api-surfaces.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md`
  → Constraint: a plugin owns its own YANG `-cmd` module and builtin proxy forwarders (so `./le doc yang-contract command-contract` finds no `OrphanYANG`); `cmd/ze` and `internal/component/command` stay generic with zero BGP-specific special cases

**Key insights:**
- `cmd/ze/hub/command_meta.go` `buildParamMeta` (lines 338-368) already derives REST/gRPC/MCP/web parameters from `PathToArgDefs(schema)` first, falling back to `rpc.Input` only when `defsByPath[path]` is empty. Meanwhile, `internal/component/config/yang/rpc_publish.go` `PublishedRPCs` feeds three other surfaces directly (`internal/component/aihelp/aihelp.go` `schemaRegistryFrom`/`Build` for `ze help ai --json` and MCP `ze_reference`, `internal/component/config/yang/cli/tree.go` `loadRPCParams` for `ze yang doc`, and `internal/component/config/schema/cli/main.go` `loadAPIRPCs` for `ze schema`). Updating `PublishedRPCs` to populate `RPCMeta.Input` from matching `-cmd` `ArgDefs` (converting `command.ArgDef` fields `Name`, `TypeName`, `ShortHelp`, `Description`, `Mandatory` into `LeafMeta`, falling back to `-api` `rpc.Input` only when `-cmd` declares zero leaves) preserves full input metadata on every interface when the 25 duplicate `-api` RPC `input` blocks are removed.
- `internal/le/doc/yangcontract/contract.go` (lines 185-190) flags any `ze:command` container whose wire method is not registered via `pluginserver.RegisterRPCs` (or local handlers) as `OrphanYANG`. Therefore, adding `ze-healthcheck-cmd.yang` and `ze-rpki-cmd.yang` requires builtin proxy forwarder files (`healthcheck/cmd_healthcheck.go` and `rpki/cmd_rpki.go`) matching `filter_irr/cmd_irr.go`.
- `ze-raw-cmd.yang` (rev 2026-09-05), `ze-update-cmd.yang` (rev 2026-09-05), and `ze-filter-irr-cmd.yang` already declare their typed leaves; their handlers simply were never switched off `validated.Tokens()` / `args[i]` onto `ValidatedArgs` / keyword-value reads.
- `announce` (2026-08-30 row) was already fixed on 2026-08-31 (`274e8e013`, `b47d1db6e`, `a0aa4e029`, tested by `test/plugin/api-announce-unicast.ci` and `test/ui/test-announce-forms-are-separate-commands.ci`); its open 2026-08-30 row only needs flipping to `fixed:` at closure.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD):**
- [ ] `internal/component/config/yang/command.go` - `BuildCommandTree`, `extractArgDefs`, `inheritArgDefs`, `PathToArgDefs` build command `ArgDefs` from `-cmd` containers, with no check refusing a leaf duplicated on the linked `-api` RPC `input`
- [ ] `internal/component/config/yang/rpc_publish.go` and `rpc.go` - `PublishedRPCs` publishes `ExtractRPCs` metadata (`RPCMeta.Input` from `-api.yang` only) to `internal/component/aihelp/aihelp.go` (`schemaRegistryFrom`, `Build`), `internal/component/config/yang/cli/tree.go` (`loadRPCParams`), and `internal/component/config/schema/cli/main.go` (`loadAPIRPCs`)
- [ ] `cmd/ze/hub/command_meta.go` - `buildParamMeta` (lines 338-368) prefers `PathToArgDefs(schema)` and falls back to `rpc.Input` only when a `-cmd` container declares no `ArgDefs`
- [ ] `internal/component/iface/yang/api_rpc_input_test.go` - `TestAPIRPCInputMatchesCommandTree` currently asserts that `ze-iface-api.yang` duplicates every `-cmd` leaf on RPC `input`
- [ ] `internal/component/command/argbind.go` and `arguments.go` - `ValidatedArgs` provides `Value`, `Keyed`, `Flag`, `Positional`, `PositionalTokens`, and `Tokens`; `WriteInvocation` (in `arguments.go`) writes anchored selectors bare and all other declared leaves as `<leaf> <value>` pairs
- [ ] `internal/component/bgp/plugins/cmd/peer/notify.go` and `yang/ze-peer-cmd.yang` - `handlePeerNotify` reads `code`, `subcode`, `hex`, and `text` (`ze:rest-of-line`) via `ValidatedArgs.Keyed` and `PositionalTokens`
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` `handleTeardown` (line 581) and `admin.go` `handlePeerDisable` (line 20) - both call `validated.Tokens()` by index; `teardown` lacks `leaf message` with `ze:rest-of-line` and `disable` `leaf message` lacks `ze:rest-of-line`
- [ ] `internal/component/bgp/plugins/cmd/raw/raw.go` `handleRaw` (line 58) and `yang/ze-raw-cmd.yang` - `ze-raw-cmd.yang` declares `encoding`, `data`, `type`, but `handleRaw` ignores `ValidatedArgs` accessors and hand-parses `validated.Tokens()` in `rawArguments`
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` `handleUpdate` (line 1214) and `yang/ze-update-cmd.yang` - `ze-update-cmd.yang` declares `encoding`, but `handleUpdate` reads `validated.Tokens()[0]` instead of `validated.Value("encoding")`
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go` `handleCommit` (line 160), `yang/ze-cli-commit-cmd.yang`, `internal/component/bgp/yang/ze-bgp-api.yang` (`rpc commit`, line 951), and callers (`commit/commit_test.go`, `commit/commit_report_test.go`, `commit/dispatch_test.go`, `internal/test/fixture/plugin_fixture_01_api.go`, `internal/test/fixture/plugin_fixture_04_cli.go`, `internal/component/config/yang/command_test.go`, `internal/component/config/schema/cli/main_test.go`) - `container commit` declares zero leaves under `ze-bgp:commit`, `ze-bgp-api.yang` declares `input { leaf name }` on `rpc commit`, and `handleCommit` parses `args[0]` action, `args[1]` name, and `args[2:]` route prefix by index while still accepting the deprecated `<name> <action>` order
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` (`forwardRibInject`, `forwardRibRPF`), `yang/ze-rib-cmd.yang`, and `internal/component/bgp/plugins/rib/rib_commands.go` (`injectRoute`, `rpfLookup`) - `inject` and `rpf` declare zero leaves in `ze-rib-cmd.yang`; `injectRoute` reads `args[0..2]` positionally and `rpfLookup` reads `args[0..1]` positionally
- [ ] `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang`, `cmd_irr.go`, and `command.go` (`showIRRPrefix`, `showIRRCheck`) - `ze-filter-irr-cmd.yang` declares `peer` and `prefix`, `cmd_irr.go` registers builtin proxy forwarders (`ze-bgp:show-irr-*`) passing raw `validated.Tokens()`, and `showIRRPrefix`/`showIRRCheck` read `args[0]`/`args[1]` by position and reject `peer <p> prefix <pfx>`
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` `handleShow`, `internal/component/bgp/plugins/rpki/rpki.go` (`aspaCommand`, `roaCommand`), and `internal/le/doc/yangcontract/contract.go` (lines 185-190) - plugin-registered commands with no `-cmd` YANG modules or builtin proxy forwarders (`OrphanYANG` requires `pluginserver.RegisterRPCs`), reading `args[0]` positionally

**Behavior to preserve:**
- Peer selectors stay positional (`request peer <selector> teardown ...`, `request peer <selector> disable ...`, `send bgp <selector> raw ...`, `send bgp <selector> update ...`).
- Existing valid CLI forms (`request peer <sel> teardown 2 "maintenance"`, `request peer <sel> disable going down`, `send bgp <sel> raw hex ... type open`, `send bgp <sel> update text ...`, canonical `request commit start <name>`, `request commit withdraw <name> route <prefix>`) keep working alongside the explicit keyword-value form emitted by web admin forms and MCP (`command.WriteInvocation`).
- `send bgp <sel> update <encoding> <route-expression>` continues passing the recursive update-text / wire-octet expression after `<encoding>` as opaque data to `ParseUpdateText`/`ParseUpdateWire`/`handleUpdateCursor`.
- Commands declared on RPC `input` alone (the 13 commands owned by `spec-declared-commands-without-leaves.md`) continue working via `buildParamMeta`'s and `PublishedRPCs`'s fallback until that sibling spec moves their leaves onto `-cmd` containers.
- `ze help ai --json`, MCP `ze_reference`, `ze yang doc`, and `ze schema` keep publishing complete RPC input leaves for all 25 commands whose duplicate `-api` RPC `input` blocks are deleted.

**Behavior to change:**
- Schema build refuses any leaf declared on both a `-cmd` container (including inherited ancestor leaves) and its linked `-api` RPC `input`; the 25 duplicate `-api` RPC `input` leaf sets are removed so `-cmd` is the single declaration, and `PublishedRPCs` populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` when `-cmd` declares leaves.
- `handleTeardown`, `handlePeerDisable`, `handleRaw`, and `handleUpdate` read declared leaves by name through `ValidatedArgs`, accepting both CLI and web/MCP keyword forms cleanly.
- `request commit` splits into 7 typed child `ze:command` containers (`list`, `start`, `end`, `eor`, `rollback`, `show`, `withdraw`, wire methods `ze-bgp:commit-*`, all pointing `ze:rpc "ze-bgp-api:commit"` with `rpc commit`'s duplicate `input` removed) reading `name` and `route` via `ValidatedArgs`, retiring the deprecated reversed `<name> <action>` grammar outright per `ai/rules/cli.md` and updating all test, fixture, snapshot, and doc callers.
- `request bgp rib inject`, `show bgp rib rpf`, `show bgp irr prefix`, `show bgp irr check`, `show bgp healthcheck`, `show bgp rpki aspa`, and `show bgp rpki roa` declare typed argument leaves on their `-cmd` containers, pair every new `-cmd` node with a builtin proxy forwarder (`healthcheck/cmd_healthcheck.go`, `rpki/cmd_rpki.go`, plus updated `cmd/rib/rib.go` and `filter_irr/cmd_irr.go`), and accept keyword-value form identically from CLI, web admin form, and MCP.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Command argument validation & binding | `internal/component/command/argbind.go` `ValidateArgs`, `ValidatedArgs.Value`, `Keyed`, `Flag`, `PositionalTokens` | Validates command tokens against `-cmd` `ArgDef`s and returns bound leaf values by name | Extends (adoption): moves owned compiled-in handlers and proxy forwarders onto these existing accessors; no new accessor needed |
| Command invocation writer | `internal/component/command/arguments.go` `WriteInvocation` | Writes anchored selectors bare and every other declared leaf as `<leaf> <value>` pairs for web admin forms, MCP, REST, and gRPC | Does not apply: already emits canonical keyword-value tokens from `-cmd` `ArgDef`s; owned handlers are updated to read them |
| Command tree & argument extraction | `internal/component/config/yang/command.go` `BuildCommandTree`, `extractArgDefs`, `inheritArgDefs`, `PathToArgDefs` | Builds `ArgDefs` from `-cmd` container leaves with `ze:help`, `description`, `ze:rest-of-line`, and `ze:keyword-only` | Extends: adds `CheckNoDuplicateCommandArgLeaves` refusing any linked `-api` RPC `input` that re-declares a leaf already declared on the `-cmd` container |
| RPC metadata publication | `internal/component/config/yang/rpc_publish.go` `PublishedRPCs` | Publishes linked `-api` RPC metadata (`RPCMeta`) to `aihelp` (`ze help ai --json`, MCP `ze_reference`), `yang/cli` (`ze yang doc`), and `schema/cli` (`ze schema`), reading `Input` from `-api.yang` alone | Extends: populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` (`PathToArgDefs`) whenever the pointing `-cmd` node declares leaves, falling back to `-api` `rpc.Input` only when `-cmd` declares none |
| Parameter schema builder | `cmd/ze/hub/command_meta.go` `buildParamMeta` | Already prefers `PathToArgDefs(schema)` over `rpc.Input`, falling back to `rpc.Input` only when `-cmd` declares no leaves | Does not apply: already derives REST/gRPC/MCP/web parameters from `-cmd` first; removing duplicate `-api` RPC `input` leaves requires no fallback change here |
| `ze:rest-of-line` reference handler | `internal/component/bgp/plugins/cmd/peer/notify.go` `handlePeerNotify` | Reads `code`/`subcode` via `Keyed`/`PositionalTokens` and captures trailing free prose via `ze:rest-of-line` `text` leaf | Does not apply: template pattern reused by `handleTeardown` and `handlePeerDisable` |
| Plugin command YANG proxy forwarders | `internal/component/bgp/plugins/filter_irr/cmd_irr.go` (`forwardShowIRR`, `forwardShowIRRPrefix`, `forwardShowIRRCheck`) and `internal/component/bgp/plugins/cmd/rib/rib.go` (`forwardRibProtocol`, `forwardRibClearOutExcludingOwner`) | Registers builtin `pluginserver.RPCRegistration` shims with `PluginCommand` forwarding validated arguments to plugin processes so `command-contract` (`internal/le/doc/yangcontract/contract.go`) pairs every `-cmd` node with a handler | Extends: updates `forwardRibInject`, `forwardRibRPF`, `forwardShowIRRPrefix`, and `forwardShowIRRCheck` to forward canonical keyword-value pairs from `ValidatedArgs`, and adds matching forwarders `healthcheck/cmd_healthcheck.go` (`ze-bgp:show-healthcheck`) and `rpki/cmd_rpki.go` (`ze-bgp:show-rpki-aspa`, `ze-bgp:show-rpki-roa`) |
| `announce`/`withdraw` split into typed `-cmd` forms | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` (`274e8e013`, `b47d1db6e`, `a0aa4e029`) | Splits one multi-form command into one `ze:command` per form (all pointing at one shared `-api` RPC via `ze:rpc`), each declaring its own typed leaves, tested by `api-announce-unicast.ci` and `test-announce-forms-are-separate-commands.ci` | Extends (precedent reused): applies the same per-action `ze:command` split to `request commit` (`ze-cli-commit-cmd.yang`), and retires the already-fixed 2026-08-30 `announce` journal row to `fixed:` at closure |
| `ze-iface-api` RPC input parity test | `internal/component/iface/yang/api_rpc_input_test.go` `TestAPIRPCInputMatchesCommandTree` | Asserts `ze-iface-api.yang` duplicates every `-cmd` leaf on RPC `input` | Replaces: updates assertion to verify zero duplicate RPC `input` leaves and that `PathToArgDefs` and `PublishedRPCs` provide every `ze-iface` parameter |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | Enforces a single `-cmd` leaf declaration per command argument, updates `PublishedRPCs` so `ze help ai --json`, MCP `ze_reference`, `ze yang doc`, and `ze schema` keep full input metadata from `-cmd`, and moves handlers to read those leaves by name across CLI, web, and MCP |
| `docs/architecture/config/yang-config-design.md` | 2. YANG Module Architecture | the // Design: page of `command.go` and `rpc_publish.go`; anchored to `command.go`, `rpc_publish.go`, `ze-bgp-api.yang`; a page this spec modifies | "A command node's `ze:rpc` statement names the rpc that documents its input and output, and the rpc is published under that node's wire method" | departs | Departs from documenting command inputs on `-api` RPCs: forbids duplicate `-api` RPC `input` leaves when `-cmd` declares leaves, teaches `PublishedRPCs` to populate `RPCMeta.Input` from matching `-cmd` `ArgDefs` (falling back to `-api` `rpc.Input` only for the 13 RPC-only commands), and updates section 2 of this page in the same change |
| `docs/architecture/api/commands.md` | Overview | the // Design: page of `peer.go`, `admin.go`, `raw.go`, `update_text.go`, `commit.go`, `rib.go`; anchored to `command.go`, `ze-peer-cmd.yang`, `rib_commands.go`, `healthcheck.go`, `rpki.go`; a page this spec modifies | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Keeps verb-first dispatch paths, enforces single `-cmd` argument declaration, moves owned handlers onto `ValidatedArgs` leaf-name accessors, and updates this page |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `internal/component/config/yang/command.go` | - | silent | Covers the five-stage plugin wire protocol; `ExecuteCommandInput.Args` remains `[]string` with no protocol wire change |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | anchored to `internal/component/config/yang/command.go` | - | silent | Covers on-demand route origination; untouched by command argument binding |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `internal/component/config/yang/command.go` | - | silent | Tab completion derives candidates from `-cmd` `ArgDefs`, so newly declared `-cmd` leaves automatically appear in completion |
| `docs/architecture/cli/command-verbs.md` | Everything is a selector the operator types | anchored to `command.go`, `ze-peer-cmd.yang`, `rib.go`, `filter_irr/command.go`; a page this spec modifies | "Every argument a handler reads MUST be declared in the schema." | agrees | Declares every owned handler argument leaf on its `-cmd` container, switches handlers onto `ValidatedArgs` leaf-name accessors, and retires T-5 (`peer raw` arguments) on this page once `handleRaw` reads its declared leaves |
| `ai/doors/config-yang.md` | Where it sits | the door covering `internal/component/config/yang/command.go`, `rpc_publish.go`, `command_test.go`, `rpc_test.go`, and `schema/cli/main_test.go` | "Each component embeds its `.yang` files and registers them from `init()`" | agrees | `CheckNoDuplicateCommandArgLeaves` and `PublishedRPCs`'s `-cmd` input synthesis operate over the generic loaded YANG schema in `internal/component/config/yang` with zero BGP imports |
| `ai/doors/forwarding.md` | Where it sits | the door covering `internal/component/iface/yang/api_rpc_input_test.go` and `ze-iface-api.yang` | "Components with a pluggable backend." | silent | Removing duplicate `ze-iface-api.yang` RPC `input` leaves preserves all interface command behavior because `buildParamMeta` and `PublishedRPCs` read `ze-iface-cmd.yang` leaves |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `internal/component/bgp/yang/ze-bgp-api.yang` | "The engine speaks the protocol and holds no policy." | silent | Removing duplicate RPC `input` leaves (including `rpc commit` `input`) from `ze-bgp-api.yang` leaves BGP protocol wire handling untouched while `PublishedRPCs` supplies input leaves from `-cmd` |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering `cmd/peer`, `cmd/raw`, `cmd/update`, `cmd/commit`, `cmd/rib`, `rib`, `filter_irr`, `healthcheck`, and `rpki` | "Every one can run in another process." | agrees | Every owned plugin-backed command (`rib`, `filter_irr`, `healthcheck`, `rpki`) continues exchanging keyword-value `Args []string` over the process-safe `ExecuteCommand` RPC through builtin proxy forwarders |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/ipc/yang/ze-system-api.yang` and `ze-plugin-api.yang` | "Everything depends on it." | silent | Removing duplicate RPC `input` leaves from `ze-system-api.yang` and `ze-plugin-api.yang` preserves every RPC's published input metadata through `PublishedRPCs` |
| `ai/doors/plugin-infra.md` | Where it sits | the door covering `internal/component/plugin/all/testdata/wire-methods.snapshot` | "The generated composition root: blank imports only" | silent | Regenerating the wire-methods snapshot reflects the split `ze-bgp:commit-*` and new `healthcheck`/`rpki` forwarder registrations with no plugin-infra contract change |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/plugins/cmd/peer/peer.go` | - | silent | Covers BGP FSM state transitions; `handleTeardown` sends the same NOTIFICATION bytes through `TeardownPeer` |
| `docs/architecture/api/architecture.md` | Command parameter values | anchored to `internal/component/config/yang/rpc_publish.go`, `internal/component/bgp/plugins/cmd/update/update_text.go`, and `internal/component/command/arguments.go` | "The shared API engine resolves structured parameters against one current command declaration before authorization and dispatch." | agrees | Enforces one `-cmd` argument declaration per command and migrates owned handlers to read keyword-value tokens written by `command.WriteInvocation` (`internal/component/command/arguments.go`) so REST, gRPC, MCP, web, and CLI resolve identically |
| `docs/architecture/api/wire-format.md` | Method Naming | anchored to `rpc_publish.go`, `ze-system-api.yang`, `ze-plugin-api.yang`, and `ze-bgp-api.yang` | "The method is declared once: a `ze:command` node declares it and points at the rpc that documents it with `ze:rpc`, and an rpc no node reaches declares its own with `ze:method`." | agrees | Keeps `ze:command` wire method declaration and `ze-bgp:` owner prefixes (`ze-bgp:commit-*`, `ze-bgp:show-healthcheck`, `ze-bgp:show-rpki-aspa`, `ze-bgp:show-rpki-roa`) intact while `PublishedRPCs` populates `RPCMeta.Input` from `-cmd` `ArgDefs` |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `internal/component/bgp/plugins/cmd/update/update_text.go` | "Unified syntax for API commands and config files to send/receive BGP UPDATE messages." | agrees | `handleUpdate` reads `encoding` via `ValidatedArgs` and preserves the exact update-text and wire-octet syntax for the trailing expression |
| `docs/architecture/bgp/as112-coordination.md` | The layering rule | anchored to `internal/component/bgp/plugins/cmd/update/update_text.go` | - | silent | Covers AS112 doctor checks; unaffected by `handleUpdate` reading `encoding` by leaf name |
| `docs/architecture/bgp/replay-cursor.md` | The decisions | anchored to `internal/component/bgp/plugins/cmd/update/update_text.go` | - | silent | Covers cursor replay state; `handleUpdate` continues dispatching `cursor` to `handleUpdateCursor` with the remaining tokens |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `update_text.go`; the // Design: page of `rib_commands.go` and `rpki.go` | "Pool with `Intern()` / `Get()`" | silent | Covers RIB pool and wire storage; changing `injectRoute`, `rpfLookup`, `aspaCommand`, and `roaCommand` to accept keyword-value arguments leaves storage untouched |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | anchored to `internal/component/bgp/plugins/cmd/update/update_text.go` | - | silent | Covers EVPN wire encoding; untouched by reading `encoding` via `ValidatedArgs` |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `internal/component/bgp/plugins/cmd/commit/commit.go` and `ze-bgp-api.yang`; a page this spec modifies | "Ze uses correlated, bidirectional RPC between the engine and plugins." | agrees | Keeps IPC transport framing unchanged while updating the Commits command table on this page from the retired `<name> <action>` spelling to canonical `request commit <action> <name>` |
| `docs/architecture/route-selection.md` | Overview | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` | - | silent | Covers BGP best-path selection; untouched by `injectRoute` and `rpfLookup` argument parsing |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` | - | silent | Covers UPDATE wire construction; untouched by `injectRoute` argument binding |
| `docs/architecture/bgp/filter-irr.md` | The decisions | the // Design: page of `internal/component/bgp/plugins/filter_irr/command.go` and `cmd_irr.go` | "The filter reads the `irr` plugin's store; it holds no IRR or PeeringDB client." | silent | Covers IRR filter store reading and UPDATE evaluation; updating `forwardShowIRRPrefix`/`forwardShowIRRCheck` and `showIRRPrefix`/`showIRRCheck` to read keyword-value `peer`/`prefix` arguments leaves filtering semantics unchanged |
| `docs/architecture/bgp/healthcheck-plugin.md` | The decisions | the // Design: page of `internal/component/bgp/plugins/healthcheck/healthcheck.go`; anchored to `healthcheck.go` | "One command path carries one answer-shape declaration." | agrees | `show bgp healthcheck` preserves its single `map` row-set answer shape with or without `name <probe>` while declaring optional `leaf name` in `ze-healthcheck-cmd.yang` paired with builtin proxy forwarder `healthcheck/cmd_healthcheck.go` |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `internal/component/bgp/plugins/rpki/rpki.go` | - | silent | Covers 4-byte AS_PATH reconstruction; untouched by `aspaCommand` and `roaCommand` argument parsing |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `internal/component/bgp/plugins/rpki/rpki.go` | - | silent | Covers plugin registration and startup phases; adding `-cmd` YANG modules and builtin proxy forwarders for `healthcheck` and `rpki` follows standard plugin registration |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/rib-inject-named-args.ci`, `test/plugin/cli-grammar-action-first.ci`, `internal/test/fixture/plugin_fixture_01_api.go`, `plugin_fixture_04_cli.go`, and `internal/le/site/testdata/published-command-paths.txt` | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | Places the new functional wiring test under `test/plugin/rib-inject-named-args.ci` and updates commit fixture callers and `cli-grammar-action-first.ci` to the canonical `request commit <action> <name>` grammar |
| `ai/doors/commands-cli.md` | Boundaries | a page this spec modifies | "Every RPC has a YANG node, and every node a handler." | agrees | Declares every owned command argument leaf once on `-cmd`, pairs every new `-cmd` node (`healthcheck`, `rpki`) with a builtin proxy forwarder registered via `pluginserver.RegisterRPCs` so `command-contract` reports zero `OrphanYANG`, moves handlers onto `ValidatedArgs`, and updates this door's Traps section |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Declares only true command arguments on `-cmd` containers; never adds scope, presentation, or format arguments to commands |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not modify pipe operation overrides or answer shapes |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not alter pipe semantics |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Preserves all existing engine operation message annotations and bus authorization |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not touch event delivery queues |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` config formatting |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Does not touch bus participant registration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not touch `send > to` routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not touch `receive > from` routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not touch bus allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not touch bus filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Does not touch bus arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus defaults display |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not touch FlowSpec next-hop emission |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Does not touch offline external plugin query mode |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator CLI text, web admin form (`command.WriteInvocation` in `internal/component/command/arguments.go`), MCP tool invocation (`dispatchGenerated`), and REST/gRPC command dispatch.
- Reference/schema metadata consumers: `internal/component/aihelp/aihelp.go` (`ze help ai --json` and MCP `ze_reference`), `internal/component/config/yang/cli/tree.go` (`ze yang doc`), and `internal/component/config/schema/cli/main.go` (`ze schema`), all reading `yang.PublishedRPCs`.
- Build-time schema validation (`internal/component/config/yang` `CheckNoDuplicateCommandArgLeaves` / `TestArgDeclaredOnce`).

### Transformation Path
1. `-cmd` YANG modules declare every command argument leaf once (types, ranges, enums, `ze:help`, `description`, `ze:rest-of-line`, `ze:keyword-only`). `CheckNoDuplicateCommandArgLeaves` refuses any linked `-api` RPC `input` that duplicates a `-cmd` leaf.
2. `BuildCommandTree` extracts `ArgDefs` into `command.Node`; `buildParamMeta` publishes those exact `ArgDefs` to REST, gRPC, MCP tools, and web admin forms; and `PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`) populates `RPCMeta.Input` from the pointing `-cmd` node's `ArgDefs` (converting `command.ArgDef` to `LeafMeta`, falling back to `-api` `rpc.Input` only when `-cmd` declares zero leaves) so `aihelp`, `yang/cli`, and `schema/cli` publish the exact same inputs from the single `-cmd` declaration.
3. At dispatch, `command.ValidateArgs` validates tokens against `ArgDefs` and populates `ValidatedArgs` keyed by leaf name.
4. Compiled-in handlers (`handleTeardown`, `handlePeerDisable`, `handleRaw`, `handleUpdate`, and the 7 split commit handlers `handleCommitList`, `handleCommitStart`, `handleCommitEnd`, `handleCommitEOR`, `handleCommitRollback`, `handleCommitShow`, `handleCommitWithdraw` under `ze-bgp:commit-*`) read bound values via `ValidatedArgs.Value`/`Keyed`/`PositionalTokens`.
5. Builtin proxy forwarders (`cmd/rib/rib.go` `forwardRibInject`/`forwardRibRPF`, `filter_irr/cmd_irr.go` `forwardShowIRRPrefix`/`forwardShowIRRCheck`, `healthcheck/cmd_healthcheck.go` `forwardShowHealthcheck`, `rpki/cmd_rpki.go` `forwardShowRPKIASPA`/`forwardShowRPKIROA`) register via `pluginserver.RegisterRPCs` (satisfying `./le doc yang-contract command-contract`), read bound leaves from `ValidatedArgs`, and forward canonical keyword-value `[]string` arguments via `ForwardToPlugin` to the plugin handlers (`injectRoute`, `rpfLookup`, `showIRRPrefix`, `showIRRCheck`, `handleShow`, `aspaCommand`, `roaCommand`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG `-cmd` schema → command dispatcher / web / MCP / REST / gRPC | `PathToArgDefs` → `command.ValidateArgs` and `buildParamMeta` | No |
| YANG `-cmd` schema → `ze help ai --json` / MCP `ze_reference` / `ze yang doc` / `ze schema` | `PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`) populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` | No |
| Daemon dispatcher → plugin process (`bgp-rib`, `bgp-filter-irr`, `bgp-healthcheck`, `bgp-rpki`) | Builtin proxy forwarders (`ForwardToPlugin`) carrying validated keyword-value argument tokens across `ExecuteCommand` RPC | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/`, `docs/`; terms `ValidatedArgs`, `WriteInvocation`, `buildParamMeta`, `PublishedRPCs`, `ExtractRPCs`, `ze-bgp:rib-inject`, `ze-bgp:rib-rpf`, `ze-bgp:commit`, `rpc commit`, `request commit`, `ze-bgp:peer-teardown`, `ze-bgp:peer-disable`, `ze-bgp:peer-raw`, `ze-bgp:peer-update`; positive controls: `argbind.go`, `arguments.go:50`, `rpc_publish.go:57`, `command_meta.go:338`, `rib.go:561`, `cmd_irr.go:111`, `peer.go:581`, `admin.go:20`, `raw.go:58`, `commit.go:160`, `dispatch_test.go:46`, `plugin_fixture_01_api.go:84`, `plugin_fixture_04_cli.go:89`. No `pkg/` RPC envelope or network protocol format changes: `ExecuteCommandInput.Args` remains `[]string`; the wire changes are (a) command-line argument tokens between callers and owned handlers carrying declared leaf names, and (b) splitting internal wire method `ze-bgp:commit` into `ze-bgp:commit-{list,start,end,eor,rollback,show,withdraw}` while retiring deprecated `request commit <name> <action>`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Command line to owned BGP handlers | writer | CLI (`internal/component/cli`), web admin form (`internal/component/command/arguments.go` `WriteInvocation`), MCP (`internal/component/mcp/tools.go` `dispatchGenerated`), REST/gRPC (`cmd/ze/hub/command_meta.go` `buildParamMeta`) | `docs/architecture/api/commands.md`, "YANG-Typed Command Arguments"; `docs/architecture/api/architecture.md`, "Command parameter values"; `arguments.go:50`; `command_meta.go:338-368` | Preserved for existing peer-selector and declared forms; web/MCP/REST/gRPC emit keyword-value pairs matching `-cmd` leaf names (`peer <ip> family <fam> prefix <pfx>`, `cease-subcode <n> message <text>`, `name <probe>`, `customer <asn>`, `prefix <pfx>`) |
| Command line to owned BGP handlers | reader | `internal/component/command/argbind.go` `ValidateArgs`; `peer.go` `handleTeardown`; `admin.go` `handlePeerDisable`; `raw.go` `handleRaw`; `update_text.go` `handleUpdate`; `cmd/rib/rib.go` `forwardRibInject`/`forwardRibRPF`; `rib_commands.go` `injectRoute`/`rpfLookup`; `filter_irr/cmd_irr.go` `forwardShowIRRPrefix`/`forwardShowIRRCheck`; `filter_irr/command.go` `showIRRPrefix`/`showIRRCheck`; `healthcheck/cmd_healthcheck.go` `forwardShowHealthcheck`; `healthcheck.go` `handleShow`; `rpki/cmd_rpki.go` `forwardShowRPKIASPA`/`forwardShowRPKIROA`; `rpki.go` `aspaCommand`/`roaCommand` | `argbind.go`, `peer.go:581`, `admin.go:20`, `raw.go:58`, `rib.go:561`, `rib_commands.go:236`, `cmd_irr.go:111`, `filter_irr/command.go:103`, `healthcheck.go`, `rpki.go:1686,1909` | Migrated to read bound values by declared leaf name (`ValidatedArgs.Value`/`Keyed`/`PositionalTokens` and canonical keyword-value forwarded tokens), so CLI, web admin form, and MCP all resolve identically |
| `request commit` command path & wire method (`ze-bgp:commit` → `ze-bgp:commit-*`) | writer | `internal/component/bgp/plugins/cmd/commit/commit_test.go`, `commit_report_test.go`, `dispatch_test.go` (`TestDispatchBGPCommitList`, `TestDispatchBGPCommitStartEnd`); `internal/test/fixture/plugin_fixture_01_api.go` (`plugin01APICommitLifecycle`, `plugin01APICommitWorkflow`); `internal/test/fixture/plugin_fixture_04_cli.go` (`grammarActionFirst04`, `commitEndPerPeerReport04`); `test/plugin/cli-grammar-action-first.ci`; `internal/component/config/yang/command_test.go` (`TestSimpleCmdModules`, `TestCommitNoEditShortcut`); `internal/component/config/schema/cli/main_test.go` (`TestBuildSchemaRegistryRPCs`) | `dispatch_test.go:29,46,51`, `plugin_fixture_01_api.go:84,97`, `plugin_fixture_04_cli.go:89-111,641-652`, `command_test.go:331,371`, `schema/cli/main_test.go:697` | Migrated outright from deprecated `request commit <name> <action>` to canonical `request commit <action> <name>` (supporting both `<name>` and `name <name>`, plus `route <prefix>` for `withdraw`) per `ai/rules/cli.md`; `cli-grammar-action-first.ci` / `grammarActionFirst04` updated to assert canonical positional and keyword forms succeed and retired `<name> <action>` is refused; schema/snapshot assertions updated from `ze-bgp:commit` to `ze-bgp:commit-*` |
| `request commit` command path & wire method (`ze-bgp:commit` → `ze-bgp:commit-*`) | reader | `internal/component/bgp/plugins/cmd/commit/yang/ze-cli-commit-cmd.yang`; `internal/component/bgp/yang/ze-bgp-api.yang` (`rpc commit`); `internal/component/bgp/plugins/cmd/commit/commit.go` (`handleCommitList`, `handleCommitStart`, `handleCommitEnd`, `handleCommitEOR`, `handleCommitRollback`, `handleCommitShow`, `handleCommitWithdraw`) | `ze-cli-commit-cmd.yang:11-20`, `ze-bgp-api.yang:951-964`, `commit.go:122-253` | Split into 7 typed `ze:command` sub-containers (`ze-bgp:commit-{list,start,end,eor,rollback,show,withdraw}`), each pointing `ze:rpc "ze-bgp-api:commit"` with `rpc commit` `input` removed; handlers read `name` and `route` via `ValidatedArgs.Value` and retire the deprecated `<name> <action>` branch |
| Published RPC input metadata (`PublishedRPCs`) | writer | `internal/component/config/yang/rpc_publish.go` `PublishedRPCs` (synthesizing `RPCMeta.Input` from `PathToArgDefs` when `-cmd` declares leaves) | `rpc_publish.go:57-128`, `command.go:151` | Extended so every command linked by `ze:rpc` publishes its `-cmd` argument leaves in `RPCMeta.Input` even after duplicate `-api` RPC `input` blocks are removed |
| Published RPC input metadata (`PublishedRPCs`) | reader | `internal/component/aihelp/aihelp.go` (`schemaRegistryFrom`, `Build`); `internal/component/config/yang/cli/tree.go` (`loadRPCParams`); `internal/component/config/schema/cli/main.go` (`loadAPIRPCs`) | `aihelp.go:223-233,362-374`, `yang/cli/tree.go:393-412`, `schema/cli/main.go:389-397` | Preserved unchanged: all three continue calling `yang.PublishedRPCs` and receive complete `Input` leaves from `-cmd` |

### Integration Points
- `internal/component/config/yang/command.go` (`CheckNoDuplicateCommandArgLeaves`) - build/schema guard against duplicate `-cmd`/`-api` leaf declarations.
- `internal/component/config/yang/rpc_publish.go` (`PublishedRPCs`) - populates `RPCMeta.Input` from `-cmd` `ArgDefs` for `aihelp`, `yang/cli`, and `schema/cli`.
- `cmd/ze/hub/command_meta.go` (`buildParamMeta`) - publishes `-cmd` `ArgDefs` to REST, gRPC, MCP tools, and web admin forms.
- `internal/component/bgp/plugins/cmd/{peer,raw,update,commit,rib}` and `internal/component/bgp/plugins/{rib,filter_irr,healthcheck,rpki}` - owned `-cmd` schemas, builtin proxy forwarders, and handlers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Every owned command passes through `ValidateArgs` against its `-cmd` YANG leaves before handler or proxy forwarder execution |
| No unintended coupling (components stay isolated) | No | Each BGP plugin owns its own `-cmd.yang`, proxy forwarder, and handler; `internal/component/config/yang` stays generic over all `-cmd`/`-api` modules |
| No duplicated functionality (extends existing, does not recreate) | No | Reuses `ValidatedArgs` accessors, `ze:rest-of-line`, `buildParamMeta`'s `-cmd`-first lookup, `filter_irr/cmd_irr.go` proxy forwarder pattern, and `PublishedRPCs`'s single publication path; deletes the 25 duplicate `-api` RPC `input` leaf blocks |
| Zero-copy preserved where applicable (refs, not copies) | No | Command control plane only; no BGP data-plane hot path touched |
| Registration over hardcoding, outbound | No | Argument metadata is discovered from registered `-cmd` YANG modules across all surfaces (`buildParamMeta` and `PublishedRPCs`) |
| Registration over hardcoding, inbound | No | Searched `api_rpc_input_test.go`, `rpc_publish.go`, `raw.go`, `peer.go`, `admin.go`, `commit.go`, `rib_commands.go`, `cmd_irr.go`; removes duplicate `-api` input declarations and hand-indexed token parsing |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Once `PublishedRPCs` populates `RPCMeta.Input` from `-cmd` `ArgDefs`, no other caller beyond `buildParamMeta` and `PublishedRPCs` reads `ExtractRPCs` `Input` directly for commands that declare `-cmd` leaves | Verified by reading `cmd/ze/hub/command_meta.go:338-368`, `rpc_publish.go:57-128`, `aihelp.go:223-233,362-374`, `yang/cli/tree.go:393-412`, `schema/cli/main.go:389-397` | Another caller reads `ExtractRPCs` directly and loses input leaves after `-api` duplicates are deleted | `git grep ExtractRPCs\|PublishedRPCs` + `TestArgDeclaredOnce` + `TestPublishedRPCsPopulatesInputFromCmdLeaves` + `./le doc yang-contract command-contract` | unvalidated |
| A-2 | Every existing `.ci`, fixture, and unit test invoking `request commit`, `request bgp rib inject`, `show bgp rib rpf`, `show bgp healthcheck`, `show bgp rpki aspa`, `show bgp rpki roa`, and `show bgp irr check` has been enumerated or accepts the canonical keyword-value grammar cleanly | Verified by `git grep` across `internal/`, `test/`, `docs/` and reading `ai/rules/cli.md` and `internal/component/command/argbind.go` | An unenumerated test sends a retired positional order | `./le cli dispatch check`, `./le cli grammar`, `./le test functional plugin`, `./le test functional ui` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Splitting `request commit` into 7 `ze-bgp:commit-*` sub-containers and retiring deprecated `request commit <name> <action>` breaks callers or snapshots that still assert `ze-bgp:commit` or reversed argument order | `./le cli dispatch check`, `TestDispatchBGPCommitStartEnd`, `TestBuildSchemaRegistryRPCs`, `plugin_fixture_01_api`, `cli-grammar-action-first.ci` | All 8 enumerated callers (`dispatch_test.go`, `commit_test.go`, `commit_report_test.go`, `plugin_fixture_01_api.go`, `plugin_fixture_04_cli.go`, `cli-grammar-action-first.ci`, `command_test.go`, `schema/cli/main_test.go`), both snapshots (`wire-methods.snapshot`, `published-command-paths.txt`), and `ipc_protocol.md` are updated together in the same commit |
| R-2 | Adding `-cmd.yang` modules and proxy forwarders for `healthcheck` and `rpki` changes `TestYANGSchemaProviders` / `TestRegisteredWireMethods` snapshots or triggers `OrphanYANG` if any `ze:command` lacks a `pluginserver.RegisterRPCs` forwarder | `internal/component/plugin/all/all_test.go` snapshot diff or `./le doc yang-contract command-contract` | Pair every new `-cmd` `ze:command` with a `pluginserver.RegisterRPCs` proxy forwarder (`healthcheck/cmd_healthcheck.go`, `rpki/cmd_rpki.go`), run `./le yang glue update` and `./le repo update`, and update snapshots cleanly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Web admin forms, MCP tools, `ze help ai --json`, `ze yang doc`, or `ze schema` lose or mispass command arguments; CLI parsing rejects valid operator forms; `ze-iface`, `ze-bgp` commit, or plugin-backed BGP command dispatch fails |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/release/fixes/spec-declared-commands-without-leaves.md` (13 commands whose leaves sit on RPC input alone), `plan/post-release/features/spec-plugin-command-named-arguments.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A test schema declaring the same leaf on both a `-cmd` container and its linked `-api` RPC `input` | → | `CheckNoDuplicateCommandArgLeaves` (`internal/component/config/yang/command.go`) | `TestArgDeclaredOnce` |
| A `-cmd` container declaring leaves linked via `ze:rpc` to an `-api` RPC with no duplicate `input` block | → | `PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`) populating `RPCMeta.Input` for `aihelp`, `yang/cli`, and `schema/cli` | `TestPublishedRPCsPopulatesInputFromCmdLeaves` |
| `request bgp rib inject` invoked from both CLI text and web admin form (`command.WriteInvocation`) | → | `ValidateArgs` → `forwardRibInject` → `injectRoute` reading by leaf name | `test/plugin/rib-inject-named-args.ci` |
| `request peer <sel> teardown cease-subcode 2 message "maintenance window"` and `request peer <sel> disable message "going down"` | → | `ValidateArgs` (`ze:rest-of-line`) → `handleTeardown` / `handlePeerDisable` reading by leaf name | `TestTeardownAndDisableKeywordAndPositionalForms` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Any `-cmd` container with `ze:rpc` whose linked `-api` RPC `input` declares a leaf also declared on (or inherited by) the `-cmd` container | `CheckNoDuplicateCommandArgLeaves` (`TestArgDeclaredOnce`) fails naming both the `-cmd` path and the `-api` RPC leaf; all 25 existing duplicate `-api` RPC `input` leaf sets are removed; `buildParamMeta` serves every REST/gRPC/MCP/web parameter from `-cmd`; and `PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`) populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` so `ze help ai --json`, MCP `ze_reference`, `ze yang doc`, and `ze schema` retain every input leaf | The 13 commands whose leaves sit on `-api` RPC `input` alone (zero leaves on `-cmd`) remain served by `buildParamMeta`'s and `PublishedRPCs`'s fallback until `spec-declared-commands-without-leaves.md` moves them to `-cmd` |
| AC-2 | `request peer <sel> teardown` and `request peer <sel> disable` invoked with positional CLI form, keyword form (`cease-subcode 2 message ...`, `disable message ...`), or free text containing leaf-name words (`"message code subcode"`) | Both handlers read `cease-subcode` and `message` (`ze:rest-of-line`) via `ValidatedArgs.Keyed`/`PositionalTokens`; keyword form never sends the literal word `"message"` in the RFC 9003 Shutdown Communication, and words inside free text are never mistaken for keywords | The peer selector stays positional (`ai/rules/cli.md`) |
| AC-3 | `send bgp <sel> raw`, `send bgp <sel> update`, and `request commit <action>` invoked via CLI text, web admin form (`command.WriteInvocation`), or MCP | `handleRaw` reads `encoding`, `data`, `type` via `ValidatedArgs`; `handleUpdate` reads `encoding` via `ValidatedArgs` and passes the trailing route expression as data to `ParseUpdateText`/`ParseUpdateWire`/`handleUpdateCursor`; `request commit` splits into 7 typed child `ze:command` containers (`ze-bgp:commit-{list,start,end,eor,rollback,show,withdraw}`) pointing `ze:rpc "ze-bgp-api:commit"` (with `rpc commit` `input` removed) and reading `name`/`route` via `ValidatedArgs.Value`, retiring deprecated `<name> <action>` and updating all enumerated callers | The recursive route expression after `update <encoding>` stays opaque trailing data owned by `ParseUpdateText`/`ParseUpdateWire`, not flat YANG leaves |
| AC-4 | `request bgp rib inject`, `show bgp rib rpf`, `show bgp irr prefix`, `show bgp irr check`, `show bgp healthcheck`, `show bgp rpki aspa`, and `show bgp rpki roa` invoked via CLI text, web admin form, and MCP | Each command declares its typed argument leaves on its `-cmd` YANG container (`peer`/`family`/`prefix` + optional attrs for `inject`; `family`/`address` for `rpf`; `peer` for `irr prefix`; `peer`/`prefix` for `irr check`; `name` for `healthcheck`; `customer` for `rpki aspa`; `prefix` for `rpki roa`), pairs every new `-cmd` node with a builtin proxy forwarder (`healthcheck/cmd_healthcheck.go`, `rpki/cmd_rpki.go`, plus updated `cmd/rib/rib.go` and `filter_irr/cmd_irr.go`), and reads the keyword-value form identically across CLI, web admin form, and MCP with one consistent answer shape | External third-party plugin SDK command wire typing stays in `plan/post-release/features/spec-plugin-command-named-arguments.md`; this spec handles in-tree commands over the existing `Args []string` RPC |
| AC-5 | `./le cli grammar`, `./le cli dispatch check`, `./le doc yang-contract command-contract`, and all 6 owned Piece D journal rows | All three gates exit 0 (with zero `OrphanYANG` for `healthcheck`/`rpki` and zero `OrphanRPCs` for `ze-bgp:commit-*`); none of the 6 owned journal rows reproduce, and all 6 rows (including the already-fixed 2026-08-30 `announce` row) turn `fixed:` at closure with their verifying tests | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Injects a synthetic BGP route from both the CLI and the web admin form | CLI / web form (`command.WriteInvocation`) → `ValidateArgs` → `forwardRibInject` → `injectRoute` by leaf name | `test/plugin/rib-inject-named-args.ci` |
| 2 | Tears down or disables a peer from MCP/web using keyword form (`teardown cease-subcode 2 message "code maintenance"`) | MCP/web → `ValidateArgs` (`ze:rest-of-line`) → `handleTeardown` / `handlePeerDisable` → RFC 9003 Shutdown Communication carrying `"code maintenance"` without literal `"message"` | `TestTeardownAndDisableKeywordAndPositionalForms` |
| 3 | Queries `ze help ai --json` / MCP `ze_reference` / `ze yang doc` / `ze schema` for a command whose duplicate `-api` RPC `input` block was removed | `PublishedRPCs` → `PathToArgDefs` → `RPCMeta.Input` populated from `-cmd` leaves → `aihelp` / `yang/cli` / `schema/cli` | `TestPublishedRPCsPopulatesInputFromCmdLeaves` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestArgDeclaredOnce` | `internal/component/config/yang/command_test.go` | AC-1 (synthetic duplicate `-cmd`/`-api` leaf refused; full shipped schema clean) | |
| `TestPublishedRPCsPopulatesInputFromCmdLeaves` | `internal/component/config/yang/rpc_test.go` | AC-1 (`PublishedRPCs` populates `RPCMeta.Input` from `-cmd` `ArgDefs` when `-api` `input` is absent, and falls back to `-api` `input` when `-cmd` declares zero leaves) | |
| `TestAPIRPCInputMatchesCommandTree` | `internal/component/iface/yang/api_rpc_input_test.go` | AC-1 (`ze-iface-api` has zero duplicate RPC input leaves; `-cmd` and `PublishedRPCs` supply every parameter) | |
| `TestTeardownAndDisableKeywordAndPositionalForms` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | AC-2 (positional, keyword, and leaf-name-inside-message cases for `teardown` and `disable`) | |
| `TestRawUpdateCommitNamedArgs`, `TestDispatchBGPCommitList`, `TestDispatchBGPCommitStartEnd` | `internal/component/bgp/plugins/cmd/raw/raw_test.go`, `internal/component/bgp/plugins/cmd/update/update_text_test.go`, `internal/component/bgp/plugins/cmd/commit/commit_test.go`, `commit_report_test.go`, `dispatch_test.go` | AC-3 (`ValidatedArgs` keyword and positional forms for `raw`, `update`, and the 7 split `request commit` handlers; retired `<name> <action>` refused) | |
| `TestSimpleCmdModules`, `TestCommitNoEditShortcut`, `TestBuildSchemaRegistryRPCs` | `internal/component/config/yang/command_test.go`, `internal/component/config/schema/cli/main_test.go` | AC-1, AC-3 (all 7 `request/commit/*` containers map to `ze-bgp:commit-*` without `ze:edit-shortcut` and appear in schema registry with `-cmd` inputs) | |
| `TestPluginBGPCommandsNamedArgs`, `TestHealthcheckAndRPKIProxyForwarders` | `internal/component/bgp/plugins/rib/rib_commands_test.go`, `internal/component/bgp/plugins/filter_irr/command_test.go`, `internal/component/bgp/plugins/healthcheck/healthcheck_test.go`, `internal/component/bgp/plugins/rpki/rpki_test.go` | AC-4 (keyword-value invocation and builtin proxy forwarders for `rib inject`, `rib rpf`, `irr prefix`, `irr check`, `healthcheck`, `rpki aspa`, `rpki roa`) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `teardown` `cease-subcode` | 0..255 (`uint8`) | 0, 255 | -1 | 256 |
| `rpki aspa` `customer` | 1..4294967295 (`zt:asn-notated`) | 1, 4294967295 | 0 | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-inject-named-args` | `test/plugin/rib-inject-named-args.ci` | `request bgp rib inject` invoked with keyword-value form (matching web admin form/MCP) and verified in `show bgp rib` | |
| `cli-grammar-action-first` | `test/plugin/cli-grammar-action-first.ci` | Canonical `request commit <action> <name>` (both positional and `name <name>` keyword form) succeeds while retired `<name> <action>` is refused | |

### Interop Tests (Scope: protocol)
N-A: Command argument schema and handler binding only; no BGP protocol wire change.

## Files to Modify
- `internal/component/config/yang/command.go` - add `CheckNoDuplicateCommandArgLeaves` refusing any leaf declared on both a `-cmd` container and its linked `-api` RPC `input`
- `internal/component/config/yang/rpc_publish.go` - update `PublishedRPCs` to populate `RPCMeta.Input` from matching `-cmd` `ArgDefs` (`PathToArgDefs`) whenever `-cmd` declares leaves, preserving input metadata for `aihelp`, `yang/cli`, and `schema/cli`
- `internal/component/config/yang/rpc_test.go` - add `TestPublishedRPCsPopulatesInputFromCmdLeaves`
- `internal/component/config/yang/command_test.go` - add `TestArgDeclaredOnce`; update `TestSimpleCmdModules` and `TestCommitNoEditShortcut` for the 7 `request/commit/*` (`ze-bgp:commit-*`) nodes
- `internal/component/config/schema/cli/main_test.go` - update `TestBuildSchemaRegistryRPCs` from `ze-bgp:commit` to `ze-bgp:commit-*`
- `internal/component/iface/yang/api_rpc_input_test.go` - update `TestAPIRPCInputMatchesCommandTree` to assert zero duplicate RPC `input` leaves while `-cmd` and `PublishedRPCs` cover every `ze-iface` parameter
- `internal/component/iface/yang/ze-iface-api.yang` - remove duplicate RPC `input` leaves already declared on `ze-iface-cmd.yang`
- `internal/core/ipc/yang/ze-system-api.yang` - remove duplicate RPC `input` leaves already declared on `-cmd` containers
- `internal/core/ipc/yang/ze-plugin-api.yang` - remove duplicate RPC `input` leaves already declared on `-cmd` containers
- `internal/component/bgp/yang/ze-bgp-api.yang` - remove duplicate RPC `input` leaves (including `rpc commit` `input { leaf name }`) already declared on `-cmd` containers (plus any remaining `-api.yang` files among the 25 duplicate commands)
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - add `leaf message` with `ze:rest-of-line` on `teardown` and add `ze:rest-of-line` to `disable` `leaf message`
- `internal/component/bgp/plugins/cmd/peer/peer.go` - update `handleTeardown` to read `cease-subcode` and `message` via `ValidatedArgs.Keyed`/`PositionalTokens`
- `internal/component/bgp/plugins/cmd/peer/admin.go` - update `handlePeerDisable` to read `message` via `ValidatedArgs.Keyed`/`PositionalTokens`
- `internal/component/bgp/plugins/cmd/raw/raw.go` - update `handleRaw` to read `encoding`, `data`, and `type` via `ValidatedArgs`
- `internal/component/bgp/plugins/cmd/update/update_text.go` - update `handleUpdate` to read `encoding` via `ValidatedArgs` and pass remaining tokens to `ParseUpdateText`/`ParseUpdateWire`/`handleUpdateCursor`
- `internal/component/bgp/plugins/cmd/commit/yang/ze-cli-commit-cmd.yang` - split `container commit` into 7 typed `ze:command` sub-containers (`list`, `start`, `end`, `eor`, `rollback`, `show`, `withdraw`, wire methods `ze-bgp:commit-*`, pointing `ze:rpc "ze-bgp-api:commit"`) declaring mandatory `name` and `route` leaves
- `internal/component/bgp/plugins/cmd/commit/commit.go` - register the 7 `ze-bgp:commit-*` handlers reading `name` and `route` via `ValidatedArgs.Value` and retire the deprecated `<name> <action>` branch
- `internal/component/bgp/plugins/cmd/commit/commit_test.go` - update unit tests to drive the 7 typed handlers with `ValidatedArgs` (positional and keyword forms) and assert retired `<name> <action>` fails
- `internal/component/bgp/plugins/cmd/commit/commit_report_test.go` - update helper calls from single `handleCommit` to typed `handleCommitStart`/`handleCommitWithdraw`/`handleCommitEnd`/`handleCommitEOR`
- `internal/component/bgp/plugins/cmd/commit/dispatch_test.go` - update `TestDispatchBGPCommitStartEnd` from deprecated `<name> <action>` to canonical `request commit start test-dispatch` / `request commit end test-dispatch` plus keyword form
- `internal/test/fixture/plugin_fixture_01_api.go` - update `plugin01APICommitLifecycle` and `plugin01APICommitWorkflow` from deprecated `<name> <action>` to canonical `request commit <action> <name>`
- `internal/test/fixture/plugin_fixture_04_cli.go` - update `grammarActionFirst04` to assert canonical positional and `name <name>` forms succeed and retired `<name> <action>` is refused
- `test/plugin/cli-grammar-action-first.ci` - update validation comment to match retired reversed grammar and canonical positional/keyword forms
- `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` - declare typed leaves on `request bgp rib inject` (`peer`, `family`, `prefix`, optional attributes) and `show bgp rib rpf` (`family`, `address`)
- `internal/component/bgp/plugins/cmd/rib/rib.go` - update `forwardRibInject` and `forwardRibRPF` to read bound leaves from `ValidatedArgs` and forward canonical keyword-value pairs
- `internal/component/bgp/plugins/rib/rib_commands.go` - update `injectRoute` and `rpfLookup` to accept declared keyword-value form
- `internal/component/bgp/plugins/filter_irr/cmd_irr.go` - update `forwardShowIRRPrefix` and `forwardShowIRRCheck` to read bound `peer`/`prefix` leaves from `ValidatedArgs` and forward canonical keyword-value pairs
- `internal/component/bgp/plugins/filter_irr/command.go` - update `showIRRPrefix` and `showIRRCheck` to read keyword-value form (`peer`, `prefix`) matching `ze-filter-irr-cmd.yang`
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - update `handleShow` to accept declared keyword-value `name <probe>`
- `internal/component/bgp/plugins/rpki/rpki.go` - update `aspaCommand` and `roaCommand` to accept declared keyword-value `customer <asn>` and `prefix <prefix>`
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - regenerate snapshot for `ze-bgp:commit-*`, `ze-bgp:show-healthcheck`, `ze-bgp:show-rpki-aspa`, and `ze-bgp:show-rpki-roa`
- `internal/le/site/testdata/published-command-paths.txt` - update published paths for `request commit {end,eor,list,rollback,show,start,withdraw}`, `show bgp healthcheck`, `show bgp rpki aspa`, and `show bgp rpki roa`
- `docs/architecture/config/yang-config-design.md` - update section 2 (`ze:rpc` table row and "API Schemas") to reflect that `-cmd` container leaves are the single input declaration and `PublishedRPCs` populates `RPCMeta.Input` from `-cmd`
- `docs/architecture/api/commands.md` - document single `-cmd` argument declaration rule and leaf-name reading for owned commands
- `docs/architecture/cli/command-verbs.md` - retire T-5 (`peer raw` arguments) once `handleRaw` reads its declared leaves via `ValidatedArgs`
- `docs/architecture/api/ipc_protocol.md` - update the Commits table (lines 1023-1028) from retired `<name> <action>` to canonical `request commit <action> <name>`
- `docs/guide/command-reference.md` - update documented spellings for `request bgp rib inject`, `show bgp rib rpf`, `show bgp healthcheck`, `show bgp rpki aspa`, `show bgp rpki roa`, and `request commit`
- `ai/doors/commands-cli.md` - retire the resolved double-declaration trap note once landed

## Files to Create
- `internal/component/bgp/plugins/healthcheck/yang/ze-healthcheck-cmd.yang` - declare `show bgp healthcheck` (`ze:command "ze-bgp:show-healthcheck"`) container and optional `leaf name` (`ze:keyword-only`)
- `internal/component/bgp/plugins/healthcheck/cmd_healthcheck.go` - register builtin proxy forwarder `ze-bgp:show-healthcheck` (`PluginCommand: "show bgp healthcheck"`) reading `name` via `ValidatedArgs.Value` and forwarding to `bgp-healthcheck` so `command-contract` reports no `OrphanYANG`
- `internal/component/bgp/plugins/rpki/yang/ze-rpki-cmd.yang` - declare `show bgp rpki aspa` (`ze:command "ze-bgp:show-rpki-aspa"`, optional `leaf customer`) and `show bgp rpki roa` (`ze:command "ze-bgp:show-rpki-roa"`, optional `leaf prefix`) containers with `ze:keyword-only`
- `internal/component/bgp/plugins/rpki/cmd_rpki.go` - register builtin proxy forwarders `ze-bgp:show-rpki-aspa` and `ze-bgp:show-rpki-roa` reading `customer`/`prefix` via `ValidatedArgs.Value` and forwarding to `bgp-rpki` so `command-contract` reports no `OrphanYANG`
- `test/plugin/rib-inject-named-args.ci` - functional test invoking `request bgp rib inject` with keyword-value arguments

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `-cmd` leaves added/refined; 25 duplicate `-api` RPC `input` leaf sets removed; `PublishedRPCs` synthesizes `RPCMeta.Input` from `-cmd` |
| YANG validation constraints | Yes | Typed leaves (`uint8`, `zt:ip-address`, `zt:ip-prefix`, `zt:address-family`, `zt:asn-notated`, `ze:rest-of-line`, `ze:keyword-only`) |
| YANG custom validators | No | None |
| CLI commands/flags | Yes | Owned BGP commands gain typed keyword-value argument leaves; `request commit` splits into 7 typed sub-actions |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` verified over every changed `-cmd` module |
| Editor autocomplete | Yes | Derived automatically from `-cmd` `ArgDefs` |
| Functional test for new RPC/API | Yes | `test/plugin/rib-inject-named-args.ci`, updated `test/plugin/cli-grammar-action-first.ci` |
| Pipe completeness | No | Answer shapes preserved |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | None |
| Doctor code per problem class | No | None |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fixes argument declaration and binding on existing commands |
| 2 | Config syntax changed? | No | Command schemas only |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (keyword-value argument spellings and `request commit` actions) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (single `-cmd` declaration; `ValidatedArgs` leaf-name reads), `docs/architecture/config/yang-config-design.md` (section 2 `ze:rpc` input contract updated) |
| 5 | Plugin added/changed? | No | Existing plugins gain `-cmd` leaves and proxy forwarders only |
| 6 | Has a user guide page? | No | Covered by `docs/guide/command-reference.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | `ExecuteCommandInput.Args` remains `[]string` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 9003 Shutdown Communication encoding unchanged (fixed keyword leakage) |
| 10 | Test infrastructure changed? | Yes | `internal/component/config/yang/command_test.go` (`TestArgDeclaredOnce`), `rpc_test.go`, `api_rpc_input_test.go`, `plugin_fixture_01_api.go`, `plugin_fixture_04_cli.go` |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, `docs/architecture/api/commands.md`, `docs/architecture/cli/command-verbs.md`, `docs/architecture/api/ipc_protocol.md`, `ai/doors/commands-cli.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `request commit` splits `ze-bgp:commit` into 7 `ze-bgp:commit-*` commands, and `healthcheck`/`rpki` add proxy forwarders; regenerate YANG glue, `wire-methods.snapshot`, and `published-command-paths.txt` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md`. Updated: `docs/architecture/config/yang-config-design.md` (section 2 updated for `-cmd` input authority and `PublishedRPCs`), `docs/architecture/api/commands.md` (updated for single `-cmd` leaf declaration and leaf-name handler reads), `docs/architecture/cli/command-verbs.md` (T-5 `peer raw` arguments retired), `docs/architecture/api/ipc_protocol.md` (Commits table updated to canonical `request commit <action> <name>`). Unaffected: `docs/architecture/api/wire-format.md` (wire method naming and `ze-bgp:` owner prefixes unchanged), `docs/architecture/bgp/filter-irr.md` (`showIRRPrefix`/`showIRRCheck` argument binding does not change IRR filtering or store reads), `docs/architecture/bgp/healthcheck-plugin.md` (`show bgp healthcheck` keeps its single `map` row-set answer shape with optional `name <probe>`), `docs/architecture/plugin/rib-storage-design.md` (`injectRoute`, `rpfLookup`, `aspaCommand`, and `roaCommand` change argument parsing only, not RIB/RPKI storage) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md` |

## Design presentation

### What changes

Command arguments have one YANG declaration—the `-cmd` container leaf—and schema
build refuses any leaf duplicated on a linked `-api` RPC `input`. To keep
reference and schema surfaces complete when the 25 duplicate `-api` RPC `input`
blocks are deleted, `PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`)
populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` whenever `-cmd` declares
leaves, preserving full input metadata for `ze help ai --json`, MCP `ze_reference`,
`ze yang doc`, and `ze schema`. All owned compiled-in BGP command handlers
(`teardown`, `disable`, `raw`, `update`, and `request commit` split into 7 typed
`ze-bgp:commit-*` actions retiring deprecated `<name> <action>`) move onto
`ValidatedArgs` leaf-name accessors (following `handlePeerNotify`), and the owned
plugin-backed BGP commands (`rib inject`, `rib rpf`, `irr prefix`, `irr check`,
`healthcheck`, `rpki aspa`, `rpki roa`) declare typed `-cmd` leaves paired with
builtin proxy forwarders (`healthcheck/cmd_healthcheck.go`, `rpki/cmd_rpki.go`,
plus `cmd/rib/rib.go` and `filter_irr/cmd_irr.go`) and accept keyword-value
invocations identically from CLI, web admin forms, and MCP.

### Before and after

1. **Declaring an argument leaf on both a `-cmd` container and its `-api` RPC `input`.**
   Before: 25 commands declare the leaf twice with nothing comparing descriptions, types, or `mandatory` flags, while `PublishedRPCs` reads `RPCMeta.Input` solely from `-api.yang`.
   After: `CheckNoDuplicateCommandArgLeaves` (`TestArgDeclaredOnce`) refuses any duplicate at schema build; all 25 duplicate `-api` RPC `input` leaf blocks are removed; `buildParamMeta` serves REST/gRPC/MCP/web parameters from `-cmd`, and `PublishedRPCs` populates `RPCMeta.Input` from `-cmd` `ArgDefs` so `ze help ai --json`, MCP `ze_reference`, `ze yang doc`, and `ze schema` retain every input leaf.
2. **Invoking `request peer <sel> teardown` or `disable` from web admin or MCP.**
   Before: `teardown cease-subcode 2 message "going down"` fails or puts the literal word `message` onto the RFC 9003 wire because `handleTeardown` and `handlePeerDisable` index `validated.Tokens()`.
   After: Both handlers read `cease-subcode` and `message` (`ze:rest-of-line`) by leaf name, so positional CLI text and web/MCP keyword forms produce identical wire Shutdown Communications.
3. **Invoking `request commit` from CLI, web admin, MCP, or plugins.**
   Before: `container commit` declares zero leaves under a single `ze-bgp:commit` handler that parses `args[0..2]` by index and accepts deprecated `<name> <action>`, while `ze-bgp-api.yang` `rpc commit` declares `input { leaf name }` alone.
   After: `request commit` splits into 7 typed `ze:command` sub-containers (`ze-bgp:commit-{list,start,end,eor,rollback,show,withdraw}`) pointing `ze:rpc "ze-bgp-api:commit"` (with `rpc commit` `input` removed), handlers read `name` and `route` via `ValidatedArgs.Value`, deprecated `<name> <action>` is retired outright, and all unit/fixture/CI callers and snapshots are updated.
4. **Invoking `request bgp rib inject`, `show bgp rib rpf`, `show bgp irr check`, `show bgp healthcheck`, `show bgp rpki aspa`, or `show bgp rpki roa` from web admin or MCP.**
   Before: Web admin forms render no input fields (or post keyword-value pairs that positional `args[0]` readers reject), and `healthcheck`/`rpki` have no `-cmd` YANG modules or builtin proxy forwarders.
   After: Every command declares typed leaves in `-cmd` YANG paired with a builtin proxy forwarder (`command-contract` green with zero `OrphanYANG`) and accepts keyword-value form identically across CLI, web admin form, and MCP.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Make `-api` RPC `input` the single declaration instead of `-cmd` containers | Only 70 of 405 command containers link a `-api` RPC, and `buildParamMeta` already prefers `PathToArgDefs(schema)` (`-cmd` leaves) over `rpc.Input`; `-cmd` is already the authoritative command tree |
| Keep single `ze-bgp:commit` with a typed `action` enum leaf and optional `name`/`route` leaves | `list` takes no `name`, `start`/`end`/`eor`/`rollback`/`show` require `name` alone, and `withdraw` requires both `name` and `route`; a flat single command cannot make `name` or `route` mandatory per action or match the `announce`/`withdraw` split precedent in `ze-cli-announce-cmd.yang`, and keeping deprecated `<name> <action>` violates `ai/rules/cli.md` ("Ze is unreleased, so an unreleased grammar MUST be replaced outright rather than deprecated") |
| Change the external plugin SDK `ExecuteCommand` RPC to pass a typed leaf map in this spec | That is a published `pkg/plugin/rpc` wire change with external-plugin migration impact; it stays scoped to `plan/post-release/features/spec-plugin-command-named-arguments.md` |
| Force recursive `send bgp <sel> update text ...` route expressions into flat YANG leaves | Recursive attribute/NLRI/modifier grammars cannot be expressed as a flat leaf list; `ze-update-cmd.yang` rev 2026-09-05 already establishes that `encoding` is the declared YANG enum while the trailing expression stays data for `ParseUpdateText` |

### Documents departed from

1. **`docs/architecture/config/yang-config-design.md`, "2. YANG Module Architecture" (`Extensions: ze-extensions.yang` `ze:rpc` row and `API Schemas: ze-bgp-api.yang and ze-*-cmd.yang`):**
   - **Before:** States that a command node's `ze:rpc` statement "names the rpc that documents the command's input and output", which licensed declaring command input leaves on `-api.yang` RPC `input` blocks alongside `-cmd.yang` container leaves (producing 25 duplicated argument declarations).
   - **After:** `-cmd.yang` container leaves are the single authoritative declaration for command input arguments; `CheckNoDuplicateCommandArgLeaves` refuses any leaf declared on both a `-cmd` container and its linked `-api` RPC `input`, and `PublishedRPCs` populates `RPCMeta.Input` from matching `-cmd` `ArgDefs` (falling back to `-api` `rpc.Input` only when `-cmd` declares zero leaves). `ze:rpc` names the RPC that documents the command's output and description (and fallback inputs only for the 13 legacy RPC-only commands until `spec-declared-commands-without-leaves.md` migrates them). Section 2 of `docs/architecture/config/yang-config-design.md` is updated in the same commit.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write `TestArgDeclaredOnce` (failing against the 25 duplicate commands), `TestPublishedRPCsPopulatesInputFromCmdLeaves` (failing before `PublishedRPCs` reads `-cmd` leaves), `TestTeardownAndDisableKeywordAndPositionalForms` (failing on keyword form), and `test/plugin/rib-inject-named-args.ci` (failing).
2. **Phase: Single declaration & publication (AC-1)** -- add `CheckNoDuplicateCommandArgLeaves` in `internal/component/config/yang/command.go`, update `PublishedRPCs` in `internal/component/config/yang/rpc_publish.go` to populate `RPCMeta.Input` from matching `-cmd` `ArgDefs`, remove the 25 duplicate `-api` RPC `input` leaf blocks (`ze-iface-api.yang`, `ze-system-api.yang`, `ze-plugin-api.yang`, `ze-bgp-api.yang`, and remaining duplicates), update `api_rpc_input_test.go`, and verify `buildParamMeta`, `aihelp`, `yang/cli`, and `schema/cli` tests pass.
3. **Phase: Compiled-in BGP handlers & `request commit` split (AC-2, AC-3)** -- add `ze:rest-of-line` `message` leaves to `ze-peer-cmd.yang` (`teardown`, `disable`); switch `handleTeardown`, `handlePeerDisable`, `handleRaw`, and `handleUpdate` onto `ValidatedArgs` leaf-name accessors; split `ze-cli-commit-cmd.yang` and `commit.go` into the 7 `ze-bgp:commit-*` commands retiring deprecated `<name> <action>`, and update all commit unit tests (`commit_test.go`, `commit_report_test.go`, `dispatch_test.go`), schema tests (`command_test.go`, `schema/cli/main_test.go`), and fixtures (`plugin_fixture_01_api.go`, `plugin_fixture_04_cli.go`, `cli-grammar-action-first.ci`).
4. **Phase: Plugin-backed BGP commands & proxy forwarders (AC-4)** -- add `-cmd` argument leaves for `rib inject`, `rib rpf`, `healthcheck`, `rpki aspa`, `rpki roa`; create builtin proxy forwarders `healthcheck/cmd_healthcheck.go` and `rpki/cmd_rpki.go`; update `forwardRibInject`, `forwardRibRPF`, `forwardShowIRRPrefix`, `forwardShowIRRCheck`, `injectRoute`, `rpfLookup`, `showIRRPrefix`, `showIRRCheck`, `handleShow`, `aspaCommand`, and `roaCommand` to accept keyword-value form; run `./le yang glue update` and `./le repo update`, and update `wire-methods.snapshot` and `published-command-paths.txt`.
5. **Phase: Docs and journal rows (AC-5)** -- update `docs/architecture/config/yang-config-design.md`, `docs/architecture/api/commands.md`, `docs/architecture/cli/command-verbs.md` (retire T-5), `docs/architecture/api/ipc_protocol.md` (Commits table), `docs/guide/command-reference.md`, and `ai/doors/commands-cli.md`, verify all 6 owned journal rows no longer reproduce, and flip all 6 rows to `fixed:` at closure.

## Review Gate

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-5 has code, unit tests, and functional verification at concrete file + symbol |
| Correctness | No owned handler reads command arguments by positional index; peer selector stays positional; `update` trailing route expression stays opaque data; `PublishedRPCs` preserves input leaves on `aihelp`, `yang/cli`, and `schema/cli` |
| Data flow | `PathToArgDefs` is the sole parameter producer for commands declaring `-cmd` leaves across `buildParamMeta` and `PublishedRPCs`; zero duplicate `-cmd`/`-api` argument leaves |
| Rule: no-layering | Duplicate `-api` RPC `input` leaf definitions deleted rather than kept synced by hand; deprecated `<name> <action>` commit grammar retired outright |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Single argument declaration gate & `PublishedRPCs` synthesis | `TestArgDeclaredOnce`, `TestPublishedRPCsPopulatesInputFromCmdLeaves` (`./le test unit pkg path ./internal/component/config/yang/`) |
| Owned compiled-in handlers on `ValidatedArgs` & 7-way `request commit` split | Unit tests in `cmd/peer`, `cmd/raw`, `cmd/update`, `cmd/commit`, `config/schema/cli` + `test/plugin/cli-grammar-action-first.ci` |
| Owned plugin-backed commands with typed `-cmd` leaves & proxy forwarders | Unit tests in `rib`, `filter_irr`, `healthcheck`, `rpki` + `test/plugin/rib-inject-named-args.ci` |
| CLI grammar and contract | `./le cli grammar`, `./le cli dispatch check`, `./le doc yang-contract command-contract` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Every newly declared `-cmd` argument leaf carries bounded YANG types, ranges, or patterns so `ValidateArgs` refuses malformed inputs before handlers run |
| Message truncation / RFC 9003 safety | `teardown` and `disable` preserve RFC 9003 Shutdown Communication length/UTF-8 handling without leaking keyword tokens onto the wire |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Duplicate leaf still reported by `TestArgDeclaredOnce` | Remove the duplicate leaf from the `-api.yang` RPC `input` and confirm `-cmd` holds its full type, `ze:help`, and `description` |
| `OrphanYANG` reported by `./le doc yang-contract command-contract` | Verify the new `-cmd` `ze:command` wire method is registered via `pluginserver.RegisterRPCs` in its plugin's `cmd_*.go` proxy forwarder |
| Existing `.ci` test fails on updated command spelling | Check whether the test uses a retired positional order forbidden by `ai/rules/cli.md` and update the sender or preserve declared peer-selector syntax |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `command-takes-an-untyped-positional-value.md` | 2026-08-24 | Five `show bgp` commands (`healthcheck`, `rpki aspa`, `rpki roa`, `rib rpf`, `irr check`) read an untyped positional |
| `command-takes-an-untyped-positional-value.md` | 2026-08-30 | `announce` handler reads args beyond the declared `selector` (already fixed 2026-08-31 by `274e8e013`/`b47d1db6e`/`a0aa4e029`; flip `open:` row to `fixed:` at closure) |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `peer raw`, `peer update` and `request commit` read undeclared positionals |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `request bgp rib inject` reads `args[0..2]` |
| `command-takes-an-untyped-positional-value.md` | 2026-10-21 | `peer teardown`/`disable` read `validated.Tokens()` by position |
| `argument-leaf-declared-on-container-and-rpc-input.md` | 2026-09-15 | 25 commands declare one leaf on the `-cmd` container and the rpc input |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `-cmd` container leaf is the single declaration; refuse duplicate `-api` RPC `input` leaves and synthesize `RPCMeta.Input` in `PublishedRPCs` from `-cmd` `ArgDefs` | Derive `-cmd` from `-api` RPC `input`, or leave `PublishedRPCs` reading `-api` alone | Only 70 of 405 commands have a `-api` RPC, `buildParamMeta` already prefers `PathToArgDefs`, and updating `PublishedRPCs` keeps `aihelp`, `yang/cli`, and `schema/cli` complete without duplicate YANG declarations |
| Split `request commit` into 7 typed `ze-bgp:commit-*` sub-containers pointing `ze:rpc "ze-bgp-api:commit"` and retire deprecated `<name> <action>` | Keep single `ze-bgp:commit` with optional leaves and deprecated branch | Follows the `announce`/`withdraw` split precedent, enforces per-action mandatory leaves (`name`, `route`), and complies with `ai/rules/cli.md` |
| Pair new `healthcheck` and `rpki` `-cmd` modules with builtin proxy forwarders (`cmd_healthcheck.go`, `cmd_rpki.go`) | Declare `-cmd` YANG without `pluginserver.RegisterRPCs` shims | `./le doc yang-contract command-contract` flags any unbacked `ze:command` container as `OrphanYANG`; follows `filter_irr/cmd_irr.go` |
| Reuse `ValidatedArgs` accessors and `ze:rest-of-line` (`handlePeerNotify` pattern) | Add per-argument generated Go structs | `ValidatedArgs` already exists and every route validates into it |
| Keep external plugin SDK `ExecuteCommandInput.Args []string` unchanged in this release fix | Redesign external plugin command RPC wire format | Published `pkg/` contract change belongs in `plan/post-release/features/spec-plugin-command-named-arguments.md` |

## Known Limitations
- The 13 commands declaring arguments on `-api` RPC `input` alone (zero leaves on `-cmd`) remain owned by `plan/release/fixes/spec-declared-commands-without-leaves.md`.
- Typed named-argument maps over the external plugin `ExecuteCommand` RPC remain post-release in `plan/post-release/features/spec-plugin-command-named-arguments.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" for every changed command

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run by the main thread: `./le test unit pkg path ./internal/component/config/yang/`, `./le test unit pkg path ./internal/component/config/schema/cli/`, `./le test unit pkg path ./internal/component/aihelp/`, `./le test unit pkg path ./internal/component/iface/yang/`, `./le test unit pkg path ./internal/component/bgp/plugins/cmd/peer/`, `./le test unit pkg path ./internal/component/bgp/plugins/cmd/raw/`, `./le test unit pkg path ./internal/component/bgp/plugins/cmd/update/`, `./le test unit pkg path ./internal/component/bgp/plugins/cmd/commit/`, `./le test unit pkg path ./internal/component/bgp/plugins/cmd/rib/`, `./le test unit pkg path ./internal/component/bgp/plugins/rib/`, `./le test unit pkg path ./internal/component/bgp/plugins/filter_irr/`, `./le test unit pkg path ./internal/component/bgp/plugins/healthcheck/`, `./le test unit pkg path ./internal/component/bgp/plugins/rpki/`, `./le test functional plugin`, `./le test functional ui`, `./le cli grammar`, `./le cli dispatch check`, `./le doc yang-contract command-contract`, and `./le doc check all`
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests pass (`test/plugin/rib-inject-named-args.ci`, `test/plugin/cli-grammar-action-first.ci`)
- [ ] Interop tests N-A: no protocol wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + 6 owned journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md` only
