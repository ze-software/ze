# Spec: a command's declared pipe operations reach its handler from every caller, and the chain is honored on every surface

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped daemon behaviour. A plugin's `dispatch-command`, MCP, REST and gRPC refuse a piped command that SSH and `ze cli` answer. Every rendering surface splits a quoted pipe value as a chain. `foldFilters` rewrites the operator's line into a different command. All three are wrong answers on shipped operator surfaces (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - (spec-plugin-dispatch-pipe-chain-reaches-the-handler, closed in `2616478e64` and `9321d263aa`, landed the quote-aware `scanCommand` this spec reuses and the refusal this spec replaces) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas's requirement, verbatim:

> command should be able to define optional modificator like "| count" and I
> documented with you how it worked ... The command should not be rewritten the
> engine should be able to handle "| count"

> If we have some commands which are common like count and which can be
> reused, we should have generic count which can be related to the command. And
> if not suitable, the command should be able to override and describe its own
> command.

> We should have a command which allows to ask for any command what are the
> modifiers available to that command.

and "override is a behavior".

The design this spec builds, and writes into the pages that document it
(`docs/architecture/api/commands.md`, `docs/architecture/cli/command-verbs.md`,
`docs/features/formatting.md`, phase 8):

- A command path names the subject and stops; scope, presentation and format are pipe operations after it.
- A common operation such as `| count` is generic and applies to every command's answer, from every caller.
- A command may declare optional pipe operations of its own. The engine hands a declared operation to the command's handler for every caller, as a structured list. The command line is never rewritten to carry one as an argument.
- A declaration with a generic's name overrides that operation's behaviour for the command, with its own description, and keeps the generic's meaning and answer shape. An operation that means something else takes a name of its own.
- Every operator command answers the same on every command surface (MCP, REST, gRPC, SSH, web) as the local `ze` command.
- For any command, `show command help <name>` lists every pipe operation the command answers to; `ze help command --json` publishes the same list, and the `|` completer offers it.

Three defects stand between the code and that design:

1. **The line is rewritten.** `foldFilters` (`internal/component/command/pipe.go`)
   turns a command-owned pipe operation into command words and dispatches the
   result: `show bgp rib | received | count` reaches the RIB plugin as the
   command `show bgp rib received count`. The handler, accounting and the
   authorizer all see a line the operator never typed.
2. **The data callers refuse the chain.** A plugin's `dispatch-command`, MCP
   `ze_execute` and every derived MCP tool, REST execute and gRPC `Execute` hand
   the whole line to `Dispatcher.Dispatch`, which refuses any pipe outside
   quotes (`pipeChainRefusal`, added this week). Its fix line is the rewritten
   command (`FoldPipeChain`), so the refusal teaches the rewritten spelling.
3. **The split is not quote-aware.** `ParsePipe` cuts at the first pipe
   character with `strings.Cut` and `parsePipeOps` splits the rest at every
   pipe character, while `scanCommand`
   (`internal/component/plugin/server/command.go`) honors double quotes. A
   quoted value holding a pipe character is torn into a chain on SSH, the web
   CLI and `ze cli`.

The three share one mechanism, the parse of a line into a subject and a chain,
so one spec owns them. The list of operations a command answers to is derived
once and read by help, the published catalog and completion.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - one command registry behind every interface; a plugin can run in another process; `pkg/` is a published contract
  → Constraint: a command is declared once and reaches every interface; a surface adds nothing of its own for one command
  → Constraint: what a handler receives crosses the process boundary as a value: an in-process-only carrier breaks the forked `bgp-rib`
- [ ] `ai/doors/commands-cli.md` - the command model, `execMiddleware`, the pipe operators as a contract scripts read
  → Constraint: a handler answers data and output goes through `ApplyPipes`; a script reading `| json` relies on `pipe_catalog.go`
  → Constraint: help, usage and completion are derived from the owner's registration, never written in a second place
- [ ] `ai/doors/api-surfaces.md` - REST, gRPC and MCP are thin adapters over `serverDispatcher`
  → Constraint: the adapters own no command; REST turns JSON parameters into command tokens (`api.BuildCommand`) before anything parses them
- [ ] `ai/doors/plugin-infra.md` - `plugin/server` `Dispatcher`, the process protocol, the SDK
  → Constraint: every feature works over both transports; a field in `rpc` is wire, not Go; the SDK stays generic
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` `serverDispatcher` and `buildAPIEngine`
  → Constraint: one dispatcher closure per surface, the same `pluginserver.Dispatcher` behind each
- [ ] `ai/doors/bgp-plugins.md` - `bgp-rib` runs internal or forked; its two declarations must agree
  → Constraint: `./le plugin declarations check` already compares the `Commands` and `Pipes` lists, so an operation declared through `Pipes` is judged with no new list
- [ ] `ai/doors/system-plugins.md` - `internal/plugins/help` and `internal/plugins/meta` are command providers whose catalog and command help publish the pipe operations
  → Constraint: a plugin owns its whole surface, help included, so each help producer is converted by its owner
- [ ] `ai/doors/access-security.md` - TACACS+ START/STOP accounting comes from one hook in `Dispatcher.Dispatch`
  → Constraint: the accounting record is the operator's command log on the TACACS+ server; it must record what was entered
- [ ] `ai/doors/le-tooling.md` - fixtures, `.ci`, interop checks and the catalog tools live here
  → Constraint: a compiled Go fixture returns an error when an assertion fails; a test change that could lower what it proves goes to `ze-test-judge`
- [ ] `docs/architecture/api/commands.md` - "The surface splits the pipe chain; the dispatcher refuses one", "Pipe aliases", "Where an alias resolves, and where it does not", the per-command registries table, the `show command help` row
  → Decision: the page states the surface split, the refusal and the fold as the design; phase 8 replaces that text with this spec's design
- [ ] `docs/architecture/cli/command-verbs.md` - operator classes (`ClassGlobal`, `ClassData`, `ClassStream`) and owned filters
  → Decision: the class decides which commands owe an operator; the applicable generics are derived from it and from the command's answer shape
- [ ] `docs/features/formatting.md` - "Command-specific filters"
  → Decision: "a command filter stops it being produced" stays true and is the property to preserve; "folded into the command itself" is replaced by the design
- [ ] `plan/records/audit-presentation-pipes.md` §1.1 to §1.3, §1.5, §1.8, §4.2, §4.4, §4.5, and `plan/records/audit-command-pipe-vs-subcommand.md` - background on the four axes and on which leaves are views
  → Constraint: a command-owned operation runs before the answer exists and can compute (§1.3); this spec keeps that and changes its carrier

**Key insights:**
- A command-owned operation is right to run in the handler, before the answer exists; it is the rewrite of the line that goes.
- `scanCommand` already answers the tokens and the offset of the first pipe outside quotes. It is the one quote rule; the command package needs it too.
- The web CLI terminal already is the split, dispatch, format pattern; the data callers need the same chain applied by the engine.
- The RIB's YANG declares its pipeline words as a command argument (`ze-rib-api.yang` `rpc show`, `leaf-list args`).
- A plugin already declares pipe names at Stage 1 (`Pipes`, `rpc.PipeDecl`, recorded by `registerPluginPipes`). That declaration is the one a plugin's operations extend.
- The per-command join of the operator catalog with the declared shape already exists, twice: `operatorsFor` (`internal/plugins/help/help_command.go`) and the operator loop in `internal/le/cli/catalog/catalog.go`. `show command help` does not call it, and the completer offers every catalog operator to every command and drops an override's description.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - `ParsePipe` cuts at the first `|`; `parsePipeOps` splits at every `|` and maps names through `knownPipeOps`; `parsePipeChain` expands aliases and binds address fields; `foldFilters` looks up the command's `PipeFilter` set by longest prefix, removes every owned operation from the chain (a generic-named `match`, `count`, `first`, `last` only before a format operator) and appends `<name> <arg>` to the command string, `Leading` ones first; `FoldPipeChain` runs the same fold for the dispatcher's refusal; the `ProcessPipes*` wrappers call `ColumnsForCommand`, `foldFilters` and `validatePipesForSurface` and return the rewritten command plus a formatter over `ApplyPipes`; `applyCount` answers `{"count":N}` over the rows `rowsForOperator` finds; `applyMatch` keeps the rows whose value text holds the pattern, case-insensitive
- [ ] `internal/component/command/pipe_catalog.go` - `PipeOperator{Name, Kind, Class, Arg, Repeat, Description, NeedsAddressField, LocalOnly}` with `Applies(shape)`; `PipeClass` is `ClassGlobal`, `ClassData`, `ClassStream`; `PipeArgKind` is `ArgNone`, `ArgOptional`, `ArgText`, `ArgCount`, `ArgFields`, `ArgPath`; `knownPipeOps` derives from the catalog
- [ ] `internal/component/command/pipe_filter.go` - `PipeFilter{Name, Description, TakesArg, Leading}`; `RegisterPipeFilters` declares a set per command path in `pipeFilterRegistry`, a longest-prefix `declarationRegistry`; an empty set is a barrier; `PipeFiltersForCommand` and `filterSuggestions` read it
- [ ] `internal/component/command/alias.go` - `RegisterAliases` and `RegisterPluginAliases` (by owner, withdrawn by `UnregisterPluginAliases`) record pipe aliases; `aliasBarriers` derives a plugin's barriers from its own command list
- [ ] `internal/component/command/completer.go` - `completePipe` offers every entry of `PipeOperators` whatever the command's shape, then the command's aliases and filters (`pipeExtras`), skipping an extra a catalog operator already offered, so an override shows the generic's description
- [ ] `internal/plugins/meta/cmd/help.go` - `commandHelp` (`show command help <name>`) answers `pipe-filters` (`PipeFiltersForCommand`) and `pipe-aliases` (`AliasesForCommand`, with expansion), beside the other declaration keys; no generic operator and no override status
- [ ] `internal/plugins/help/help_command.go` - `operatorsFor` joins `command.PipeOperatorCatalog()` with the declared shape and address fields, and answers each operator's `available` (`always`, `with-rows`, `when-streaming`) and `local-only`; `commandEntry` publishes `operators`, `pipes` and `pipe-aliases` for `ze help command --json`
- [ ] `internal/le/cli/catalog/catalog.go` - a second copy of the same operator join; `Entry` carries `pipes`, `operators`, `pipe-aliases`
- [ ] `internal/le/cli/catalog/render.go` - `renderPipes` renders `operators` by availability
- [ ] `internal/le/cli/list/report.go`, `internal/le/doc/yangcontract/command_surfaces.go`, `internal/le/site/catalog.go` - read the published `pipe-aliases`, and the last two the `operators` availability
- [ ] `internal/test/fixture/plugin_fixture_11_alias.go`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `internal/test/fixture/ui_fixture_pipe_review_entry_contracts.go` - assert `pipe-aliases`, `pipe-filters`, `pipes` and the operator availability sets
- [ ] `internal/component/plugin/server/startup.go` - `registerPluginPipes` turns each Stage 1 `rpc.PipeDecl` into a `command.PluginAlias` and calls `RegisterPluginAliases`; `validatePipeDecls` checks the plugin owns the paths it names
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` declares on `show bgp rib`: `received`, `advertised` (Leading), `peer`, `family`, `prefix`, `path`, `community`, `match`, `first`, `last` (TakesArg), `count`, `histogram`, `graph`; on `show bgp rib best` the same without scope plus `reason`; empty sets on the scalar and meta commands. `forwardRibRoutes` forwards `validated.Tokens()` and `ctx.PeerSelector()` through `ForwardToPlugin`
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `showPipeline` calls `parsePipelineArgs(args)`; with no terminal it streams `sdk.Records{Key: "routes"}`; `count` answers `{"count": N}` from the iterator; `(*matchFilter).matches` reads the prefix, the peer, the family and the route attributes
- [ ] `internal/component/bgp/plugins/rib/rib_topology.go` - `graphAnswerKey` documents `show bgp rib ... graph`, the argument form
- [ ] `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` - `rpc show` declares `leaf peer` and `leaf-list args`
- [ ] `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` - `container rib` names the pipe operators in its description
- [ ] `internal/component/plugin/server/command.go` - `Dispatch` starts accounting on the raw input, calls `scanCommand`, and when `pipeAt != noPipe` answers `pipeChainRefusal`; `scanCommand` with an odd quote count silently treats the rest of the line as one quoted token; `routeToProcess` sends `rpc.ExecuteCommandInput{Command, Args, Peer}`; `CommandContext` has no pipe field
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `opDispatchCommand` reaches `Dispatch` with the plugin's line
- [ ] `internal/component/plugin/dispatch.go` - `CallerIdentity`; `CommandDispatcher.JSON` flattens with `ResponseJSON`; `CommandDispatcher.Answer` keeps a row generator for SSH
- [ ] `pkg/plugin/rpc/types.go` - `ExecuteCommandInput{Serial, Command, Args, Peer}`; `PipeDecl{Command, Name, Description, Expansion}` declares an alias and nothing else
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `ExecuteCommandHandler` takes serial, command, args and peer; 46 non-test files and 8 test files register `OnExecuteCommand`
- [ ] `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/web/handler_tools.go`, `internal/component/web/page_tools.go`, `internal/component/cli/model_mode.go`, `internal/component/cli/client/main.go`, `internal/test/cli/cmd_engine_steps.go` - split at the edge with the `ProcessPipes*` wrappers and dispatch the rewritten command part
- [ ] `internal/component/mcp/tools.go` - `ze_execute` and `run` call `s.dispatch.JSON` with the whole line
- [ ] `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`, `internal/component/api/engine.go` - `handleExecute` and `Execute` reach `APIEngine.Execute`, which judges read-only and authorization on the whole string
- [ ] `internal/component/api/requests.go` - `BuildCommand` refuses whitespace in a parameter and accepts `|` or `"`
- [ ] `cmd/ze/hub/main_servers.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/main.go` - `serverDispatcher`, `buildAPIEngine`, the four surface dispatchers
- [ ] `internal/plugins/help/help_ai.go` - `printRIBPipeline` teaches the argument form

**Behavior to preserve:**
- A line with no pipe outside quotes answers exactly as today on every surface, in the same envelope
- What every rendering surface prints for a chain the operator types today
- A declared operation computes in the handler before the answer exists
- Column order resolves on the command path as typed
- Alias expansion, address binding, the alias and filter name collision panic, the longest-prefix lookup and its barriers
- A plugin's alias declared through `Pipes` works exactly as today
- `| save` stays refused on a remote surface and allowed in the local client
- The operator availability rules `operatorsFor` states (`always`, `with-rows`, `when-streaming`, `local-only`, the address-field gate)
- The REST and gRPC permission-denied status, and the MCP refusal envelope

**Behavior to change:**
- No caller rewrites the line: the handler receives its declared operations as a structured list, and accounting records the line as typed
- A plugin's `dispatch-command`, MCP, REST and gRPC answer a piped line with the chain applied
- A pipe character inside double quotes is an argument character on every surface; an unbalanced double quote is refused by name
- The RIB's argument grammar stops being a carrier, and every in-tree user is converted
- A plugin can declare operations its own handler runs, through the `Pipes` declaration it already sends
- `show command help` and `ze help command --json` answer one `pipe-operations` list, and the completer offers exactly that list

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The one registry of a command's own operations | `internal/component/command/pipe_filter.go` `pipeFilterRegistry` (a `declarationRegistry`), filled by `RegisterPipeFilters`, read by `lookupPipeFilters` | Declares, per command path by longest prefix, the operation names a command accepts; empty sets are barriers | extends: it stays the ONE registry of declared operations. In-tree commands keep filling it with `RegisterPipeFilters`; a plugin's declared operations are recorded into it under the plugin's name (next row) and withdrawn with the plugin. `Leading` goes: the handler receives the list in chain order |
| A plugin's pipe declaration | `pkg/plugin/rpc/types.go` `PipeDecl` and the Stage 1 `Pipes` field (closed spec-plugin-registers-pipe-operations, `80506f5dbd`); `internal/component/plugin/server/startup.go` `validatePipeDecls`, `registerPluginPipes`; `internal/component/command/alias.go` `RegisterPluginAliases`, `UnregisterPluginAliases`, `aliasBarriers` | A `PipeDecl` names a command, a name, a description and an expansion, recorded as an alias by owner | extends: `PipeDecl` gains whether the operation takes an argument. An entry with an expansion stays an alias; one with no expansion declares an operation the plugin's handler runs, recorded by `registerPluginPipes` into `pipeFilterRegistry` under the same owner, with the same barriers and withdrawal. `validatePipeDecls` and `./le plugin declarations check` judge both kinds |
| The per-command operator join | `internal/plugins/help/help_command.go` `operatorsFor`, and its copy in `internal/le/cli/catalog/catalog.go` | Joins `command.PipeOperatorCatalog()` with the declared shape and address fields and answers each operator's availability | extends: the join moves into `internal/component/command` as the one introspection function, which also folds in the command's declared operations and aliases; `operatorsFor` and the copy are deleted, and every reader calls the function |
| Generic operators and their classes | `internal/component/command/pipe_catalog.go` `PipeOperator`, `PipeClass`; `docs/architecture/cli/command-verbs.md` operator classes | The one list of generic operators, each with its class, argument kind and the shapes it acts on | extends: each entry also states the answer shape its operator produces, which the override check reads |
| Precedence of a command's name over a generic one | `internal/component/command/pipe.go` `foldFilters` with `lookupPipeFilters` and `lookupPipeOperatorByKind` | A generic kind (`match`, `count`, `first`, `last`) is taken by the command when its set holds the name and no format came earlier; nothing checks meaning | keeps: name-based precedence, the longest-prefix lookup, the format-before rule. replaces: the fold. extends: a declared name the catalog holds is an OVERRIDE, held to the generic's argument contract when recorded and to its answer shape when it answers |
| Command help | `internal/plugins/meta/cmd/help.go` `commandHelp`, `pipeFilterHelp`, `pipeAliasHelp` | `show command help <name>` answers `pipe-filters` and `pipe-aliases` | extends: answers `pipe-operations` from the introspection function; the two keys and their helpers go |
| Published command catalog | `internal/plugins/help/help_command.go` `commandEntry` (`ze help command --json`); readers `internal/le/cli/catalog`, `internal/le/doc/yangcontract`, `internal/le/site` | Publishes `operators`, `pipes` and `pipe-aliases` | extends: publishes `pipe-operations` from the same function; the three keys go and every reader converts |
| Pipe completion | `internal/component/command/completer.go` `completePipe`, `pipeExtras`, `aliasSuggestions`, `filterSuggestions` | Offers every catalog operator, then the command's aliases and filters | replaces the listing: `completePipe` reads the introspection function; argument completion is unchanged |
| Answer shape declaration | `internal/component/command/answer_shape.go` `RegisterShape`, `ShapeForCommand` | States whether a command answers rows or one document | extends: the introspection reads it, as `operatorsFor` does. Set aside as the override's shape carrier: it is the command's shape, not one operation's answer |
| The fold, and the dispatcher's fold | `internal/component/command/pipe.go` `foldFilters`, `FoldPipeChain` | Rewrite owned operations into command words | replaces: a partition returning the typed command part, the declared list and the remaining chain; `FoldPipeChain` deleted |
| Chain parse | `internal/component/command/pipe.go` `ParsePipe`, `parsePipeOps`, `parsePipeChain`, `expandAliases`, `bindAddressFields` | Quote-blind cut and split; alias expansion; address binding | extends: quote-aware through the shared scan; expansion and binding unchanged and also run for every data caller |
| Quote-aware scan | `internal/component/plugin/server/command.go` `scanCommand`, `tokenize` | Tokens and the offset of the first pipe outside quotes | extends: its quote rule moves to `internal/component/command`; the dispatcher and the chain parse both call it; an odd quote count becomes a refusal |
| Dispatcher refusal | `internal/component/plugin/server/command.go` `Dispatch` (the `pipeAt != noPipe` arm), `pipeChainRefusal`, `zeCLIForm` | Refuses any unsplit chain | replaces: `Dispatch` accepts the chain; the refusal and its tests go |
| Chain formatter | `internal/component/command/pipe.go` `ProcessPipesDefaultFormatChecked`, `ProcessPipesDefaultFormatLocal`, `processStreamPipes`, `ProcessStreamPipesDefaultFunc`, `ApplyPipes` | Split, fold, validate per surface, and a formatter | extends: return the typed command part and the residual chain; `ApplyPipes` also runs inside the dispatcher for a data caller |
| Plugin command carrier | `pkg/plugin/rpc/types.go` `ExecuteCommandInput`; `internal/component/plugin/server/command.go` `routeToProcess`; `pkg/plugin/sdk/sdk_callbacks.go` `ExecuteCommandHandler`, `executeCommandAnswer` | Carries command, args and peer | extends: a pipe list beside the args on the wire, delivered to the SDK handler; pre-release, no compatibility path |
| Handler context and shared dispatcher | `internal/component/plugin/server/command.go` `CommandContext`; `cmd/ze/hub/main_servers.go` `serverDispatcher`; `internal/component/plugin/dispatch.go` `CallerIdentity` | Carry identity and surface into `Dispatch` | extends: `CommandContext` carries the declared list; `CallerIdentity` carries the rendering statement |
| Split-then-format surface | `internal/component/web/cli_terminal.go` `executeTerminalOperational`, `internal/component/ssh/ssh.go` `execMiddleware`, `internal/component/cli/model_mode.go` `executeOperationalCommand` | Split at the edge, dispatch the command part, format | extends: they send the typed line and state that they render the residual chain |
| RIB pipeline and its model | `internal/component/bgp/plugins/rib/rib_pipeline.go` `showPipeline`, `parsePipelineArgs`; `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` `rpc show` `leaf-list args` | Read scope, filters and one terminal from args, declared as a YANG argument | extends the pipeline: stages built from the structured list. replaces the argument reading and the leaf-list |
| REST parameter tokens | `internal/component/api/requests.go` `BuildCommand` | Turns JSON parameters into tokens, refusing whitespace | extends: refuses `|` and `"` in a parameter value by name |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator line on SSH exec, the web CLI terminal, `ze cli`, MCP, REST execute, gRPC `Execute`; a plugin's `dispatch-command`; the functional-test engine steps
- `show command help <name>`, `ze help command --json`, and `|` followed by Tab

### Transformation Path
1. The caller sends the line as typed. A rendering surface also parses it with the same function to build its formatter
2. `Dispatch` starts accounting on the line as typed (secret-masked as today)
3. `Dispatch` scans once with the shared quote rule; an unbalanced quote is refused
4. The chain is parsed and partitioned: operations `pipeFilterRegistry` holds for the command part become the declared list, in chain order; the rest is the residual chain; an unknown operation on a command that declares a set is refused (`unknownFilterError`)
5. The command part is resolved, validated and authorized as any line without a chain (choice T-6)
6. The declared list reaches the handler: `CommandContext` in-process; the pipe list of `ExecuteCommandInput` for a plugin process
7. The handler computes with it and answers data; an override's answer is checked against the generic's shape
8. The residual chain runs on the answer: by the rendering surface when the caller said it renders; otherwise inside `Dispatch` through `ApplyPipes` (choice T-1 for a format)
9. Introspection: one function in `internal/component/command` answers a command's pipe operations; `commandHelp`, `commandEntry` and `completePipe` read it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator surface ↔ daemon dispatcher | the typed line through `serverDispatcher`, with the rendering statement on `CallerIdentity` | No |
| Plugin ↔ engine (`dispatch-command`) | the typed line in `DispatchCommandInput` | No |
| Engine ↔ plugin (`execute-command`) | the declared list in `ExecuteCommandInput`, over the internal bridge or forked TLS RPC | No |
| Plugin Stage 1 (`declare-registration`) | a declared operation as a `PipeDecl` with no expansion, in the existing `Pipes` list | No |
| Daemon or catalog ↔ help reader | the `pipe-operations` list | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms `ProcessPipes`, `ProcessStreamPipes`, `ParsePipe(`, `foldFilters`, `FoldPipeChain`, `pipeChainRefusal`, `ExecuteCommandInput{`, `OnExecuteCommand(`, `DispatchCommandInput`, `PipeDecl`, `registerPluginPipes`, `pipe-filters`, `pipe-aliases`, `json:"pipes`, `json:"operators`, `with-rows`, and a regular expression for an argument-form RIB line; positive controls `internal/component/ssh/ssh.go` `execMiddleware`, `internal/component/plugin/server/command.go` `routeToProcess`, `internal/component/plugin/server/startup.go` `registerPluginPipes`, `internal/plugins/meta/cmd/help.go` `commandHelp`, `internal/plugins/help/help_command.go` `operatorsFor`. Grep output of 2026-11-12, re-derived at phase 1.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Operator line with a chain | writer | SSH clients, the web terminal page, `ze cli`, MCP clients, REST and gRPC clients, any plugin sending `dispatch-command`, `internal/test/cli/cmd_engine_steps.go` `pipedEngineDispatch`, `internal/le/le/root/leroot.go`, `internal/component/command/local_data.go` | grep of `ProcessPipes*` call sites | preserve the text; every reader now honors it |
| Operator line with a chain | reader | `internal/component/ssh/ssh.go` `execMiddleware`; `internal/component/web/cli_terminal.go`; `internal/component/web/handler_tools.go`; `internal/component/web/page_tools.go`; `internal/component/cli/model_mode.go`; `internal/component/cli/client/main.go`; `internal/component/mcp/tools.go`; `internal/component/api/engine.go`; `internal/component/plugin/server/dispatch_registry.go` `opDispatchCommand`, all into `Dispatch` | read at each site | change: parse once, quote-aware; `Dispatch` accepts the chain |
| RIB argument-form command line | writer | `internal/component/command/pipe.go` `foldFilters`, through every `ProcessPipes*` wrapper | `foldFilters` builds it with `textbuf` | replace: never built |
| RIB argument-form command line | writer | in-tree senders typed by hand: `internal/component/lg/handler_api.go`, `internal/component/lg/handler_ui.go`, `internal/component/lg/handler_graph.go`, `internal/component/api/rest/server.go` (convenience routes), `internal/le/interoplab/bgp/check_extras.go`, `internal/le/interoplab/bgp/check_special.go`, `internal/le/interoplab/bgp/check_rfc4456.go`, `internal/le/interoplab/bgp/check_route_server_replacement.go`, `internal/le/interoplab/bgp/check_rpki_reload.go`, `internal/plugins/help/help_ai.go` `printRIBPipeline`, `internal/test/fixture/plugin_fixture_02.go`, `internal/test/fixture/plugin_fixture_05.go`, `internal/test/fixture/plugin_fixture_11.go`, `internal/test/fixture/plugin_fixture_12_prefix.go`, `internal/test/fixture/plugin_fixture_13_rib.go`, `internal/test/fixture/plugin_fixture_15_show.go`, `internal/test/fixture/plugin_fixture_16.go` (`plugin16PipeFirstLast`), the comments in `internal/component/bgp/plugins/rib/rib_topology.go`, and the `.ci` bodies the search hits in `test/plugin/` (`show-rib-under-load.ci`, `prefix-warn-only-drops-nlri.ci`, `lg-best-table.ci`, `filter-wire-batch.ci`, `rib-pipe-filter.ci`, `test-pipe-first-last.ci`) | grep output, 2026-11-12 | convert every one to the pipe form in the same work |
| RIB argument-form command line | reader | `internal/component/bgp/plugins/cmd/rib/rib.go` `forwardRibRoutes`, `forwardRibBest` → `ForwardToPlugin` → `internal/component/bgp/plugins/rib/rib_pipeline.go` `parsePipelineArgs`; the YANG `leaf-list args`, which the REST and MCP parameter routes read | read | replace: stages from the structured list; a filter or terminal word in args is refused by name with the pipe form as its fix; the leaf-list goes |
| `ze-plugin-callback:execute-command` JSON (`ExecuteCommandInput`) | writer | `internal/component/plugin/server/command.go` `routeToProcess`; `internal/component/plugin/server/subsystem.go`; `internal/component/plugin/server/system.go` (completion request); `internal/test/pluginengine/pluginengine.go` | grep `ExecuteCommandInput{` | change: `routeToProcess` writes the pipe list; the other three send none |
| `ze-plugin-callback:execute-command` JSON (`ExecuteCommandInput`) | reader | `pkg/plugin/sdk/sdk_callbacks.go` `executeCommandAnswer`; every plugin outside the repository, in any language, written against `docs/architecture/api/process-protocol.md` | read | change: the SDK delivers the list; pre-release, a plugin built on the old SDK is rebuilt |
| Stage 1 `pipes` entry (`PipeDecl`) | writer | a plugin's `sdk.Registration` `Pipes`; in-tree plugins that declare aliases today | read `PipeDecl` | change: an entry may carry no expansion and an argument flag; an alias entry is unchanged |
| Stage 1 `pipes` entry (`PipeDecl`) | reader | `internal/component/plugin/server/startup.go` `validatePipeDecls`, `registerPluginPipes`; `./le plugin declarations check` | read | change: an entry with no expansion is recorded into `pipeFilterRegistry` under the plugin's name |
| `show command help` pipe keys | writer | `internal/plugins/meta/cmd/help.go` `commandHelp` (`pipe-filters`, `pipe-aliases`) | read | replace: one `pipe-operations` list |
| `show command help` pipe keys | reader | `internal/test/fixture/plugin_fixture_11_alias.go` `aliasMap`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `test/ui/alias-peers.ci`, `test/ui/alias-summary.ci`, `test/ui/lg-peer-table-flat-payload.ci`, `docs/architecture/api/commands.md` (the `show command help` row); scripts and AI tools outside the repository that read `show command help \| json` | grep `pipe-filters`, `pipe-aliases` | change: each reads `pipe-operations` |
| Published command catalog pipe keys (`operators`, `pipes`, `pipe-aliases`) | writer | `internal/plugins/help/help_command.go` `commandEntry`, `operatorsFor` (`ze help command --json`); `internal/le/cli/catalog/catalog.go` `Entry` and its copy of the join | read | replace: one `pipe-operations` list from the introspection function; the copy deleted |
| Published command catalog pipe keys (`operators`, `pipes`, `pipe-aliases`) | reader | `internal/le/cli/catalog/render.go` `renderPipes`, `internal/le/cli/list/report.go`, `internal/le/doc/yangcontract/command_surfaces.go` (catalog parse and its availability check), `internal/le/site/catalog.go` (the website's command catalog), `internal/test/fixture/ui_fixture_pipe_review_entry_contracts.go`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `docs/features/introspection.md`; the wiki and website pages generated from them | grep `json:"pipes`, `json:"operators`, `pipe-aliases`, `with-rows` | change: each reads `pipe-operations` and its `available` field |
| Dispatcher refusal of a chain | writer | `internal/component/plugin/server/command.go` `pipeChainRefusal` | read | replace: removed |
| Dispatcher refusal of a chain | reader | `test/plugin/dispatch-pipe-chain-refused.ci` with `internal/test/fixture/plugin_fixture_16.go`, the `plugin/server` unit tests, `internal/component/command/pipe_fold_test.go` | grep | replace: the `.ci` asserts the applied answer instead (test-judge) |
| TACACS+ accounting start record | writer | `internal/component/plugin/server/command.go` `BeginAccounting` → `internal/component/tacacs/accounting.go` | read | change: SSH, web and `ze cli` lines are recorded as typed instead of folded |
| TACACS+ accounting start record | reader | the operator's TACACS+ server, outside the repository | RFC 8907 Section 8.3, quoted in `Dispatch` | change as above; the masked form is preserved |

### Integration Points
- `pipeFilterRegistry` - the one registry the partition and the introspection read, filled by `RegisterPipeFilters` and by `registerPluginPipes`
- the introspection function - read by `commandHelp`, `commandEntry`, the `internal/le` catalogs and `completePipe`
- `CallerIdentity` and `serverDispatcher` - the rendering statement
- `ForwardToPlugin` and `routeToProcess` - the declared list onto the wire
- `command.ApplyPipes` - the residual chain inside `Dispatch` for a data caller

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove at implementation: every caller reaches `Dispatch` with the typed line |
| No unintended coupling (components stay isolated) | No | `plugin/server`, `plugins/meta` and `plugins/help` already import `command`; nothing new imports `bgp` |
| No duplicated functionality (extends existing, does not recreate) | No | one scan, one parse, one partition, one registry of declared operations, one introspection function replacing two copies of the operator join; no new Stage 1 field, no new command |
| Zero-copy preserved where applicable (refs, not copies) | No | a residual chain on a data caller materializes the answer, as the surfaces do today |
| Registration over hardcoding, outbound | No | operations are declared through `RegisterPipeFilters` in-tree and through `Pipes` by a plugin, both into `pipeFilterRegistry` |
| Registration over hardcoding, inbound | No | the dispatcher, help, the catalog and the completer name no operation and no command; the applicable generics come from the catalog's classes and the declared shape |

## Design presentation

Written for Thomas. The design is the main session's to present and his to
approve before `ready`. Answers marked (illustrative) show the shape, not
captured bytes. `{"count": 5}` is the shape the RIB's `count` terminal and the
generic `applyCount` both answer today.

### The design in six lines

1. Generic operations apply to every command's answer, from every caller, with no declaration: the global ones to every command producing output, the data ones where the answer carries rows, the stream ones on monitors.
2. A command declares its own operations once, in the one existing registry: `RegisterPipeFilters` in-tree, the existing `Pipes` declaration for a plugin. A declared generic name overrides that operation and keeps its meaning and answer shape; a different meaning takes its own name.
3. Every caller sends the line as typed; nothing rewrites it.
4. `Dispatch` parses the chain once, quote-aware, and hands the declared operations to the handler as a structured list. The handler computes with them before the answer exists.
5. The remaining operations run on the answer: the rendering surfaces keep doing it, and `Dispatch` does it for a plugin, MCP, REST and gRPC.
6. `show command help <name>` and `ze help command --json` list every operation the command answers to, generic, override, own or alias, from one function the `|` completer also reads.

### Before and after

| Where | Line | Before | After |
|-------|------|--------|-------|
| SSH exec | `show bgp rib \| received \| count` | prints the count; the RIB receives the command `show bgp rib received count`; accounting records `show bgp rib received count` | prints the same count; the RIB receives the command `show bgp rib` with operations `received`, `count`; accounting records `show bgp rib \| received \| count` |
| A plugin's `dispatch-command` | `show bgp rib \| count` | error: the command "show bgp rib" carries the pipe chain "\| count", which the dispatcher does not apply; fix `show bgp rib count` | `{"status":"done","data":{"count":5}}` (illustrative) |
| A plugin's `dispatch-command` | `show bgp \| peers \| count` (no declared operation) | the same refusal, fix `show bgp` | the generic chain applied by the engine: the peer count (illustrative) |
| MCP `ze_execute` | `show bgp rib \| received \| count` | refusal text | text result `{"count": 5}`, what `ze cli -c` prints with `\| json` (illustrative) |
| REST execute | `show bgp rib \| family ipv4/unicast \| count` | error status with the refusal | `{"status":"done","data":{"count":5}}` (illustrative) |
| Any caller | `show interface \| count` (a command that declares nothing) | SSH, web, `ze cli`: the generic `count`, `{"count":3}` (illustrative); a data caller: refused as a chain | every caller: the generic `count`, `{"count":3}` (illustrative), with no declaration on the command |
| Any caller | `show bgp rib \| count` (the RIB overrides `count`) | SSH, web, `ze cli`: the RIB counts in its iterator, `{"count":5}`; data callers refused | every caller: the RIB's override runs in the handler, serializes no row, and answers `{"count":5}`, the generic's shape |
| Every caller | a command whose `count` override answers a route list, or `{"total":5}` | answered as written | refused by name: the command's override of `count` answered something `count` does not answer; the generic answers `{"count": N}` |
| Startup | a command declaring `first` with no argument (the generic takes a number) | accepted; the handler meets a missing number | refused when recorded: a `BUG:` panic in-tree, a Stage 1 refusal for a plugin, naming the command, the operation and the generic's argument contract |
| A plugin's Stage 1 | a `Pipes` entry naming `top-talkers` with no expansion on its own command | refused: an alias without an expansion | recorded as an operation of that command; `show x \| top-talkers` reaches the plugin's handler in its pipe list |
| `show command help "show bgp rib" \| json` | what does the RIB command take after a pipe | `pipe-filters`: the RIB's thirteen names; no generic operator; nothing says which names replace a generic | `pipe-operations`: one entry per name with `name`, `source`, `argument`, `available`, `description`: `count` (source `override`, argument `none`, "Count matching routes without serializing rows"), `histogram` (`command`), `display` (`generic`, argument `fields`), `json` (`generic`), and so on (illustrative) |
| `show command help "show interface" \| json` | the same for a command that declares nothing | no pipe key at all, so the answer suggests the command takes nothing | `pipe-operations`: the generics that apply to its answer, each `source: generic` (illustrative) |
| `show command help "show bgp" \| json` | a command with aliases | `pipe-aliases`: `summary`, `peers` with their expansions | the same aliases in `pipe-operations` with `source: alias` and `expansion`, beside the generics (illustrative) |
| `ze help command --json` | the published catalog row of `show bgp rib` | three keys: `operators` (generics with `available`), `pipes` (the RIB's names), `pipe-aliases`; `count` appears twice, once in each of the first two | one `pipe-operations` list, the same entries `show command help` answers; `count` appears once, as `override` (illustrative) |
| Completion | `show bgp rib \| <Tab>` | every catalog operator, then the RIB's names; `count` shows the generic's description | exactly that command's `pipe-operations`, same names and descriptions; `count` shows the RIB's |
| Completion | `show bgp rib status \| <Tab>` (one document) | offers `count`, `first`, `last`, which are then refused | offers only what applies to a document |
| Any surface | `show bgp rib received count` (the old argument form) | `{"count": 5}` | refused: `count` is a pipe operation of `show bgp rib`; fix `show bgp rib \| received \| count` |
| SSH, web, `ze cli` | `show log recent \| match "error\|warn"` | the chain splits at the quoted pipe: `match "error` runs and `warn"` is refused as an unknown operator | `match` receives `error\|warn` as one argument |
| Any surface | `show bgp rib \| prefix "10.0` (odd quote) | the rest of the line silently becomes one token | refused: the line has an unbalanced double quote; fix: close the quote |
| REST parameter | a parameter value `10.0.0.0/8\|count` | becomes part of the line, and would be read as a chain once the dispatcher applies chains | refused like whitespace |
| REST or a plugin | `show bgp \| table` (a format on a data caller) | refused as a chain | under T-1 as recommended: the data is the rendered table as a JSON string, the bytes `ze cli -c` prints (illustrative) |

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `foldFilters` and teach the data callers to fold too (this file's skeleton plan) | Rewrites the line; accounting and authorization keep seeing a line nobody typed |
| Keep the dispatcher refusal and make each data caller split at its edge | Four edges with the same code, and a plugin's `dispatch-command` stays refused |
| Carry declared operations as marked args | Still a rewrite of the argument list |
| A new Stage 1 field for a plugin's operations, beside `Pipes` | A second declaration of pipe names for one plugin, a second list for the declarations check, a second registry |
| A new command to list a command's operations | `show command help` already answers a command's live declarations from inside the daemon |
| Keep `pipe-filters`, `pipe-aliases`, `operators` and `pipes` and add the missing generics | Several lists a reader must merge on two surfaces with different spellings; an override appears twice |
| Move every generic operation into `Dispatch` for every caller, rendering surfaces included (T-2, option B) | Moves the SSH streaming record path, `\| display` selection, `\| save` and paging into the dispatcher; a larger change than the design needs |
| Let each plugin parse the chain text itself | Every plugin reimplements quoting, aliases and validation |

### Design documentation this spec writes, and what it replaces

The design above is written into the body of the pages that document the pipe
operations (phase 8), replacing the rewrite text they carry today. Every file
this spec changes names the page that documents its part in a `// Design:`
header.

| Page | Text it carries today | What phase 8 writes in its place |
|------|-----------------------|----------------------------------|
| `docs/architecture/api/commands.md`, "The surface splits the pipe chain; the dispatcher refuses one" (about lines 2039 to 2077) | The surface splits; the dispatcher refuses; MCP, REST and gRPC refuse | The engine parses the chain for every caller; declared operations reach the handler as a list; generic operations run on the answer; the rendering statement |
| `docs/architecture/api/commands.md`, the per-command registries table (about line 2700) | `RegisterPipeFilters` declares segments "which `foldFilters` rewrites into server-side arguments" | The one registry of declared operations, filled in-tree by `RegisterPipeFilters` and by a plugin through `Pipes`, read by the partition and the introspection |
| `docs/architecture/api/commands.md`, "Pipe aliases", and the `show command help` row (about line 3164) | A plugin's `Pipes` entry is an alias; help answers `pipe-aliases` beside `pipe-filters` | A `Pipes` entry is an alias with an expansion and a declared operation without one; help answers `pipe-operations` |
| `docs/architecture/cli/command-verbs.md`, the owned-filter paragraph (about lines 433 to 437) | "`foldFilters` rewrites `show bgp rib \| peer 10.0.0.1` into `show bgp rib peer 10.0.0.1`" | Subject in the path, operations after the pipe; generic operations by class; an override keeps the generic's meaning and shape; a different meaning takes its own name; help, the catalog and completion read one derivation. "An owned filter takes the name of the field it selects on" stays |
| `docs/features/formatting.md`, "Command-specific filters" (about lines 374 to 398) | Command filters are "folded into the command itself" and "resolved into command arguments" | The handler receives them as operations; the RIB's overrides and own names; `show command help` lists them. "a command filter stops it being produced" is kept |
| `docs/features/introspection.md`, `docs/guide/command-reference.md` | `pipe-aliases`, `operators` and `pipes` as catalog keys; the RIB argument form | `pipe-operations` and its sources; the pipe form |
| `ai/rules/cli.md`, "CLI Grammar: Keywords Before Values", its second sentence, generated from `ai/rules/points/cli/cli-grammar-keywords-before-values/the-first-token-after-the-noun-must-be-a-keyword.md` | "A command addressing one member of a set MUST type the selector with `name`, `id`, `index`, `address`, `type`, `key`, or another schema-defined closed keyword": the typed-selector form on the command path (`show sysctl key <k>`), overturned for selector words on 2026-08-21 (`plan/records/audit-command-pipe-vs-subcommand.md`) | A command path names the subject and stops; selecting members of a set is a pipe operation the command declares, named for the field it selects on (`show sysctl \| key <k>`, `show bgp rib \| peer <sel>`). Keyword before value still governs every token left on the command path, and peer commands keep their positional exception until a conversion spec moves them. The point file is edited and `ai/rules/cli.md` regenerated with `./le ai rules update condensed` |
| `docs/architecture/api/process-protocol.md`, `docs/plugin-development/commands.md`, `docs/plugin-development/handlers.md` | `execute-command` carries args and peer; `Pipes` declares aliases | The pipe list on `execute-command`; a `Pipes` entry with no expansion |

`plan/records/audit-presentation-pipes.md` §1.3 and §4.2 describe the fold as
the mechanism; they are records of an investigation, not design documentation,
and get a dated note pointing at `docs/architecture/api/commands.md`.

`docs/architecture/api/commands.md` holds no design-decisions section today.
The design is written into the page body, so this spec depends on no other
spec for a section format; when such a format lands, the text moves under it.

### What `pipe-operations` holds, and where it comes from

One function in `internal/component/command` answers it for a command path,
built from `operatorsFor`'s join, which moves there. `commandHelp`,
`commandEntry`, the `internal/le` catalogs and `completePipe` read it; nothing
lists an operation by hand.

| Field | Content | Source |
|-------|---------|--------|
| `name` | the word after the pipe | the catalog entry, the declaration, or the alias |
| `source` | `generic`, `override`, `command` or `alias` | `generic`: a catalog operator the command did not declare. `override`: a declared name the catalog holds. `command`: a declared name the catalog does not hold. `alias`: an in-tree or plugin alias |
| `argument` | `none`, `optional`, `text`, `count`, `fields` or `path` | the catalog's `Arg` for a generic or an override; `text` or `none` from the declaration's argument flag for a command operation; `none` for an alias |
| `available` | `always`, `with-rows` or `when-streaming` | `operatorsFor`'s rule for a generic: `ClassGlobal` always; `ClassData` always when the declared shape supports it, `with-rows` when no shape is declared, absent when the declared shape cannot; `ClassStream` when streaming; `always` for an override, a command operation and an alias, which the command itself answers |
| `local-only` | the catalog's `LocalOnly` | generics only |
| `description` | one line | the declaration's for an override or a command operation; the catalog's for a generic; the alias's own |
| `expansion` | the chain an alias stands for | aliases only |

The address-field gate `operatorsFor` applies to `| resolve` and `| origin` is
kept. Whether the `pipe-filters` and `pipe-aliases` keys are replaced or kept
beside the list is decided here, not asked: they are replaced, with every
reader converted, because two lists for one fact is the drift this list exists
to end and no release consumes them.

### How an override keeps the generic's shape

Decided from the code. The catalog is the one list of generic names, the
argument contract is a static fact of the declaration, and the answer exists
only at run time.

| Half | Where it is checked | Why there |
|------|---------------------|-----------|
| The name is a generic one | when the declaration is recorded: `RegisterPipeFilters` and `registerPluginPipes` look the name up in the catalog (`knownPipeOps`) and mark the entry as an override | the catalog is the one list of generic names |
| The argument contract | when recorded: an override's argument flag must agree with the catalog entry's `Arg`; a mismatch is a `BUG:` panic in-tree and a Stage 1 refusal for a plugin | a static fact, refused before any caller meets it |
| The answer shape | at answer time, in `Dispatch`: an answer to an overridden operation is checked against the shape the catalog entry states (`count`: one object holding an integer `count`; `first`, `last`, `match`: the command's rows envelope) and refused by name when it differs | a third-party plugin's tests are not ours to require; the check costs one look at the head of an answer already received |
| The meaning | by review and by each override's own test, which compares it with the generic over the same rows (`TestRIBOverridesKeepGenericMeaning`) | meaning cannot be checked mechanically |

A declared answer shape (`RegisterShape`) was set aside as the carrier: it
states the command's shape, not one operation's answer. A test every override
must pass was kept as the meaning check and set aside as the only check.

### Declarations audited against the override design

Every in-tree `RegisterPipeFilters` call is in
`internal/component/bgp/plugins/cmd/rib/rib.go` `registerPipeFilters`; the two
empty sets are barriers. No in-tree plugin declares an operation through
`Pipes` today, because `PipeDecl` carries aliases only.

| Command | Declared name | Generic of the same name | Verdict |
|---------|---------------|--------------------------|---------|
| `show bgp rib`, `show bgp rib best` | `count` | `applyCount`, `{"count":N}` over the rows | override: same meaning, same shape |
| same | `first`, `last` | `applyFirst`, `applyLast`, N rows | override: same meaning; whether the RIB and the generic refuse N of 0 alike is checked in phase 3 |
| same | `match` | `applyMatch`, case-insensitive substring over every value of a row | override: same meaning on a narrower field set. `(*matchFilter).matches` reads the prefix, the peer, the family and the route attributes, and not the row's `direction` field, so `\| match sent` could keep a row the generic keeps and the override drops. Unverified at the producer for every field: a finding for Thomas; phase 4 compares the two over the same rows |
| same | `histogram`, `graph`, `reason` | none | own names, own descriptions |
| `show bgp rib` | `received`, `advertised`, `peer`, `family`, `prefix`, `path`, `community` | none | own names |
| `show bgp rib` (argument grammar, not a declaration) | `json` terminal in `terminalKeywords` | the `json` format | a different meaning under a generic name, reachable only through the argument form this spec removes |

### Choices only Thomas can make

| # | Question | Options | Recommendation |
|---|----------|---------|----------------|
| T-1 | A format operation (`\| table`, `\| text`, `\| yaml`) on a data caller: a plugin's `dispatch-command`, REST, gRPC | A: render it, and carry the text as the answer's data, the bytes `ze cli -c` prints. B: refuse it by name, saying a data caller receives JSON | A: every surface answers what `ze` answers, and MCP already answers text |
| T-2 | Where the generic operations run for SSH, the web CLI and `ze cli` | A: at the surface, as today, from the same parse. B: inside `Dispatch` for every caller | A: the smaller change, and it leaves the streaming record path where the zero-alloc work put it |
| T-5 | How a derived MCP tool and a REST parameter route express a declared operation, once `leaf-list args` is gone | A: each declared operation becomes an optional parameter of the derived tool, read from the introspection function, and the adapter appends it to the line as a pipe operation. B: derived tools stay at the subject; a caller uses `ze_execute` with the chain | A: derived from the declaration, no list kept by the adapter |
| T-6 | What the authorizer sees for a piped line | A: the typed line, chain included, for a string authorizer, and the command part plus the declared list for a typed one, with read-only judged on the command path. B: the command part only | A: a TACACS+ per-command policy keeps seeing what the operator typed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `APIEngine.Execute` can judge read-only and authorization on the command part of a piped line | `internal/component/api/engine.go` `commandReadOnly` | a chain changes the verdict | `TestExecutePipedReadOnlyCaller` | unvalidated |
| A-2 | Every `OnExecuteCommand` handler can take the request with a pipe list without behaviour change | the 46 non-test registrations read args and peer only | a handler that reads args positionally misreads | the scoped package tests of each plugin touched | unvalidated |
| A-3 | No command other than `show bgp rib` and `show bgp rib best` declares a non-empty operation set today | `RegisterPipeFilters` call sites (grep, 2026-11-12) | another handler reads folded words and breaks | grep at phase 1 | unvalidated |
| A-4 | The rendering surfaces and `Dispatch` produce the same partition from the same line | one function, deterministic, the same registry | SSH and the dispatcher disagree | `TestPartitionSameOnSurfaceAndDispatch` | unvalidated |
| A-5 | Applying a residual chain inside `Dispatch` can reuse `ApplyPipes` over the flattened answer | `CommandDispatcher.JSON` flattens with `ResponseJSON` | a streamed `sdk.Records` answer needs its own path | `TestDispatchAppliesResidualChainToStreamedRows` | unvalidated |
| A-6 | `pipeFilterRegistry` can take an owner-scoped declaration and its withdrawal as the plugin alias registry does | `declarationRegistry.declare`; `RegisterPluginAliases`, `UnregisterPluginAliases` | a plugin's operations outlive the plugin | `TestPluginOperationWithdrawnOnStop` | unvalidated |
| A-7 | `operatorsFor`'s availability rule agrees with what the chain validator admits | `internal/plugins/help/help_command.go` `operatorsFor`, `internal/component/command/pipe.go` `validateDeclaredShape` | help lists an operation the chain refuses | `TestPipeOperationsAgreeWithValidator` | unvalidated |
| A-8 | `operatorsFor` can move into `internal/component/command` | it reads `command.PipeOperatorCatalog()` and `command.Declared`; its local-handler check reads `command.HasLocal` and `HasLocalData`, and `cli.WireToPaths` is passed in by the caller | an import cycle keeps part of it in `plugins/help` | compiling the move in phase 6 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A quote-aware split changes an SSH line that relied on a quote being ignored | an existing `.ci` with a quote and a pipe goes red | read each such test against its expectation before changing it |
| R-2 | The unbalanced-quote refusal reddens a line that worked by accident | a `.ci` or fixture with an odd quote | fix the line |
| R-3 | A handler handed a declared operation ignores it | an answer unfiltered with status done | a wiring test per declaring command |
| R-4 | A residual chain on a large streamed answer from a data caller materializes it | memory on `show bgp rib \| match x` over REST | the cost SSH pays today; recorded, not new |
| R-5 | `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` (design) is written on `foldFilters` rewriting the line | its A-1 and Wiring rows name `foldFilters` | reworked onto the structured list before it is approved; this spec is its dependency |
| R-6 | The SDK signature change misses a handler built outside the default flavor | a tagged build fails | `./le repo compiles check` after the SDK commit |
| R-7 | The published catalog key change breaks the wiki and website generators | `./le doc yang-contract` or the site catalog check goes red | both are readers in the census and convert in phase 6 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Piped commands answer wrongly or are refused on some surface; the RIB answers its full table where a count was asked; a plugin built on the old SDK does not start; a script or generator reading the old help or catalog keys finds nothing |
| How is it reverted? | commit revert per phase; the SDK phase reverts with its plugin rebuilds |
| Who else touches this path? | `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md`, `plan/release/fixes/spec-plugin-alias-reaches-the-client.md`, the zero-alloc RIB streaming work, every plugin with `OnExecuteCommand` or `Pipes`, the catalog tools under `internal/le/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin's `dispatch-command` with `show bgp rib \| count` | → | `Dispatch` partition → the pipe list of `ExecuteCommandInput` → `bgp-rib` stages | `test/plugin/dispatch-pipe-chain-applied.ci` |
| MCP `ze_execute` with a piped line | → | `Dispatch` applies the residual chain | `TestZeExecuteAppliesPipeChain` (`internal/component/mcp/`) |
| REST execute with a piped line | → | `APIEngine.Execute` → `Dispatch` | `TestRESTExecuteAppliesPipeChain` (`internal/component/api/rest/`) |
| gRPC `Execute` with a piped line | → | same | `TestGRPCExecuteAppliesPipeChain` (`internal/component/api/grpc/`) |
| SSH exec with `show bgp rib \| received \| count` | → | the typed line to `Dispatch`, the declared list to the RIB | `test/plugin/rib-pipe-filter.ci`, extended to assert the accounting record holds the typed line |
| SSH exec with a quoted pipe value | → | the shared quote rule | `test/ui/pipe-quoted-value-is-an-argument.ci` |
| A plugin declaring an operation through `Pipes` | → | `registerPluginPipes` → `pipeFilterRegistry` → `execute-command` pipe list | `test/plugin/plugin-pipe-operation.ci` |
| `show command help` and `\|` Tab, for a builtin and a plugin command | → | the introspection function, read by `commandHelp` and `completePipe` | `test/plugin/command-help-pipe-operations.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show bgp rib \| received \| count` on SSH exec | The RIB handler receives the command `show bgp rib` and the operation list `received`, `count` in that order; the answer prints the count; the accounting start record holds the line as typed | none |
| AC-2 | The same line from a plugin's `dispatch-command`, internal and forked | The answer is done with data `{"count": N}`, N the received route count | none |
| AC-3 | The same line on MCP `ze_execute`, REST execute and gRPC `Execute` | Each answers the count in its envelope, the value `ze cli -c` prints for the line with `\| json` | none |
| AC-4 | A piped line whose operations the command does not declare (`show bgp \| peers \| count`) from a plugin, MCP, REST and gRPC | The generic chain is applied by the engine and the answer equals what `ze cli -c` prints with `\| json` | none |
| AC-5 | A format operation on a data caller | Per T-1: the rendered text as the answer's data (A), or a refusal naming the operation and the data caller (B) | none: a format is never dropped in silence |
| AC-6 | A double-quoted argument holding a pipe character, on every surface | The handler receives the whole quoted value as one argument and no operation runs | none |
| AC-7 | A line with an odd number of double quotes, on every surface | Refused with an error naming the unbalanced quote and a fix line; no handler runs | none |
| AC-8 | `show bgp rib received count` (the argument form), on every surface | Refused by `bgp-rib` naming `count` as a pipe operation of `show bgp rib`, fix `show bgp rib \| received \| count` | none: no compatibility path for the old form |
| AC-9 | `bgp-rib` run external | AC-1 and AC-2 hold over the forked transport | none |
| AC-10 | A line with no pipe outside quotes, on any surface | Answers exactly as today | every existing answer |
| AC-11 | A REST parameter value holding `\|` or `"` | Refused by `BuildCommand`, naming the parameter | none |
| AC-12 | Every in-tree argument-form RIB line in the census | Converted to the pipe form; its test or caller answers as before | none |
| AC-13 | An unknown operation on a command that declares a set | Refused as today by `unknownFilterError`, naming the valid set | none |
| AC-14 | A plugin's `Pipes` entry with no expansion on one of its own commands | Recorded into `pipeFilterRegistry` under the plugin; its command receives the operation through `execute-command`; withdrawn when the plugin stops; an entry with an expansion is an alias exactly as today | none |
| AC-15 | (T-5 = A) the derived MCP tool for `show bgp rib` | Offers the declared operations as optional parameters; setting `count` answers the count | none |
| AC-16 | A generic row operation (`count`, `first`, `last`, `match`, `display`) on a command that declares none and answers rows, from every caller | Each caller receives the generic's answer over that command's rows, identical across callers; a command answering one document gets the generic's refusal by name | none |
| AC-17 | A declaration whose name is a generic's (the RIB's `count`, `first`, `last`, `match`), in-tree or through `Pipes` | The handler runs it; its answer has the generic's shape; an argument flag that disagrees with the catalog is refused when recorded; an answer of another shape is refused by name at answer time | none |
| AC-18 | A declaration that means something else (`histogram`, `graph`, `reason`) | It carries a name the catalog does not hold, and help, the catalog and completion show its own description | none |
| AC-19 | `show command help "<name>"` and the `ze help command --json` row for a builtin with declared operations (`show bgp rib`), a command declaring none (`show interface`), a command with aliases (`show bgp`), and a plugin command with a `Pipes` operation | Both answer one identical `pipe-operations` list, each entry with `name`, `source`, `argument`, `available`, `description`, `local-only` for a generic, and `expansion` for an alias; the generics listed are exactly those the chain validator admits for that command; `pipe-filters`, `pipe-aliases`, `operators` and `pipes` are absent | none: pre-release, every reader is converted |
| AC-20 | `\|` then Tab after each command of AC-19, in the editor and in `ze cli` | The names and descriptions offered are exactly that command's `pipe-operations` list | none |
| AC-21 | Every in-tree reader of the old help and catalog keys in the census | Reads `pipe-operations`; `operatorsFor` and its copy in `internal/le/cli/catalog/catalog.go` are gone | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator counts received routes over SSH | SSH → `execMiddleware` → `Dispatch` → partition → `forwardRibRoutes` → `execute-command` with the pipe list → `showPipeline` count terminal → formatter | `test/plugin/rib-pipe-filter.ci` |
| 2 | a plugin asks the engine for a count | `dispatch-command` → `Dispatch` → the same path → data `{"count": N}` | `test/plugin/dispatch-pipe-chain-applied.ci` |
| 3 | an AI agent counts routes over MCP | `ze_execute` → `serverDispatcher` → `Dispatch` → answer | `test/plugin/mcp-pipe-chain-applied.ci` |
| 4 | an operator filters logs with a regex alternation | SSH → the shared scan keeps the quoted value → `match` | `test/ui/pipe-quoted-value-is-an-argument.ci` |
| 5 | a third-party plugin offers its own operation | Stage 1 `Pipes` → `registerPluginPipes` → `pipeFilterRegistry` → `Dispatch` → `execute-command` | `test/plugin/plugin-pipe-operation.ci` |
| 6 | an operator or an AI asks what a command takes after a pipe | `show command help` → `commandHelp` → the introspection; `\|` Tab → `completePipe` → the same | `test/plugin/command-help-pipe-operations.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitPipeLineQuoteAware` | `internal/component/command/pipe_test.go` | the first pipe outside quotes ends the command; a quoted pipe stays; each further unquoted pipe ends an operation | |
| `TestSplitPipeLineUnbalancedQuote` | same | an odd quote count is refused by name | |
| `TestPartitionDeclaredOperations` | same | declared operations leave the chain in order; the command part is unchanged text | |
| `TestPartitionGenericNameAfterFormatStaysResidual` | same | `match`, `count`, `first`, `last` after a format stay residual | |
| `TestPartitionSameOnSurfaceAndDispatch` | same | A-4 | |
| `TestOverrideArgContractRefusedAtRegistration` | same | AC-17 | |
| `TestPipeOperationsForCommand` | same | AC-19: the derivation for a rows command, a document command, an undeclared shape, a streaming command, a command with an address field, an override, an own name and an alias, each with its source, argument and availability | |
| `TestPipeOperationsAgreeWithValidator` | same | A-7: every generic listed as available is admitted by `validatePipesForSurface`, and every generic it admits is listed | |
| `TestCompletionMatchesPipeOperations` | `internal/component/command/completer_test.go` | AC-20 | |
| `TestCommandHelpPipeOperations` | `internal/plugins/meta/cmd/help_test.go` | AC-19 on `show command help` | |
| `TestCatalogPipeOperations` | `internal/plugins/help/help_command_test.go` | AC-19 on `ze help command --json`, the same list as `show command help` | |
| `TestDispatchHandsDeclaredOperationsToHandler` | `internal/component/plugin/server/command_test.go` | `CommandContext` carries the list | |
| `TestRouteToProcessCarriesPipe` | same | the pipe list of `ExecuteCommandInput` | |
| `TestDispatchAppliesResidualChainForDataCaller` | same | AC-4 | |
| `TestDispatchLeavesResidualToRenderingCaller` | same | a caller stating it renders gets the answer without the residual | |
| `TestDispatchAppliesResidualChainToStreamedRows` | same | A-5 | |
| `TestGenericRowOperatorsOnUndeclaredCommand` | same | AC-16 | |
| `TestOverrideAnswerShapeRefused` | same | AC-17 | |
| `TestPluginPipeDeclWithoutExpansionIsAnOperation` | `internal/component/plugin/server/startup_test.go` | AC-14 | |
| `TestPluginOperationWithdrawnOnStop` | same | A-6 | |
| `TestSDKDeliversPipeOperations` | `pkg/plugin/sdk/sdk_callbacks_test.go` | the decoder passes the list to the handler | |
| `TestRIBStagesFromPipeOperations` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | the same stages from the list | |
| `TestRIBRefusesArgumentForm` | same | AC-8 | |
| `TestRIBOverridesKeepGenericMeaning` | same | AC-17, `direction` included | |
| `TestBuildCommandRefusesPipeInValue` | `internal/component/api/requests_test.go` | AC-11 | |
| `TestExecutePipedReadOnlyCaller` | `internal/component/api/engine_test.go` | A-1 | |
| `TestZeExecuteAppliesPipeChain`, `TestRESTExecuteAppliesPipeChain`, `TestGRPCExecuteAppliesPipeChain` | the mcp, rest and grpc packages | AC-3 | |

The `plugin/server` tests of `pipeChainRefusal`,
`internal/component/command/pipe_fold_test.go` (`FoldPipeChain`), and the
fixtures, `.ci` and catalog tests that read the old help and catalog keys
assert behaviour this design removes or renames. They are replaced, not
weakened: the replacement asserts the applied answer, or the same entries in
`pipe-operations`. Each such change is judged by an independent
`ze-test-judge` agent against `docs/architecture/api/commands.md` as rewritten
before it lands. The converted RIB fixtures keep their expected answers and
change only the line they send.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `\| first N`, `\| last N` as declared RIB operations | 1 to the largest int | 1 | 0 | N/A (an overflowing number is refused at parse) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dispatch-pipe-chain-applied` | `test/plugin/dispatch-pipe-chain-applied.ci`, replacing `test/plugin/dispatch-pipe-chain-refused.ci` (fixture in `internal/test/fixture/plugin_fixture_16.go`) | a plugin's `dispatch-command` with a chain answers with it applied, internal and forked | |
| `mcp-pipe-chain-applied` | `test/plugin/mcp-pipe-chain-applied.ci` | an MCP client's piped `ze_execute` answers the count | |
| `rest-pipe-chain-applied` | `test/plugin/rest-pipe-chain-applied.ci` | a REST piped execute answers the count | |
| `plugin-pipe-operation` | `test/plugin/plugin-pipe-operation.ci` | a test plugin declares an operation through `Pipes` and its handler receives it | |
| `command-help-pipe-operations` | `test/plugin/command-help-pipe-operations.ci` | `show command help`, `ze help command --json` and `\|` completion agree, for `show bgp rib` (builtin, overrides) and for the test plugin's command | |
| `rib-pipe-filter` | `test/plugin/rib-pipe-filter.ci` | SSH `\| received \| count`; accounting holds the typed line | |
| `test-pipe-first-last` | `test/plugin/test-pipe-first-last.ci` (`plugin16PipeFirstLast`) | converted to the pipe form, same answers | |
| `pipe-quoted-value-is-an-argument` | `test/ui/pipe-quoted-value-is-an-argument.ci` | a quoted pipe value reaches `match` whole | |
| `rib-argument-form-refused` | `test/plugin/rib-argument-form-refused.ci` | AC-8 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| the scenarios whose checks send argument-form RIB lines (`internal/le/interoplab/bgp/check_special.go`, `check_extras.go`, `check_rfc4456.go`, `check_route_server_replacement.go`, `check_rpki_reload.go`), named in phase 5 from the checks' callers | `test/interop/scenarios/` | FRR, BIRD | goal validation: the converted pipe-form lines answer the counts and rows the checks assert, over routes a real peer sent | |

No wire-visible BGP behaviour changes, so no new protocol interop scenario is
owed (`ai/rules/interop-and-goal-validation.md`: a change to the command
surface with no protocol peer).

## Files to Modify
- `internal/component/command/pipe.go` - quote-aware split; the partition replaces `foldFilters`; `FoldPipeChain` deleted; the `ProcessPipes*` wrappers return the typed command part
- `internal/component/command/pipe_filter.go` - `Leading` goes; override recognition and argument check; the owner-scoped recording and withdrawal; the introspection function
- `internal/component/command/pipe_catalog.go` - each row operator states the answer shape an override is held to
- `internal/component/command/completer.go` - `completePipe` reads the introspection; `pipeExtras` goes
- `internal/plugins/meta/cmd/help.go` - `commandHelp` answers `pipe-operations`; `pipeFilterHelp` and `pipeAliasHelp` go
- `internal/plugins/help/help_command.go` - `commandEntry` publishes `pipe-operations`; `operatorsFor` moves to the introspection function
- `internal/le/cli/catalog/catalog.go`, `internal/le/cli/catalog/render.go`, `internal/le/cli/list/report.go`, `internal/le/doc/yangcontract/command_surfaces.go`, `internal/le/site/catalog.go` - read `pipe-operations`; the copy of the join goes
- `internal/component/plugin/server/startup.go` - `registerPluginPipes` and `validatePipeDecls` handle a `PipeDecl` with no expansion
- `internal/component/plugin/server/command.go` - the shared quote rule; `Dispatch` accepts the chain; `CommandContext` carries the list; `routeToProcess` writes it; the residual chain for a data caller; the override shape check; `pipeChainRefusal` and `zeCLIForm` deleted
- `internal/component/plugin/dispatch.go` - `CallerIdentity` carries the rendering statement
- `cmd/ze/hub/main_servers.go` - `serverDispatcher` carries it into `CommandContext`
- `pkg/plugin/rpc/types.go` - the pipe list on `ExecuteCommandInput`; the argument flag on `PipeDecl`
- `pkg/plugin/sdk/sdk_callbacks.go` - the handler receives the list; every `OnExecuteCommand` registration follows (46 non-test, 8 test files)
- `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/web/handler_tools.go`, `internal/component/web/page_tools.go`, `internal/component/cli/model_mode.go`, `internal/component/cli/client/main.go`, `internal/test/cli/cmd_engine_steps.go` - send the typed line and state that they render
- `internal/component/mcp/tools.go` - under T-5 the derived parameters
- `internal/component/api/engine.go`, `internal/component/api/requests.go` - read-only on the command part; the `BuildCommand` refusal
- `internal/component/bgp/plugins/cmd/rib/rib.go` - the forwarders pass the list; `registerPipeFilters` stays the RIB's declaration
- `internal/component/bgp/plugins/rib/rib_pipeline.go`, `internal/component/bgp/plugins/rib/rib_topology.go` - stages from the list; the argument grammar removed with its refusal
- `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang`, `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` - `leaf-list args` and the argument wording go
- the test readers of the old keys: `internal/test/fixture/plugin_fixture_11_alias.go`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `internal/test/fixture/ui_fixture_pipe_review_entry_contracts.go`, `test/ui/alias-peers.ci`, `test/ui/alias-summary.ci`, `test/ui/lg-peer-table-flat-payload.ci`
- every census writer of an argument-form RIB line (the LG handlers, the REST convenience routes, the interoplab checks, `internal/plugins/help/help_ai.go`, the fixtures, the `.ci` bodies)
- `// Design:` headers: every file above whose header names none of the pipe pages gains the line naming the page that documents its part. Today that is `internal/component/plugin/server/command.go`, `internal/component/plugin/server/startup.go`, `cmd/ze/hub/main_servers.go`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/cli/model_mode.go`, `internal/component/mcp/tools.go`, `internal/component/api/engine.go`, `internal/component/api/requests.go`, `internal/component/bgp/plugins/rib/rib_pipeline.go`, `internal/plugins/meta/cmd/help.go` and `internal/plugins/help/help_command.go` (to `docs/architecture/api/commands.md`), and `internal/component/command/completer.go` and `pipe_catalog.go` (to `docs/architecture/cli/command-verbs.md`); `internal/component/command/pipe.go`, `pipe_filter.go`, `alias.go`, `internal/component/plugin/dispatch.go` and `internal/component/bgp/plugins/cmd/rib/rib.go` already name `docs/architecture/api/commands.md`
- docs: `docs/architecture/api/commands.md`, `docs/architecture/cli/command-verbs.md`, `docs/features/formatting.md`, `docs/features/introspection.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/commands.md`, `docs/plugin-development/handlers.md`, `docs/plugin-development/protocol.md`, `docs/guide/api.md`, `docs/features/mcp-integration.md`, `docs/guide/bgp-policy.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/features/bgp-protocol.md`, `docs/guide/command-reference.md`, and the dated note in `plan/records/audit-presentation-pipes.md`
- `ai/rules/points/cli/cli-grammar-keywords-before-values/the-first-token-after-the-noun-must-be-a-keyword.md` - the typed-selector sentence replaced by the pipe-selection directive; `ai/rules/cli.md` (and `ai/rules/CORE.md`, `ai/rules/TRIGGERS.md` if they change) regenerated, never edited by hand

## Files to Create
- `test/plugin/dispatch-pipe-chain-applied.ci`
- `test/plugin/mcp-pipe-chain-applied.ci`
- `test/plugin/rest-pipe-chain-applied.ci`
- `test/plugin/plugin-pipe-operation.ci`
- `test/plugin/command-help-pipe-operations.ci`
- `test/plugin/rib-argument-form-refused.ci`
- `test/ui/pipe-quoted-value-is-an-argument.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-rib-api.yang` loses `leaf-list args`; no new RPC |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | none |
| CLI commands/flags | No | no command added; `show command help` and `ze help command --json` change their answer; the RIB loses argument words |
| CLI grammar (keyword before value) | Yes | the RIB words move after the pipe (`ai/rules/cli.md` still states the typed-selector rule; phase 8 aligns it with the design pages) |
| Editor autocomplete | Yes | `completePipe` reads the introspection; argument completion for the removed words goes |
| Functional test for new RPC/API | Yes | the `.ci` rows above |
| Pipe completeness | Yes | every caller reaches `ApplyPipes` |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Doctor code per problem class | N-A | the new refusals are operator errors on a command line, answered at the line |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the feature file covering pipe operators, found with `./le feature report` in phase 8 |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show bgp rib` operations, the `show command help` answer); `ai/rules/cli.md` "CLI Grammar: Keywords Before Values" through its point file (the typed-selector sentence replaced by pipe selection) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/commands.md`, `docs/plugin-development/handlers.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/api.md`, `docs/guide/bgp-policy.md`, `docs/features/formatting.md`, `docs/features/introspection.md` |
| 7 | Wire format changed? | No | BGP wire unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, and `ai/rules/plugins.md` if it names the handler signature |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-verbs.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the changed files' `// Design:` headers: `docs/architecture/api/architecture.md` (REST and gRPC execute apply the chain: affected), `docs/guide/mcp/overview.md` (`ze_execute` with a chain: affected), `docs/architecture/api/ipc_protocol.md` (the `PipeDecl` and `execute-command` fields: affected), `docs/architecture/system-architecture.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/web-interface.md` and `docs/architecture/config/yang-config-design.md` (re-read in phase 8; expected unaffected). The full list is derived by `./le spec citation anchors spec plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` in phase 8 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every argument-form example and every old help or catalog key example in the census docs |

## Implementation Steps

Each phase is sized for one agent of about 60 tool calls.

1. **Phase: Wiring (MANDATORY FIRST)** -- failing tests at every entry point
   - Tests: the Wiring Test rows
   - Files: the new `.ci` drafts, the three surface unit tests
   - Verify: each fails because the chain is refused or folded, or help answers the old keys
2. **Phase: one quote rule** -- the shared scan in `internal/component/command`; the unbalanced quote refused
   - Tests: `TestSplitPipeLineQuoteAware`, `TestSplitPipeLineUnbalancedQuote`, AC-6, AC-7
   - Files: `internal/component/command/pipe.go`, `internal/component/plugin/server/command.go`
3. **Phase: partition, overrides and dispatch** -- the partition replaces `foldFilters`; override recognition, the argument check and the shape check; `Dispatch` accepts the chain; the carriers; the residual chain for a data caller; the surfaces send the typed line; the refusal and `FoldPipeChain` deleted
   - Tests: AC-1, AC-3, AC-4, AC-5, AC-10, AC-13, AC-16, AC-17, AC-18
   - Files: `internal/component/command/pipe.go`, `pipe_filter.go`, `pipe_catalog.go`, `internal/component/plugin/server/command.go`, `internal/component/plugin/dispatch.go`, `cmd/ze/hub/main_servers.go`, the surface files, `internal/component/api/engine.go`, `internal/component/api/requests.go`
4. **Phase: the plugin carrier and declaration** -- the pipe list on `execute-command`, the SDK handler, every `OnExecuteCommand` rebuilt; `PipeDecl` with no expansion recorded into `pipeFilterRegistry`; the RIB builds its stages from the list and refuses the argument form
   - Tests: AC-2, AC-8, AC-9, AC-14
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `internal/component/plugin/server/startup.go`, the 54 handler files, the RIB files and YANG. If the 54 handler edits do not fit one agent, the main thread cuts the handler rebuild into its own package
5. **Phase: convert every RIB user** -- the census writers converted (the LG, the REST routes, the interoplab checks, `internal/plugins/help/help_ai.go`, the fixtures, the `.ci` bodies)
   - Tests: AC-12; the interop checks
6. **Phase: introspection** -- `operatorsFor` moves into `internal/component/command` and gains the declared operations and aliases; `commandHelp`, `commandEntry` and `completePipe` read it; every reader of the old keys converted; the copy in `internal/le/cli/catalog/catalog.go` deleted
   - Tests: `TestPipeOperationsForCommand`, `TestPipeOperationsAgreeWithValidator`, `TestCompletionMatchesPipeOperations`, `TestCommandHelpPipeOperations`, `TestCatalogPipeOperations`, `command-help-pipe-operations.ci`, AC-19, AC-20, AC-21
   - Files: `internal/component/command/pipe_filter.go`, `completer.go`, `internal/plugins/meta/cmd/help.go`, `internal/plugins/help/help_command.go`, the census readers
7. **Phase: T-5, when approved** -- the derived parameters
   - Tests: AC-15
8. **Phase: documentation** -- the design written into each page of "Design documentation this spec writes", the `// Design:` headers, the record note, and every other page in the Documentation checklist

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No caller builds a command string from a pipe operation |
| Correctness | The declared list reaches a forked plugin, not only an internal one |
| Correctness | One registry of declared operations, one introspection function: no new Stage 1 field, no second registry, no new command, no hand list of operators in help, the catalog or completion |
| Naming | The wire fields, JSON tags and help keys are kebab-case and named in `docs/architecture/api/process-protocol.md` and `docs/architecture/api/commands.md` |
| Data flow | One parse function (A-4); help, the catalog, completion and the validator agree (A-7) |
| Operator errors and doctor codes | The unbalanced-quote, argument-form, override-shape and REST-value refusals are `opererr` errors with an exact fix line |
| Rule: no-layering | `foldFilters`, `FoldPipeChain`, `pipeChainRefusal`, `zeCLIForm`, `Leading`, `leaf-list args`, the RIB argument reading, the keys `pipe-filters`, `pipe-aliases`, `operators` and `pipes`, `pipeFilterHelp`, `pipeAliasHelp`, `pipeExtras`, `operatorsFor` and its copy are deleted, not left beside the new path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No rewrite remains | a grep of `internal/` for `foldFilters`, `FoldPipeChain` and `pipeChainRefusal` answers nothing |
| The wire fields exist | `pkg/plugin/rpc/types.go` shows the pipe list on `ExecuteCommandInput` and the argument flag on `PipeDecl` |
| Every surface applies the chain | the AC-3 tests and the new `.ci` pass |
| No argument-form RIB line remains in-tree | the census search answers nothing outside the refusal test |
| Help, the catalog and completion agree | `command-help-pipe-operations.ci` passes; a grep for the old keys answers nothing |
| The design is in the pages | the three pages carry the design text and no rewrite text; every changed file's `// Design:` header names its page |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a REST parameter cannot open a chain (AC-11); an MCP derived parameter is written as a pipe operation by the adapter, never spliced as text; a plugin's `Pipes` entry names only its own commands (`validatePipeDecls`) |
| Authorization | read-only judged on the command path; an operation never turns a write into a read (T-6); a string authorizer still sees the typed line |
| Accounting | the record holds the typed line, secret-masked by `config.DisplayCommand` as today |
| Resource exhaustion | a residual chain on a data caller materializes the answer only when a chain exists (R-4) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The refusal landed this week was right that a data caller cannot render a chain, and wrong about who handles it: the declared operations belong in the handler and the generic ones on the answer, for every caller.
- A plugin's operations need no new declaration: `Pipes` already names a pipe word per command, and an entry with no expansion is the operation itself.
- The per-command operator join already exists twice (`operatorsFor` and its `internal/le` copy); the introspection is that join moved to its owner and completed, not a new derivation.
- The completer offers every catalog operator to every command, so a document-shaped command is offered `count` and then refuses it.
- `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` A-1 assumes the rewrite; it becomes a dependent of this spec.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-12 | MCP `ze_execute`, REST `handleExecute`, gRPC `Execute` hand `Dispatch` the pipe chain unsplit |
| `splitter-ignores-the-quoting-its-tokenizer-honors.md` | 2026-11-12 | `ParsePipe` and `parsePipeOps` split a quoted pipe value as a chain |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Declared operations travel as a structured list | rewritten command words; marked args; the chain text for the plugin to parse | a list crosses the process boundary as a value, and the line stays as typed |
| A plugin declares operations through its existing `Pipes` entries, recorded into the existing `pipeFilterRegistry` | a new Stage 1 field and registry | one declaration of pipe words per plugin, one registry, one list for the declarations check |
| `show command help` and `ze help command --json` answer one `pipe-operations` list from `operatorsFor` moved into `internal/component/command`, and the completer reads it | a new command; keeping `pipe-filters`, `pipe-aliases`, `operators` and `pipes` and adding the generics | one question, one answer, one derivation; an override appears once, with its source; the `internal/le` copy of the join goes |
| `Dispatch` parses the chain for every caller | each surface parses and folds | one parse, and the engine handles a declared operation for every caller |
| The zero value of the rendering statement means the engine applies the residual chain | the zero value means the caller applies it | a caller added later that forgets to say gets the chain applied, never dropped |
| An override's answer shape is checked by `Dispatch` at answer time, its argument contract when recorded | a declared answer shape (`RegisterShape`); a test every override must pass, alone | `RegisterShape` states the command's shape, not one operation's; a test cannot be required of a third-party plugin |
| The RIB's argument words are refused, not accepted beside the pipe | keep both forms | no-layering, and no compatibility path for a form no release shipped |
| One spec for the three defects | three specs | they share the parse |

## Known Limitations
- Interactive `ze cli` parses the chain in the client, so a plugin's alias or declared operation is unknown to its parse and its completion; `plan/release/fixes/spec-plugin-alias-reaches-the-client.md` owns it.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path` for `./internal/component/command/`, `./internal/component/plugin/server/`, `./internal/plugins/meta/cmd/`, `./internal/plugins/help/`, `./internal/component/mcp/`, `./internal/component/api/`, `./pkg/plugin/sdk/`, the `bgp-rib` package under `ze_bgp`, the `internal/le` catalog packages touched, and the named tests in `./le test functional plugin` and `./le test functional ui`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` only, in the same `./le commit create` script
