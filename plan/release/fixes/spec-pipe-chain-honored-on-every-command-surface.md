# Spec: a command's declared pipe operations reach its handler from every caller, and the chain is honored on every surface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped daemon behaviour. A plugin's `dispatch-command`, MCP, REST and gRPC refuse a piped command that SSH and `ze cli` answer. Every rendering surface splits a quoted pipe value as a chain. `foldFilters` rewrites the operator's line into a different command. All three are wrong answers on shipped operator surfaces (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - (spec-plugin-dispatch-pipe-chain-reaches-the-handler, closed in `2616478e64` and `9321d263aa`, landed the refusal this spec replaces; the quote-aware scan this spec extends to the chain is `ScanInvocation` and `ScanExecution` in `internal/component/command/invocation_scan.go`). `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` depends on this spec |
| Phase | 8/8 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-20 |

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
3. **The chain is not quote-aware after its first pipe.** Part of this defect
   landed since the spec was written: `ParsePipe` now takes the first pipe
   outside quotes from `ScanInvocation`
   (`internal/component/command/invocation_scan.go`), and `ScanExecution`
   refuses an unfinished quote in the command part with its column. What
   remains: `parsePipeOps` still splits the rest at every pipe character and
   reads each segment with `strings.Fields`, so a quoted value in the chain is
   torn at a pipe and at a space, and its quotes reach the operator as data.
   `show log recent | match "error|warn"` runs `match "error` and refuses
   `warn"`. The separate chain grammar is deliberate today (`splitPipeAt`:
   "the existing filter grammar owns the chain after the first operator"); this
   spec overturns it, see "The chain grammar".

Two small defects met during implementation are fixed in this work, each in a
commit of its own (`ai/META-DECISIONS.md`, a small problem found while working
widens the spec):

4. **The web test binary refused every bgp commit.** f08db299d4 added a peer
   summary check to `configcli.ValidateContent`, and `infra.SummarizeBGPPeers`
   fails closed (`errNoBGPEngine`) when no summarizer is registered. The web
   package's engine stand-in, `installPeerValidator`, registered the tree
   resolver and the peer validator but no summarizer, so each web test that
   commits a bgp config was refused with "this binary was built without the
   BGP engine". Fixed in 67c4ea9356 (AC-40).
5. **The selector refusal lost its text.** After b5a7d0dfe7 a dispatched line
   missing its selector is refused with the generic missing-argument text
   instead of `missingSelectorError`'s, so
   `TestSelectorRefusalPlacesTheSelectorWhereDispatchReadsIt` and
   `TestDispatchImplicitSelectorMissingValue` are red. Being fixed as a
   separate small commit (AC-41). Fixed in ccb004dcfb.
6. **Graceful-restart replay was refused.** Since a27a951239 the container of
   `clear bgp rib out-excluding-owner` declared no leaves; `owner` and
   `family` existed only on the `ze-rib-api` rpc input, so the argument
   validator bound nothing and `ribClearOutExcludingOwnerArgs` refused every
   replay `bgp-gr` dispatches (`owner N family F`). The container now declares
   both leaves. Fixed in 34cc4767ac (AC-42).
7. **The OpenAPI document published routes no handler served.** It listed a
   `/api/v1/execute/<command path>` operation per command, while
   `registerRoutes` serves only `POST /api/v1/execute` and
   `GET /api/v1/execute/stream`, so a client following it got 405 or 404. It
   now publishes the served routes only and describes each command as a
   request-body schema. Fixed in 6537dc0f86 (AC-43).
8. **The RIB `match` never read large or extended communities.** The match
   filter (`rib_pipeline.go` `matchOutRoute`, `matchInEntry`) skipped both, so
   no RIB selection could find a large community; a status-only fixture check
   hid it. Fixed in 4faad7bbe7 (AC-44).
9. **A stream's `| match` refused every one-document event.** Each event was
   refused with "match needs rows ... it holds one document", so
   `monitor event | match x`, which `docs/guide/monitoring.md` documents,
   never worked. Each streamed event is now one row of the stream: a JSON
   event is kept or dropped whole by its values, a text event is matched line
   by line (`applyStreamEventPipes`, `applyEventMatch`, `pipe.go`). Fixed in
   72502f3950 (AC-45).
10. **A `ze cli -c` monitor lost its `| save` on a signal.** The monitor ends
    only by signal, and the signal killed the client, so the save never
    committed and left a `.ze-save` temporary behind. `StreamMonitor` now
    catches SIGINT and SIGTERM, the SSH stream closes on its context, and the
    saves commit with exit 0. Fixed in 72502f3950 (AC-46).

Three routes answer a piped line without ever reaching `Dispatch`, so a fix
placed in `Dispatch` alone leaves them rewriting:

- **The stream route.** Four sites look up a streaming handler through
  `pluginserver.GetStreamingHandlerForCommand`, and none calls `Dispatch`:
  `serverStreamCommand` (`cmd/ze/hub/server_stream.go`, the web),
  `apiStreamSource` (`cmd/ze/hub/api.go`, which `APIEngine.Stream` calls for
  REST and gRPC), the SSH exec stream arm (`internal/component/ssh/ssh.go`,
  into the `SetStreamingExecutorFactory` closure of
  `cmd/ze/hub/service_ssh.go`) and the daemon-hosted model
  (`internal/component/cli/model_monitor.go`). The first three run their own
  accounting and authorization. The web site and the SSH arm split with
  `ProcessRemoteStreamPipes`; the REST and gRPC site calls no split and hands
  the lookup the whole line. `StreamingHandler` takes no `CommandContext`.
- **The local-data route.** `ServeLocal` and `RenderLocalAnswer`
  (`internal/component/command/local_data.go`) answer in the caller's process
  through `ProcessPipesDefaultFormatLocal`.
- **The `ze cli` monitor.** `StreamMonitor`
  (`internal/component/cli/client/main.go`) splits in the client with
  `ProcessStreamPipesDefaultFunc` and sends the command part over SSH.

The defects and the routes share one mechanism, the parse of a line into a
subject and a chain, so one spec owns them and one partition function in
`internal/component/command` serves every route. The list of operations a
command answers to is derived once and read by help, the published catalog and
completion.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - one command registry behind every interface; a plugin can run in another process; `pkg/` is a published contract
  → Constraint: a command is declared once and reaches every interface; a surface adds nothing of its own for one command
  → Constraint: what a handler receives crosses the process boundary as a value: an in-process-only carrier breaks the forked `bgp-rib`
- [ ] `ai/doors/commands-cli.md` - the command model, `execMiddleware`, the pipe operators as a contract scripts read
  → Constraint: a handler answers data and output goes through `ApplyPipes`; a script reading `| json` relies on `pipe_catalog.go`
  → Constraint: help, usage and completion are derived from the owner's registration, never written in a second place
- [ ] `ai/doors/api-surfaces.md` - REST, gRPC and MCP are thin adapters over `serverDispatcher`
  → Constraint: the adapters own no command; REST and gRPC turn structured parameters into command tokens in `api.prepareCommand` and `buildInvocation` (`internal/component/api/invocation.go`) before anything parses them. `api.BuildCommand` has no non-test caller
- [ ] `ai/doors/plugin-infra.md` - `plugin/server` `Dispatcher`, the process protocol, the SDK
  → Constraint: every feature works over both transports; a field in `rpc` is wire, not Go; the SDK stays generic
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` `serverDispatcher` and `buildAPIEngine`
  → Constraint: one dispatcher closure per surface, the same `pluginserver.Dispatcher` behind each
- [ ] `ai/doors/bgp-plugins.md` - `bgp-rib` runs internal or forked; its two declarations must agree
  → Constraint: `./le plugin declarations check` already compares the `Commands` and `Pipes` lists, so an operation declared through `Pipes` is judged with no new list
- [ ] `ai/doors/system-plugins.md` - `internal/plugins/help` and `internal/plugins/meta` are command providers whose catalog and command help publish the pipe operations
  → Constraint: a plugin owns its whole surface, help included, so each help producer is converted by its owner
- [ ] `ai/doors/access-security.md` - TACACS+ START/STOP accounting comes from one hook in `Dispatcher.Dispatch`
  → Constraint: the accounting record is the operator's command log on the TACACS+ server; it must record what was entered. RFC 8907 Section 8.3: "To support this mode of operation, TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized." (read in `rfc/full/rfc8907.txt`)
  → Constraint: the typed authorizer contract is `aaa.CommandArgsAuthorizer` (`internal/component/aaa/command_args.go`), implemented in `tacacs/authorizer.go`, `authz/register.go` and `aaa/login_profiles.go`; it receives the command, its tokens and the peer, never a rebuilt string
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
- `ScanInvocation` and `ScanExecution` (`internal/component/command/invocation_scan.go`) already answer the tokens and the offset of the first pipe outside quotes, and the dispatcher, `ParsePipe` and the API engine call them. The quote rule stops at that first pipe; the chain after it is still read by a second, quote-blind grammar.
- Three routes never reach `Dispatch` (the stream route, the local-data route, the `ze cli` monitor), so the partition lives in `internal/component/command` and every route calls it.
- A caller in another process cannot use `CallerIdentity` to say it renders. The SSH session environment (`sshclient.EnvConfigName`, `EnvCLIMode`, read by `parseSessionRequest`) is the carrier that already crosses that boundary.
- The web CLI terminal already is the split, dispatch, format pattern; the data callers need the same chain applied by the engine.
- The RIB's YANG declares its pipeline words as a command argument (`ze-rib-api.yang` `rpc show`, `leaf-list args`).
- A plugin already declares pipe names at Stage 1 (`Pipes`, `rpc.PipeDecl`, recorded by `registerPluginPipes`). That declaration is the one a plugin's operations extend.
- The per-command join of the operator catalog with the declared shape already exists, twice: `operatorsFor` (`internal/plugins/help/help_command.go`) and the operator loop in `internal/le/cli/catalog/catalog.go`. `show command help` does not call it, and the completer offers every catalog operator to every command and drops an override's description.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/invocation_scan.go` - `ScanInvocation` answers the decoded command words, the offset of the first pipe outside double quotes and whether a quote is open; a quoted value decodes escaped quotes, backslashes, n, r and t, and a backslash outside quotes is refused (`ErrBackslashInInvocation`). `ScanExecution` refuses an unfinished quote with its one-based column and stops at the first unquoted pipe: "the caller owns the filter grammar beyond that boundary"
- [ ] `internal/component/command/pipe.go` - `ParsePipe` takes the first unquoted pipe from `ScanInvocation` and hands the rest to `splitPipeAt`, whose comment states the chain keeps its own grammar; `parsePipeOps` splits that rest at every `|` with `strings.SplitSeq`, reads each segment with `strings.Fields` and maps names through `knownPipeOps`, so quotes in the chain are data; `parsePipeChain` and `parseExecutionPipeChain` expand aliases and bind address fields; `foldFilters` first runs `collectPipeMeta`, `validateKnownPipeArguments` and `validateCountDocumentFollowers`, then looks up the command's `PipeFilter` set by longest prefix and removes every owned operation from the chain wherever it stands: a generic-named `match`, `count`, `first`, `last` unless a format operator came earlier (`formatSeen`), an own-named one always. It appends `<name> <arg>` to the command string, `Leading` ones first, through `appendFilter`, which splits the value with `strings.Fields`; `FoldPipeChain` runs the same fold for the dispatcher's refusal; `HasFormatPipe` folds to answer whether the chain names a format; the `ProcessPipes*` and `ProcessStreamPipes*` wrappers call `ColumnsForCommand`, `foldFilters` and `validatePipesForSurface` and return the rewritten command plus a formatter over `ApplyPipes`, which takes the `pipeChainMeta` and injects it (`injectPipeMeta`); `applyCount` answers `{"count":N}` over the rows `rowsForOperator` finds; `applyMatch` keeps the rows whose value text holds the pattern as a case-insensitive substring, never a regular expression
- [ ] `internal/component/command/render_records.go` `RenderRecords`, `internal/component/command/pipe_records.go` `applyPipesRecords`, `internal/component/command/render_counted_document.go` `prepareDocumentPipes` - three more callers of `foldFilters`, on the record-stream and counted-document paths
- [ ] `internal/component/command/local_data.go` - `ServeLocal` answers a local-data command in the caller's process from the operator's line and builds the formatter with `ProcessPipesDefaultFormatLocal(input, sessionFormat)`; the handler receives validated arguments and no chain. `RenderLocalAnswer` is handed a command path and a payload, never a line: it calls `ProcessPipesDefaultFormatLocal(path, "")` only for the path's default format, so it reads no chain
- [ ] `cmd/ze/hub/server_stream.go` - `serverStreamCommand` builds its own `CommandContext`, calls `BeginAccounting` and `IsAuthorized` on the whole input, then `ProcessRemoteStreamPipes`, hands the command part to `GetStreamingHandlerForCommand`, and wraps the handler in `RunStreamPipes`; it never calls `Dispatch`
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - six wrappers implement `AuthorizeCommandArgs` in front of the backends: `liveLocalAuthorizer`, `liveLocalProfileAuthorizer`, `acceptedLocalGenerationAuthorizer`, `acceptedLocalGenerationProfileFallback`, `liveAAABundleAuthorizer` and `liveAAABundleProfileAuthorizer`. Each passes the argument tokens on to a typed authorizer, or flattens them with `aaa.CanonicalCommand` for a string one. `liveAAABundleAccountant` (`commandStart`, `commandStop`) does the same for accounting: the tokens to a typed accountant, `aaa.CanonicalCommand` of the tokens to a string one
- [ ] `internal/component/tacacs/authorizer.go` - `AuthorizeCommandArgs` sends the command and its tokens as `cmd` and `cmd-arg` fields (`splitTacacsCommandArgs`); on a server error `fallbackArgs` hands the same tokens to the local authorizer, typed when it is one, flattened through `aaa.CanonicalCommand` otherwise
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container rib` under `show bgp peer` (`ze:command "ze-bgp:peer-rib"`) declares `container scope` with `leaf scope`, an enumeration of `sent`, `advertised`, `received` and `sent-received`, whose description says "The word is typed first, before any pipe stage". `internal/component/bgp/plugins/rib/yang_enum_agreement_test.go` `TestRIBScopeKeywordsMatchTheYANGModel` holds `scopeKeywords` to that enumeration
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` `newJSONTerminal`, `showRows`, and `rib_pipeline_best.go` `bestPipeline`, `bestResultFor` - the `json` terminal answers one document `{"routes": [...]}`, each row `serializeRouteItem` plus `peer` and `direction`, in the sources' own order. For `show bgp rib` the bare command streams the same rows under the same `routes` key (`showRows`). For `show bgp rib protocol` the bare command itself runs `newJSONTerminal` (`rib_inject.go` `showProtocolPipeline`). For `show bgp rib best` the bare command answers another shape: rows under `best-path`, each `family`, `prefix`, `best-peer`, `multipath-peers` and nested `attributes` (`bestResult`), where `best json` answers `routes` rows with `peer`, `direction` and flat attributes. `refusePipelineArgs` gives `show bgp rib count` and `show bgp rib best count`, the argument form, as the fix line of every refused argument
- [ ] `internal/component/plugin/server/handler.go` - `StreamingHandler` takes a context, the server, a writer, the username and validated arguments, and no `CommandContext`
- [ ] `cmd/ze/hub/service_ssh.go` - the `SetStreamingExecutorFactory` closure is where an SSH exec stream is accounted (`BeginAccounting`), authorized (`IsAuthorized`) and looked up (`GetStreamingHandlerForCommand`); it receives only what `execMiddleware` passes, the command part
- [ ] `cmd/ze/hub/api.go` - `apiStreamSource`, the REST and gRPC stream source: `BeginAccounting`, then `GetStreamingHandlerForCommand`, then `IsAuthorized`, each on the whole command it is handed, with no pipe split. `APIEngine.Stream` calls this site, not `server_stream.go`, which serves the web
- [ ] `internal/component/cli/model_monitor.go` - `extractMonitorCmdArgs` is the fourth caller of `GetStreamingHandlerForCommand`, for the daemon-hosted model
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` `registerRibAnswerShapes` - `show bgp peer rib` (`cmdBgpPeerRib`) "forwards to cmdRibShow with a peer selector (forwardRibRoutes)" and "inherits none of them": a fourth command reaches the RIB argument grammar, with no pipe set on its path
- [ ] `internal/component/cli/model_keys.go`, `internal/component/cli/model_ping.go`, `internal/component/cli/model_traceroute.go` - the model's stream split picks `ProcessStreamPipes` or `ProcessRemoteStreamPipes` by filesystem authority; the ping and traceroute monitors call `ParsePipe` to recognize their command
- [ ] `internal/component/cli/client/answer.go` - `daemonOutput.end` calls `HasFormatPipe` on the operator's line to decide whether an empty answer prints `OK`
- [ ] `internal/component/ssh/session.go` - `parseSessionRequest` reads the session environment keys `sshclient.EnvConfigName` and `EnvCLIMode`: the existing carrier of a client statement across the SSH boundary
- [ ] `internal/component/command/pipe_catalog.go` - `PipeOperator{Name, Kind, Class, Arg, Repeat, Description, NeedsAddressField, LocalOnly}` with `Applies(shape)`; `PipeClass` is `ClassGlobal`, `ClassData`, `ClassStream`; `PipeArgKind` is `ArgNone`, `ArgOptional`, `ArgText`, `ArgCount`, `ArgFields`, `ArgPath`; `knownPipeOps` derives from the catalog
- [ ] `internal/component/command/pipe_filter.go` - `PipeFilter{Name, Description, TakesArg, Leading}`; `RegisterPipeFilters` declares a set per command path in `pipeFilterRegistry`, a longest-prefix `declarationRegistry`; an empty set is a barrier; it refuses with a `BUG:` panic, holds no owner and has no withdrawal; `PipeFiltersForCommand` and `filterSuggestions` read it
- [ ] `internal/component/command/alias.go` - `RegisterAliases` and `RegisterPluginAliases` (by owner, withdrawn by `UnregisterPluginAliases`) record pipe aliases; `aliasBarriers` derives a plugin's barriers from its own command list
- [ ] `internal/component/command/completer.go` - `completePipe` offers every entry of `PipeOperators` whatever the command's shape, then the command's aliases and filters (`pipeExtras`), skipping an extra a catalog operator already offered, so an override shows the generic's description
- [ ] `internal/plugins/meta/cmd/help.go` - `commandHelp` (`show command help <name>`) answers `pipe-filters` (`PipeFiltersForCommand`) and `pipe-aliases` (`AliasesForCommand`, with expansion), beside the other declaration keys; no generic operator and no override status
- [ ] `internal/plugins/help/help_command.go` - `operatorsFor` joins `command.PipeOperatorCatalog()` with the declared shape and address fields, and answers each operator's `available` (`always`, `with-rows`, `when-streaming`) and `local-only`; `commandEntry` publishes `operators`, `pipes` and `pipe-aliases` for `ze help command --json`
- [ ] `internal/le/cli/catalog/catalog.go` - a second copy of the same operator join; `Entry` carries `pipes`, `operators`, `pipe-aliases`
- [ ] `internal/le/cli/catalog/render.go` - `renderPipes` renders `operators` by availability
- [ ] `internal/le/cli/list/report.go`, `internal/le/doc/yangcontract/command_surfaces.go`, `internal/le/site/catalog.go` - read the published `pipe-aliases`, and the last two the `operators` availability
- [ ] `internal/test/fixture/plugin_fixture_11_alias.go`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `internal/test/fixture/ui_fixture_pipe_review_entry_contracts.go` - assert `pipe-aliases`, `pipe-filters`, `pipes` and the operator availability sets
- [ ] `internal/component/plugin/server/startup.go` - `registerPluginPipes` turns each Stage 1 `rpc.PipeDecl` into a `command.PluginAlias` and calls `RegisterPluginAliases`, which refuses the whole batch with an error; `validatePipeDecls` checks the plugin owns the paths it names and refuses an entry whose expansion is empty ("expands to nothing"); the rollback arms of the registration call `UnregisterPluginAliases`
- [ ] `internal/component/plugin/server/restart.go` - the plugin cleanup calls `UnregisterPluginAliases` on the three paths a plugin leaves by: a failed startup, a removal from the config, a restart
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - `registerPipeFilters` declares fifteen names on `show bgp rib`: `received`, `advertised` (Leading), `peer`, `family`, `prefix`, `covering`, `exact`, `path`, `community`, `match`, `first`, `last` (TakesArg), `count`, `histogram`, `graph`; twelve on `show bgp rib best`: the same without the scope words, `covering` and `exact`, plus `reason`; empty sets on the scalar and meta commands. `show bgp rib protocol` has no declaration of its own, so the longest-prefix lookup hands it the fifteen names of `show bgp rib`. `forwardRibRoutes` forwards `validated.Tokens()` and `ctx.PeerSelector()` through `ForwardToPlugin`
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `showPipeline` calls `parsePipelineArgs(args)`; with no terminal it streams `sdk.Records{Key: "routes"}`; `count` answers `{"count": N}` from the iterator; `(*matchFilter).matches` reads the prefix, the peer, the family and the route attributes. `parsePipelineArgs` accepts words the declaration does not name: the scopes `sent` and `sent-received` (`scopeKeywords`), the stage `aspath` beside `path` (`filterASPath`), and the terminal `json` (`terminalKeywords`). It accepts a scope word at any position before the terminal and refuses a second one ("multiple route direction filters")
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `parseBestPipelineArgs` is the second argument reader, for `show bgp rib best`; it reads the same keyword maps and adds the `reason` terminal
- [ ] `internal/component/bgp/plugins/rib/rib_inject.go` - `showProtocolPipeline` is the third: `show bgp rib protocol <protocol>` hands the words after the protocol name to `parsePipelineArgs` and discards the scope it answers, because a protocol RIB holds received routes only
- [ ] `internal/component/lg/prefix_lookup.go` - `receivedCommand` builds `show bgp rib received` plus its filters as command words, the argument form
- [ ] `internal/component/bgp/plugins/rib/rib_topology.go` - `graphAnswerKey` documents `show bgp rib ... graph`, the argument form
- [ ] `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` - `rpc show` and `rpc best` each declare `leaf peer` and `leaf-list args`; both descriptions teach the argument words, the `json` terminal included
- [ ] `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` - `container rib` names the pipe operators in its description
- [ ] `internal/component/plugin/server/command.go` - `Dispatch` starts accounting on the raw input (the RFC 8907 Section 8.3 quote sits above it), calls `command.ScanExecution`, which refuses an unfinished quote in the command part, and when `pipeAt != noPipe` answers `pipeChainRefusal`; `BeginAccounting` masks the line with `config.DisplayCommand` and hands a typed accountant (`aaa.CommandArgsAccountant`) its tokens; `isAuthorized` hands a typed authorizer (`aaa.CommandArgsAuthorizer`) the first token as the command and the rest as arguments, and a string authorizer the whole input; `IsAuthorized` and `BeginAccounting` are exported for the stream route; `routeToProcess` sends `rpc.ExecuteCommandInput{Command, Args, Peer}`; `CommandContext` has no pipe field
- [ ] `internal/component/aaa/command_args.go` - `CommandArgsAuthorizer.AuthorizeCommandArgs` takes the username, the remote address, the command, its argument tokens, the peer and the read-only flag; `CanonicalCommand` rebuilds a string only for an authorizer that has no typed method
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `opDispatchCommand` reaches `Dispatch` with the plugin's line
- [ ] `internal/component/plugin/dispatch.go` - `CallerIdentity`; `CommandDispatcher.JSON` flattens with `ResponseJSON`; `CommandDispatcher.Answer` keeps a row generator for SSH
- [ ] `pkg/plugin/rpc/types.go` - `ExecuteCommandInput{Serial, Command, Args, Peer}`; `PipeDecl{Command, Name, Description, Expansion}` declares an alias and nothing else
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `ExecuteCommandHandler` takes serial, command, args and peer; 53 non-test files and 19 test files register `OnExecuteCommand` (recounted 2026-11-20)
- [ ] `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/web/handler_tools.go`, `internal/component/web/page_tools.go`, `internal/component/cli/model_mode.go`, `internal/test/cli/cmd_engine_steps.go` - split at the edge with the `ProcessPipes*` wrappers and dispatch the rewritten command part. The SSH exec stream arm calls `ProcessRemoteStreamPipes` and `RunStreamPipes` before the one-shot arm
- [ ] `internal/component/cli/client/main.go` - `execute` (`ze cli -c`) first tries `ServeLocal`, then sends the whole line, chain included, to the daemon over SSH exec, where `execMiddleware` splits it; `commandWithFormat` calls `HasFormatPipe`; `StreamMonitor` splits in the client with `ProcessStreamPipesDefaultFunc`, sends the command part and formats and saves each event itself; `runInteractiveSession` calls `sshclient.RunInteractive`, a shell on the daemon, so an interactive line is parsed by the daemon's model
- [ ] `internal/component/mcp/tools.go` - `ze_execute` and `run` call `s.dispatch.JSON` with the whole line
- [ ] `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`, `internal/component/api/engine.go` - `handleExecute` and `Execute` reach `APIEngine.Execute`, and the stream endpoints `APIEngine.Stream`; both call `prepareCommand` and judge the caller's read-only flag on the matched command's declaration and the authorizer on the built string
- [ ] `internal/component/api/invocation.go` - `prepareCommand` scans with `ScanExecution` and matches the command words before the pipe; `buildInvocation` writes parameters through `command.WriteInvocation`, which quotes a value holding a quote or a pipe so it stays one argument; when the input holds a pipe it appends the parameters after the whole input, so they land after the chain
- [ ] `internal/component/api/requests.go` - `ValidateParams` refuses whitespace in a parameter key or value; `BuildCommand` has no non-test caller
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
- `collectPipeMeta`, `validateKnownPipeArguments` and `validateCountDocumentFollowers` keep running before any operation is taken out of the chain, and the `pipeChainMeta` they answer still reaches `ApplyPipes`
- Every route the RIB reaches today stays reachable: the scopes `sent` and `sent-received`, the `aspath` spelling and the route output a `json` terminal asked for
- An unfinished quote in the command part is refused with its column (`ScanExecution`)
- A REST or gRPC parameter value holding a quote or a pipe stays one argument (`WriteInvocation`)

**Behavior to change:**
- No caller rewrites the line: the handler receives its declared operations as a structured list, and accounting records the line as typed
- A plugin's `dispatch-command`, MCP, REST and gRPC answer a piped line with the chain applied, on the one-shot route and on the stream route
- A local-data command and a monitor receive their declared operations the same way
- A pipe character or a space inside double quotes is an argument character everywhere on the line, the chain included; an unfinished quote in the chain is refused with its column
- A declared operation is taken from the chain only while no residual operation precedes it
- The RIB's argument grammar stops being a carrier, every word it accepted is a declared operation, and every in-tree user is converted
- A plugin can declare operations its own handler runs, through the `Pipes` declaration it already sends
- A REST or gRPC parameter is written before the chain of a piped line
- `show command help` and `ze help command --json` answer one `pipe-operations` list, and the completer offers exactly that list

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The one registry of a command's own operations | `internal/component/command/pipe_filter.go` `pipeFilterRegistry` (a `declarationRegistry`), filled by `RegisterPipeFilters`, read by `lookupPipeFilters` | Declares, per command path by longest prefix, the operation names a command accepts; empty sets are barriers | extends: it stays the ONE registry of declared operations. In-tree commands keep filling it with `RegisterPipeFilters`, which keeps its `BUG:` panic. A plugin's declared operations enter through a second entry point on the same registry that returns an error, records under the plugin's name and refuses the whole batch on any fault, as `RegisterPluginAliases` does for aliases; its withdrawal runs at the sites that call `UnregisterPluginAliases` today (the two rollback arms in `startup.go`, the cleanup in `restart.go`). `Leading` goes with the fold that read it: the handler receives the list in chain order, and a command that needs an order states it in its handler (the RIB accepts a scope at any position). `TakesArg` is replaced by one argument field holding a catalog argument kind (`none`, `optional`, `text`, `count`, `fields`, `path`), the same vocabulary a plugin's `PipeDecl` `argument` field uses, so the partition validates a builtin's and a plugin's operation by one rule |
| A plugin's pipe declaration | `pkg/plugin/rpc/types.go` `PipeDecl` and the Stage 1 `Pipes` field (closed spec-plugin-registers-pipe-operations, `80506f5dbd`); `internal/component/plugin/server/startup.go` `validatePipeDecls`, `registerPluginPipes`; `internal/component/command/alias.go` `RegisterPluginAliases`, `UnregisterPluginAliases`, `aliasBarriers` | A `PipeDecl` names a command, a name, a description and an expansion, recorded as an alias by owner | extends: `PipeDecl` gains one explicit `argument` field. Its presence marks an operation the plugin's handler runs, and its value states the argument contract; an entry carries exactly one of `expansion` and `argument`, and `validatePipeDecls` refuses an entry with both or with neither, so the guard "expands to nothing" stays for a declaration that states nothing. An operation is recorded by `registerPluginPipes` into `pipeFilterRegistry` under the same owner, with the same barriers and withdrawal. Still one `Pipes` list; `./le plugin declarations check` judges both kinds |
| The per-command operator join | `internal/plugins/help/help_command.go` `operatorsFor`, and its copy in `internal/le/cli/catalog/catalog.go` | Joins `command.PipeOperatorCatalog()` with the declared shape and address fields and answers each operator's availability | extends: the join moves into `internal/component/command` as the one introspection function, which also folds in the command's declared operations and aliases; `operatorsFor` and the copy are deleted, and every reader calls the function |
| Generic operators and their classes | `internal/component/command/pipe_catalog.go` `PipeOperator`, `PipeClass`; `docs/architecture/cli/command-verbs.md` operator classes | The one list of generic operators, each with its class, argument kind and the shapes it acts on | extends: each entry also states whether a command may override it and, when it may, the answer shape an override is held to. `foldFilters` holds that fact today as a four-case switch (`pipeMatch`, `pipeCount`, `pipeFirst`, `pipeLast`); the switch goes and the partition, the registration check and the introspection read the catalog entry. Each entry also states whether the operation is presentation only (the formats, paging and `save`), the fact the rendering key's suffix is found from. `Renders` (formats) and `LocalOnly` (`save`) hold parts of it today; `no-more` is in neither |
| Precedence of a command's name over a generic one | `internal/component/command/pipe.go` `foldFilters` with `lookupPipeFilters` and `lookupPipeOperatorByKind` | A generic kind (`match`, `count`, `first`, `last`) is taken by the command when its set holds the name and no format came earlier; nothing checks meaning | keeps: name-based precedence and the longest-prefix lookup. replaces: the fold, and its `formatSeen` gate, which lets a declared operation jump ahead of any residual operation that is not a format (`show bgp rib \| resolve \| count` counts before it resolves). The gate becomes a prefix rule: a declared operation is taken only while no residual operation precedes it. extends: a declared name the catalog holds is an OVERRIDE, allowed only where the catalog entry says so, held to the generic's argument contract when recorded and to its answer shape when it answers |
| Command help | `internal/plugins/meta/cmd/help.go` `commandHelp`, `pipeFilterHelp`, `pipeAliasHelp` | `show command help <name>` answers `pipe-filters` and `pipe-aliases` | extends: answers `pipe-operations` from the introspection function; the two keys and their helpers go |
| Published command catalog | `internal/plugins/help/help_command.go` `commandEntry` (`ze help command --json`); readers `internal/le/cli/catalog`, `internal/le/doc/yangcontract`, `internal/le/site` | Publishes `operators`, `pipes` and `pipe-aliases` | extends: publishes `pipe-operations` from the same function; the three keys go and every reader converts |
| Pipe completion | `internal/component/command/completer.go` `completePipe`, `pipeExtras`, `aliasSuggestions`, `filterSuggestions` | Offers every catalog operator, then the command's aliases and filters | replaces the listing: `completePipe` reads the introspection function; argument completion is unchanged |
| Answer shape declaration | `internal/component/command/answer_shape.go` `RegisterShape`, `ShapeForCommand` | States whether a command answers rows or one document | extends: the introspection reads it, as `operatorsFor` does. Set aside as the override's shape carrier: it is the command's shape, not one operation's answer |
| The fold, and the dispatcher's fold | `internal/component/command/pipe.go` `foldFilters`, `FoldPipeChain`, `HasFormatPipe`; `render_records.go` `RenderRecords`; `pipe_records.go` `applyPipesRecords`; `render_counted_document.go` `prepareDocumentPipes` | Rewrite owned operations into command words; eight call sites of `foldFilters` in the package | replaces: one partition function returning the typed command part, the declared list, the residual chain and the `pipeChainMeta`; every call site above calls it; `FoldPipeChain` deleted. keeps: `collectPipeMeta`, `validateKnownPipeArguments` and `validateCountDocumentFollowers`, which the partition runs first as the fold does |
| Chain parse | `internal/component/command/pipe.go` `ParsePipe`, `splitPipeAt`, `parsePipeOps`, `parsePipeChain`, `parseExecutionPipeChain`, `expandAliases`, `bindAddressFields` | A quote-aware first cut, then a quote-blind split of the chain into segments read by `strings.Fields`; alias expansion; address binding | replaces the chain's separate grammar with the one defined in "The chain grammar"; expansion and binding unchanged and also run for every data caller |
| Quote-aware scan | `internal/component/command/invocation_scan.go` `ScanInvocation`, `ScanExecution`; `internal/component/plugin/server/command.go` `scanCommand`, `tokenize` (wrappers over it) | Decoded command words and the offset of the first pipe outside quotes; an unfinished quote refused in the command part | extends: the scan continues past the first pipe and answers each operation's name and decoded argument words; the unfinished-quote refusal covers the whole line |
| Stream route | `internal/component/plugin/server/handler.go` `GetStreamingHandlerForCommand`, `StreamingHandler`, and its four callers: `cmd/ze/hub/server_stream.go` `serverStreamCommand` (web); `cmd/ze/hub/api.go` `apiStreamSource` (REST and gRPC, under `APIEngine.Stream`); the `SetStreamingExecutorFactory` closure in `cmd/ze/hub/service_ssh.go`, fed by the stream arm of `internal/component/ssh/ssh.go` `execMiddleware`; `internal/component/cli/model_monitor.go` `extractMonitorCmdArgs` | Each site accounts and authorizes outside `Dispatch` and looks the handler up; the web site and the SSH arm split first and run the handler under `RunStreamPipes`; the REST and gRPC site splits nothing | extends: `GetStreamingHandlerForCommand` takes the typed line, calls the partition and attaches the declared list to the `ValidatedArgs` it answers, so all four sites get it from one function and the handler signature and every registration stay. `execMiddleware` passes the typed line to the `StreamingExecutor`. The REST and gRPC site gains `RunStreamPipes` |
| Local-data route | `internal/component/command/local_data.go` `ServeLocal`, `RenderLocalAnswer`, `ProcessPipesDefaultFormatLocal`; the daemon side `plugin/server` `loadLocalData` | Answers in the caller's process, or in the daemon through the same handler | extends: `ServeLocal` calls the partition and hands the declared list in the same `ValidatedArgs` value; the daemon side reaches it through `Dispatch`. `RenderLocalAnswer` is handed a path, never a line, so it carries no chain and is unchanged |
| A client statement across SSH | `internal/component/cli/sshclient/terminal.go` `EnvConfigName`, `EnvCLIMode`, `session.Setenv`; `internal/component/ssh/session.go` `parseSessionRequest` | The interactive client names its config and mode in the session environment | extends: one more key by which a `ze cli` client states that it renders the residual chain, read on the exec channel as well |
| Typed authorization and accounting | `internal/component/aaa/command_args.go` `CommandArgsAuthorizer`; `internal/component/plugin/server/command.go` `isAuthorized`, `isAuthorizedCommandArgs`, `BeginAccounting`, `beginAccounting`, which read their tokens from `tokenize`; the six typed wrappers and `liveAAABundleAccountant` in `cmd/ze/hub/aaa_lifecycle.go`; `fallbackArgs` in `internal/component/tacacs/authorizer.go` | A typed authorizer or accountant receives tokens, a string one the line. `isAuthorized` denies a line `tokenize` refuses, and `BeginAccounting` records such a line as one token | extends: the accountant receives the typed line and its tokens, chain included (not a choice, RFC 8907 Section 8.3). Under T-6 A the typed authorizer receives the chain as tokens after the arguments, for authorization only. Both read their tokens from the chain-grammar scan the partition uses, never from a second invocation scan, which refuses a chain backslash (AC-38). The wrappers and `fallbackArgs` pass tokens through unchanged and are tested with chain tokens |
| Dispatcher refusal | `internal/component/plugin/server/command.go` `Dispatch` (the `pipeAt != noPipe` arm), `pipeChainRefusal`, `zeCLIForm` | Refuses any unsplit chain | replaces: `Dispatch` accepts the chain; the refusal and its tests go |
| Chain formatter | `internal/component/command/pipe.go` `ProcessPipesDefaultFormatChecked`, `ProcessPipesDefaultFormatLocal`, `processStreamPipes`, `ProcessStreamPipesDefaultFunc`, `ApplyPipes` | Split, fold, validate per surface, and a formatter | extends: return the typed command part and the residual chain; `ApplyPipes` also runs inside the dispatcher for a data caller |
| Plugin command carrier | `pkg/plugin/rpc/types.go` `ExecuteCommandInput`; `internal/component/plugin/server/command.go` `routeToProcess`; `pkg/plugin/sdk/sdk_callbacks.go` `ExecuteCommandHandler`, `executeCommandAnswer` | Carries command, args and peer | extends: a pipe list beside the args on the wire, delivered to the SDK handler; pre-release, no compatibility path |
| Handler context and shared dispatcher | `internal/component/plugin/server/command.go` `CommandContext`; `cmd/ze/hub/main_servers.go` `serverDispatcher`; `internal/component/plugin/dispatch.go` `CallerIdentity` | Carry identity and surface into `Dispatch` | extends: `CommandContext` carries the declared list; `CallerIdentity` carries the rendering statement for an in-process caller, and the SSH session environment carries it for a client in another process |
| Split-then-format surface | `internal/component/web/cli_terminal.go` `executeTerminalOperationalResult`, `internal/component/ssh/ssh.go` `execMiddleware`, `internal/component/cli/model_mode.go` `executeOperationalCommand`, `internal/component/cli/model_keys.go` `processStreamPipes`, `internal/component/cli/client/main.go` `StreamMonitor` | Split at the edge, dispatch the command part, format | extends (T-2 A): they send the typed line and state that they render the residual chain |
| RIB pipeline and its model | `internal/component/bgp/plugins/rib/rib_pipeline.go` `showPipeline`, `parsePipelineArgs`; `rib_pipeline_best.go` `parseBestPipelineArgs`; `rib_inject.go` `showProtocolPipeline`; `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` `rpc show` and `rpc best`, each with `leaf-list args` | Three readers take scope, filters and one terminal from args, declared as a YANG argument on two rpcs | extends the pipeline: stages built from the structured list by all three readers. replaces the argument reading and both leaf-lists |
| The peer-scoped RIB command | `internal/component/bgp/plugins/cmd/rib/rib.go` `cmdBgpPeerRib` (`ze-bgp:peer-rib`, `forwardRibRoutes` into `cmdRibShow`); `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` `container rib`, `leaf scope` | `show bgp peer <selector> rib <scope>` reaches `parsePipelineArgs` with the scope as a validated YANG leaf; the path declares a shape, columns and address fields of its own and no pipe set | extends: the path declares its own operation set in `registerPipeFilters`, the names of `show bgp rib`, scope included. replaces: `leaf scope` and its container go, and the scope is a declared operation |
| REST and gRPC parameter tokens | `internal/component/api/invocation.go` `prepareCommand`, `buildInvocation`; `internal/component/command` `WriteInvocation`; `internal/component/api/requests.go` `ValidateParams`, `BuildCommand` | Turn structured parameters into quoted tokens; append them after the whole input when it holds a pipe | extends: parameters are written at the pipe offset, before the chain. `WriteInvocation`'s quoting already keeps a quote or a pipe in a value as data, so no new refusal is added. deletes: `BuildCommand`, which has no non-test caller |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator line on SSH exec, the web CLI terminal, `ze cli`, MCP, REST execute, gRPC `Execute`; a plugin's `dispatch-command`; the functional-test engine steps
- `show command help <name>`, `ze help command --json`, and `|` followed by Tab

### Transformation Path
1. The caller sends the line as typed. A rendering surface also runs the partition function on it to build its formatter. A REST or gRPC caller's parameters are written into the line at the pipe offset, before the chain
2. The route starts accounting on the line as typed (secret-masked as today): `Dispatch` on the one-shot route; on the stream route `serverStreamCommand`, `apiStreamSource` and the SSH exec closure in `service_ssh.go`, which receives the typed line from `execMiddleware`. The accountant's and the authorizer's tokens come from the scan of step 3
3. The line is scanned once with the one grammar, command part and chain; an unfinished quote anywhere is refused with its column
4. The one partition function in `internal/component/command` runs the argument and count-follower validation, expands aliases, binds address fields, then walks the chain in order. While no residual operation has been met, an operation `pipeFilterRegistry` holds for the command part joins the declared list. From the first residual operation on, everything stays residual: a generic-named operation runs as the generic, and an operation only the command declares is refused by name with the reordered line as its fix. An unknown operation on a command that declares a set is refused (`unknownFilterError`)
5. The command part is resolved, validated and authorized as any line without a chain. The authorizer sees the chain as T-6 decides
6. The declared list, each entry a name and its decoded argument words, reaches the handler: `CommandContext` for a builtin; the pipe list of `ExecuteCommandInput` for a plugin process; the `ValidatedArgs` value for a streaming handler and a local-data handler
7. The handler computes with it and answers data; an override's answer is checked against the generic's shape by the command package's answer step, which `Dispatch`, `ServeLocal` and the stream sites all run
8. The residual chain runs on the answer: by a rendering surface inside the daemon when it said it renders; otherwise by the engine through `ApplyPipes` with the partition's `pipeChainMeta`, inside `Dispatch` on the one-shot route and through `RunStreamPipes` on the stream route. A `ze cli` client that set the rendering key takes over the presentation suffix only. A format operation is rendered and its text carried as the answer's data
9. Introspection: one function in `internal/component/command` answers a command's pipe operations; `commandHelp`, `commandEntry` and `completePipe` read it

### The carrier on each route

| Route | Who calls the partition | Carrier of the declared list | Who runs the residual chain | How the caller says it renders |
|-------|-------------------------|------------------------------|-----------------------------|-------------------------------|
| One-shot, builtin handler | `Dispatch` | `CommandContext` | the engine, or the surface that said it renders | `CallerIdentity`, in-process |
| One-shot, plugin handler | `Dispatch` | the pipe list of `ExecuteCommandInput`, over the bridge or forked TLS RPC | same | same |
| One-shot, local-data handler in the daemon | `Dispatch`, through `loadLocalData` | `ValidatedArgs` | same | same |
| Stream (`monitor ...`), every surface | `pluginserver.GetStreamingHandlerForCommand`, the one function all four handler-lookup sites call: `cmd/ze/hub/server_stream.go` `serverStreamCommand` (web), `cmd/ze/hub/api.go` (REST and gRPC streams), the `SetStreamingExecutorFactory` closure in `cmd/ze/hub/service_ssh.go` (SSH exec), `internal/component/cli/model_monitor.go` `extractMonitorCmdArgs` (the daemon-hosted model). It partitions and attaches the declared list to the `ValidatedArgs` it answers | `ValidatedArgs` | the engine, through `RunStreamPipes` | not stated: the engine renders |
| Stream over SSH exec | as above. `execMiddleware` passes the TYPED line to the `StreamingExecutor`, not the command part it passes today, so the closure in `service_ssh.go` accounts and authorizes what was typed | `ValidatedArgs` | the engine; a `ze cli` client may take over only the presentation suffix (below) | a session environment key beside `EnvCLIMode`, read by `parseSessionRequest` |
| Local-data in the caller's process (`ze <verb>`, `ze cli -c`) | `ServeLocal` | `ValidatedArgs` | `ServeLocal`, in the same process | no boundary is crossed |
| `ze cli -c` one-shot to the daemon | `Dispatch`, reached through `execMiddleware` | as the one-shot rows | the daemon | not stated: the daemon renders, as today |
| `ze cli -c` monitor (`StreamMonitor`) | `GetStreamingHandlerForCommand` in the daemon; the client reads only the catalog, to find the presentation suffix | `ValidatedArgs`, in the daemon | the daemon, for everything before the presentation suffix; the client, for the suffix: the event formatter, paging and `\| save` | the session environment key |

A client in another process holds no `CallerIdentity`: that value is built by
the transport inside the daemon. The statement therefore crosses the SSH
boundary the way the interactive client's mode already does, as a session
environment key the server validates. Its absence means the daemon renders, so
a plain `ssh` client and a client that forgets the key get the chain applied.

**The key hands over presentation only, never selection.** The SSH server
accepts any environment key from any client, so the key is untrusted input: a
client that could switch off the residual chain could skip a `| match` the
authorizer judged, and read rows a policy that allows only the filtered line
never granted. So the daemon ALWAYS applies every residual operation that
selects or reduces (`match`, `count`, `first`, `last`, `display`, `resolve`,
`origin`, and every declared operation, which runs in the handler anyway). The
catalog gains one fact per entry, whether the operation is presentation only:
the formats, paging and `save`. With the key set, the client takes over the
PRESENTATION SUFFIX of the chain, the longest trailing run of presentation
operations, and nothing else; the daemon applies everything before it and
answers data for the client to format, page and save.

| Line, with the key set | The daemon applies | The client applies |
|------------------------|--------------------|--------------------|
| `monitor x \| match up \| save f` | `match` | the default format, `save` |
| `monitor x \| match up \| table \| save f` | `match` | `table`, `save` |
| `monitor x \| json \| match up` | `json`, then `match` over the rendered lines | nothing: no trailing presentation run |
| `monitor x \| top-talkers \| save f` (a plugin's operation the client has never heard of) | `top-talkers`, in the handler | the default format, `save` |
| `monitor x \| brief \| save f` (`brief` an alias, whatever it expands to) | the whole expansion of `brief`, a format in it included | the default format over what the daemon answered, `save` |

Both sides find the suffix from the catalog alone, which is compiled into
both, and on the names as typed, before any alias is expanded. A name that is
not a presentation operator of the catalog ends the suffix, an alias included:
the daemon applies an alias's whole expansion. Were the suffix found after
expansion, the daemon would leave a format an alias expands to for a client
that never heard of the alias, and nobody would render it. A name the client
does not know is by definition not a presentation operation, so the client
never refuses it and never needs the daemon's registries: the daemon
classifies, the client renders (R-10). The key is honoured on the exec stream route only: on
one-shot exec and on the interactive session it changes nothing, because the
daemon renders there. A value other than the one defined is refused wherever
it is sent, as the two existing keys are. `| save` is never run by the daemon
for any caller.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator surface ↔ daemon dispatcher | the typed line through `serverDispatcher`, with the rendering statement on `CallerIdentity` | No |
| Plugin ↔ engine (`dispatch-command`) | the typed line in `DispatchCommandInput` | No |
| Engine ↔ plugin (`execute-command`) | the declared list in `ExecuteCommandInput`, over the internal bridge or forked TLS RPC | No |
| Plugin Stage 1 (`declare-registration`) | a declared operation as a `PipeDecl` with an `argument` field and no expansion, in the existing `Pipes` list | No |
| Daemon or catalog ↔ help reader | the `pipe-operations` list | No |
| `ze cli` client ↔ daemon (SSH exec) | the typed line, and the rendering statement as a session environment key | No |
| Daemon ↔ AAA backend | the typed line to the accountant; under T-6 A the chain as tokens after the arguments to a typed authorizer | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms `ProcessPipes`, `ProcessStreamPipes`, `ParsePipe(`, `foldFilters`, `FoldPipeChain`, `pipeChainRefusal`, `ExecuteCommandInput{`, `OnExecuteCommand(`, `DispatchCommandInput`, `PipeDecl`, `registerPluginPipes`, `pipe-filters`, `pipe-aliases`, `json:"pipes`, `json:"operators`, `with-rows`, `HasFormatPipe(`, `RunStreamPipes(`, `AuthorizeCommandArgs`, a regular expression for an argument-form RIB line with a filter or terminal word, and the scope-word form `bgp rib( best| protocol [a-z0-9-]+)? (received|advertised|sent|sent-received)\b` (extended regular expression, files listed); positive controls `internal/component/ssh/ssh.go` `execMiddleware`, `internal/component/plugin/server/command.go` `routeToProcess`, `internal/component/plugin/server/startup.go` `registerPluginPipes`, `internal/plugins/meta/cmd/help.go` `commandHelp`, `internal/plugins/help/help_command.go` `operatorsFor`, and for the scope-word form `internal/component/lg/prefix_lookup.go` `receivedCommand`. Grep output of 2026-11-12 for the first set of terms and of 2026-11-19 for the scope-word form, re-derived at phase 1. Added on 2026-11-20: the peer-scoped form `bgp peer \S+ rib (sent|advertised|received|sent-received)` over `internal`, `test`, `cmd` and `pkg` (three files, positive control `internal/component/plugin/server/usage_model_test.go`); the `json` terminal as an argument, `bgp rib` followed by words and `json` with no pipe before it, over the same trees (two files); the streaming-handler lookups `GetStreamingHandlerForCommand(`, `IsAuthorized(` and `BeginAccounting(` over `cmd` and `internal` (positive control `cmd/ze/hub/server_stream.go`).

The scope-word form on 2026-11-19: 99 files over `internal`, `test`, `docs`, `cmd` and `pkg` (the main session's count). 82 of them are outside `docs/`: `internal/test/fixture` 21, `test/plugin` 12, `internal/component/lg` 12, `internal/component/bgp/plugins/rib` 11, `internal/le/interoplab` 6, `test/reload` 4, `internal/component/bgp/plugins/cmd` 4, `internal/exabgp/bridge` 3, `internal/le/site` 2, `internal/component/plugin` 2, `internal/component/bgp` 2, `test/interop` 1, `internal/plugins/help` 1, `internal/component/command` 1. The 17 under `docs/` were re-derived on 2026-11-20 by one recursive file-name search of `docs` for the scope-word form and the peer-scoped form together, and are named in the writer row below. Two limits of that search are stated, not hidden: the derived page `docs/features/rfc-status.md` could not be built that day (another session's edit to an RFC summary), so it was absent and not searched; and the hook refused the follow-up search that would have printed the matching lines, so which of the 17 pages hold the peer-scoped form, and which hits belong to another command, is read at phase 1. Not every hit is a RIB line: `docs/architecture/exabgp-bridge.md` and `docs/architecture/api/update-syntax.md` are in the 17 and may match on another command's words, as the bridge sources below do. `internal/component/bgp/plugins/cmd/peer/advertised.go`, its YANG and tests, the two `internal/component/bgp/reactor/reactor_api_advertised*` files and the three `internal/exabgp/bridge` files match on another command's words and are read at phase 1 before any is changed.

A test in the census that carries an `RFC requirement:` tag owes a re-taken proof record in the commit that changes its line (`./le rfc record`, `ai/rules/rfc-compliance.md`); `internal/component/plugin/server/rfc8907_command_test.go` is in the census and is checked for tags first.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Operator line with a chain | writer | SSH clients, the web terminal page, `ze cli`, MCP clients, REST and gRPC clients, any plugin sending `dispatch-command`, `internal/test/cli/cmd_engine_steps.go` `pipedEngineDispatch`, `internal/le/le/root/leroot.go`, `internal/component/command/local_data.go` | grep of `ProcessPipes*` call sites | preserve the text; every reader now honors it |
| Operator line with a chain | reader | `internal/component/ssh/ssh.go` `execMiddleware`; `internal/component/web/cli_terminal.go`; `internal/component/web/handler_tools.go`; `internal/component/web/page_tools.go`; `internal/component/cli/model_mode.go`; `internal/component/cli/client/main.go` `execute`; `internal/component/mcp/tools.go`; `internal/component/api/engine.go` `Execute`; `internal/component/plugin/server/dispatch_registry.go` `opDispatchCommand`, all into `Dispatch` | read at each site | change: parse once, quote-aware; `Dispatch` accepts the chain |
| Operator line with a chain | reader | the routes that never reach `Dispatch`. The four streaming-handler lookups through `internal/component/plugin/server/handler.go` `GetStreamingHandlerForCommand`: `cmd/ze/hub/server_stream.go` `serverStreamCommand` (the web), `cmd/ze/hub/api.go` `apiStreamSource` (REST and gRPC, called by `internal/component/api/engine.go` `Stream`), the `SetStreamingExecutorFactory` closure in `cmd/ze/hub/service_ssh.go` (fed the command part by the stream arm of `internal/component/ssh/ssh.go` `execMiddleware`), and `internal/component/cli/model_monitor.go` `extractMonitorCmdArgs`. Then `internal/component/command/local_data.go` `ServeLocal`; `internal/le/le/root/leroot.go`; `internal/component/cli/client/main.go` `StreamMonitor`; `internal/component/cli/model_keys.go` `processStreamPipes`. `RenderLocalAnswer` is not one: it is handed a path, not a line | grep of `ProcessPipes*`, `ProcessStreamPipes*`, `ProcessRemoteStreamPipes`, `RunStreamPipes`, 2026-11-19; grep of `GetStreamingHandlerForCommand(`, 2026-11-20, four non-test callers, each read | change: the four lookups get the partition and the declared list from `GetStreamingHandlerForCommand`; the others call the one partition; each carries the declared list as "The carrier on each route" states |
| Operator line with a chain | reader | chain readers that only inspect: `internal/component/command/pipe.go` `HasFormatPipe` (callers `internal/component/cli/client/main.go` `commandWithFormat`, `internal/component/cli/client/answer.go` `daemonOutput.end`); `internal/component/cli/model_ping.go` and `model_traceroute.go` (`ParsePipe`); `internal/component/command/render_records.go` `RenderRecords`; `pipe_records.go` `applyPipesRecords`; `render_counted_document.go` `prepareDocumentPipes`; `cmd/ze/ze_core_pipe.go` (`ze pipe`, the operators over stdin with no command) | same grep | change: each reads through the partition or the one grammar; `ze pipe` takes the chain grammar and has no declared operation to hand |
| RIB argument-form command line | writer | `internal/component/command/pipe.go` `foldFilters`, through every `ProcessPipes*` wrapper and the three record and document renderers | `foldFilters` builds it with `textbuf` | replace: never built |
| RIB argument-form command line | writer | the scope-word form, by tree, from the 2026-11-19 search: product, `internal/component/lg/prefix_lookup.go` `receivedCommand`, `handler_api.go`, `handler_ui.go`, `internal/plugins/help/help_ai.go`, the comments in `internal/component/bgp/plugins/rib/rib_topology.go`, and `rib_pipeline_covering.go`, the comment in `internal/component/bgp/plugins/bmp/state.go` (`show bgp rib protocol bmp peer <key>`, found by the review, not by the scope-word search), the descriptions in `internal/component/bgp/plugins/rib/yang/ze-rib.yang` and the comment in `internal/component/command/pipe.go`; interoplab, 6 files under `internal/le/interoplab`; fixtures, 21 files under `internal/test/fixture`; `.ci`, 12 under `test/plugin` and 4 under `test/reload` (`bgp-dynamic-filter-swap-live.ci`, `bgp-filter-aspath-reload.ci`, `bgp-filter-prefix-definition-reload.ci`, `match-condition-definition-reload.ci`); the interop scenario `test/interop/scenarios/bgp-paths-limit-receive-bird/ze.conf`; unit tests, 9 under `internal/component/lg`, 8 under `internal/component/bgp/plugins/rib`, `internal/component/plugin/server/rfc8907_command_test.go` and `plugin_rpc_test.go`; site test data, `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json`; docs, 17 files: `docs/architecture/api/birdwatcher-compat.md`, `commands.md`, `ipc_protocol.md`, `process-protocol.md` and `update-syntax.md` (all under `docs/architecture/api/`), `docs/architecture/exabgp-bridge.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/testing/interop.md`, `docs/features/api-commands.md`, `docs/features/mcp-integration.md`, `docs/guide/cli.md`, `command-reference.md`, `graceful-restart.md`, `looking-glass.md`, `monitoring.md` and `route-injection.md` (all under `docs/guide/`), and `docs/plugin-development/handlers.md` | the scope-word search, 2026-11-19; the `docs` file-name search, 2026-11-20; `receivedCommand` read | convert every one to the pipe form in the same work, one phase per tree; the site test data is regenerated from its source, never edited by hand |
| RIB argument-form command line | writer | in-tree senders typed by hand: `internal/component/lg/handler_api.go`, `internal/component/lg/handler_ui.go`, `internal/component/lg/handler_graph.go`, `internal/component/api/rest/server.go` (convenience routes), `internal/le/interoplab/bgp/check_extras.go`, `internal/le/interoplab/bgp/check_special.go`, `internal/le/interoplab/bgp/check_rfc4456.go`, `internal/le/interoplab/bgp/check_route_server_replacement.go`, `internal/le/interoplab/bgp/check_rpki_reload.go`, `internal/plugins/help/help_ai.go` `printRIBPipeline`, `internal/test/fixture/plugin_fixture_02.go`, `internal/test/fixture/plugin_fixture_05.go`, `internal/test/fixture/plugin_fixture_11.go`, `internal/test/fixture/plugin_fixture_12_prefix.go`, `internal/test/fixture/plugin_fixture_13_rib.go`, `internal/test/fixture/plugin_fixture_15_show.go`, `internal/test/fixture/plugin_fixture_16.go` (`plugin16PipeFirstLast`), the comments in `internal/component/bgp/plugins/rib/rib_topology.go`, and the `.ci` bodies the search hits in `test/plugin/` (`show-rib-under-load.ci`, `prefix-warn-only-drops-nlri.ci`, `lg-best-table.ci`, `filter-wire-batch.ci`, `rib-pipe-filter.ci`, `test-pipe-first-last.ci`) | grep output, 2026-11-12 | convert every one to the pipe form in the same work |
| RIB argument-form command line | reader | `internal/component/bgp/plugins/cmd/rib/rib.go` `forwardRibRoutes`, `forwardRibBest` → `ForwardToPlugin` → the three readers `internal/component/bgp/plugins/rib/rib_pipeline.go` `parsePipelineArgs`, `rib_pipeline_best.go` `parseBestPipelineArgs` and `rib_inject.go` `showProtocolPipeline`, reached through the command table in `rib_commands.go`; the walk in `rib_pipeline_covering.go` that `covering` and `exact` select; both YANG `leaf-list args` (`rpc show`, `rpc best`), which the REST and MCP parameter routes read | read | replace: stages from the structured list in all three; a scope, filter or terminal word typed as an argument is refused by the engine's argument validator, by name, with the pipe form as its fix, and never reaches the RIB; both leaf-lists go |
| RIB argument-form command line | writer | the `json` terminal typed as an argument: `internal/plugins/help/help_ai.go` `printRIBPipeline` (`show bgp rib best family ipv4/unicast json`, and "json Full route details (default)") and `internal/component/bgp/plugins/rib/rib_pipeline_best_age_test.go` `TestBestJSONTerminalRowCarriesNoAge` | the `json`-argument search, 2026-11-20, two files | replace: the help text teaches the pipe form; the test goes with the terminal it tests, judged by `ze-test-judge` (D-7) |
| RIB argument-form command line | writer | the RIB's own refusals: `refusePipelineArgs` is called with `show bgp rib count` and `show bgp rib best count` as the fix line (`rib_pipeline.go` `showPipeline`, `rib_pipeline_best.go` `bestPipeline`) | read, 2026-11-20 | replace: every fix line the RIB prints is the pipe form |
| Peer-scoped RIB line `show bgp peer <selector> rib <scope>` | writer | `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go` (`rib received`, line 226), `internal/le/interoplab/bgp/check_ospf_redistribute_default_diagnostics_test.go` (`rib advertised`, line 40), `internal/component/plugin/server/usage_model_test.go` (`rib received`, line 554); an operator, and any script, typing the scope after `rib`; the pages among the 17 above that hold the form, read at phase 1 | the peer-scoped search, 2026-11-20: three files outside `docs/` | convert each to `show bgp peer <selector> rib \| <scope>`, in the phase of its tree; the fixture keeps its expected answer |
| Peer-scoped RIB line `show bgp peer <selector> rib <scope>` | reader | `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` `container rib`, `container scope`, `leaf scope` (the argument validator and completion read it); `internal/component/bgp/plugins/cmd/rib/rib.go` `forwardRibRoutes` under `ze-bgp:peer-rib`, which forwards the validated tokens to `cmdRibShow`; `internal/component/bgp/plugins/rib/rib_pipeline.go` `parsePipelineArgs`, `scopeKeywords`; `internal/component/bgp/plugins/rib/yang_enum_agreement_test.go` `TestRIBScopeKeywordsMatchTheYANGModel`, which reads the enumeration | read, 2026-11-20 | replace: the leaf and its container go; `show bgp peer rib` declares its own operation set; the agreement test is replaced by one holding `scopeKeywords` to the declared scope operations (test-judge); the argument form is refused (AC-34) |
| `ze-plugin-callback:execute-command` JSON (`ExecuteCommandInput`) | writer | `internal/component/plugin/server/command.go` `routeToProcess`; `internal/component/plugin/server/subsystem.go`; `internal/component/plugin/server/system.go` (completion request); `internal/test/pluginengine/pluginengine.go` | grep `ExecuteCommandInput{` | change: `routeToProcess` writes the pipe list; the other three send none |
| `ze-plugin-callback:execute-command` JSON (`ExecuteCommandInput`) | reader | `pkg/plugin/sdk/sdk_callbacks.go` `executeCommandAnswer`; every plugin outside the repository, in any language, written against `docs/architecture/api/process-protocol.md` | read | change: the SDK delivers the list; pre-release, a plugin built on the old SDK is rebuilt |
| Stage 1 `pipes` entry (`PipeDecl`) | writer | a plugin's `sdk.Registration` `Pipes`; in-tree plugins that declare aliases today | read `PipeDecl` | change: an entry carries exactly one of `expansion` and the new `argument` field; an alias entry is unchanged |
| Stage 1 `pipes` entry (`PipeDecl`) | reader | `internal/component/plugin/server/startup.go` `validatePipeDecls`, `registerPluginPipes`; `./le plugin declarations check` | read | change: an entry with `argument` is recorded into `pipeFilterRegistry` under the plugin's name; one with both fields or neither is refused |
| SSH session environment of a `ze cli` client | writer | `internal/component/cli/sshclient/terminal.go` (`EnvConfigName`, `EnvCLIMode`); new: the exec and stream calls of `internal/component/cli/client/main.go` `StreamMonitor` | `session.Setenv` call sites | change: one more key, the rendering statement |
| SSH session environment of a `ze cli` client | reader | `internal/component/ssh/session.go` `parseSessionRequest`; an SSH client outside the repository sets none | read | change: the key is validated like the two others and read on the exec channel; absent means the daemon renders |
| Typed authorization tokens | writer | `internal/component/plugin/server/command.go` `isAuthorized`, `isAuthorizedCommandArgs` | read | change under T-6 A: the chain follows the arguments as tokens |
| Typed authorization tokens | reader | `internal/component/tacacs/authorizer.go` (`AuthorizeCommandArgs`, and `fallbackArgs` on a server error), `internal/component/authz/register.go` (`StoreAuthorizer`, `boundStoreAuthorizer`), `internal/component/aaa/login_profiles.go` (`acceptedGenerationAuthorizer`); the six wrappers in `cmd/ze/hub/aaa_lifecycle.go` that sit in front of them (`liveLocalAuthorizer`, `liveLocalProfileAuthorizer`, `acceptedLocalGenerationAuthorizer`, `acceptedLocalGenerationProfileFallback`, `liveAAABundleAuthorizer`, `liveAAABundleProfileAuthorizer`); the operator's TACACS+ server and its per-command policy, outside the repository | grep `AuthorizeCommandArgs`, 2026-11-19 and 2026-11-20, every implementer read | change under T-6 A: each receives the chain tokens. The wrappers and `fallbackArgs` pass the tokens on or flatten them through `aaa.CanonicalCommand`, which quotes a token holding whitespace, a quote or a backslash, so a chain argument keeps its boundary; preserve under T-6 B |
| Typed accounting tokens | writer | `internal/component/plugin/server/command.go` `BeginAccounting`, `beginAccounting` | read | change: tokens from the chain-grammar scan, chain included |
| Typed accounting tokens | reader | `cmd/ze/hub/aaa_lifecycle.go` `liveAAABundleAccountant` (`commandStart`, `commandStop`), which hands a typed accountant the tokens and a string one their `aaa.CanonicalCommand`; `internal/component/tacacs/accounting.go`; the operator's TACACS+ server | read, 2026-11-20 | change: each receives the chain tokens; the task id pairing of start and stop is preserved |
| REST and gRPC structured parameters | writer | REST and gRPC clients, outside the repository; MCP derived tools | `internal/component/api/invocation.go` `prepareCommand` | preserve the request format; under T-5 C an optional ordered `pipe` list is added |
| REST and gRPC structured parameters | reader | `internal/component/api/invocation.go` `buildInvocation`, `internal/component/command` `WriteInvocation` | read | change: written at the pipe offset; `BuildCommand` deleted |
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

### The design in seven lines

1. Generic operations apply to every command's answer, from every caller, with no declaration: the global ones to every command producing output, the data ones where the answer carries rows, the stream ones on monitors.
2. A command declares its own operations once, in the one existing registry: `RegisterPipeFilters` in-tree, the existing `Pipes` declaration for a plugin. A declared generic name overrides that operation where the catalog allows an override, and keeps its meaning and answer shape; a different meaning takes its own name.
3. Every caller sends the line as typed; nothing rewrites it.
4. One partition function in `internal/component/command` reads the line once, with one quote grammar for the command and the chain, and every route calls it: `Dispatch`, the stream route, the local-data route and the `ze cli` monitor. The declared operations reach the handler as a structured list, and the handler computes with them before the answer exists.
5. A declared operation is taken only while no residual operation precedes it, so the chain runs in the order typed.
6. The remaining operations run on the answer: the rendering surfaces keep doing it, and the engine does it for a plugin, MCP, REST and gRPC, a format operation included.
7. `show command help <name>` and `ze help command --json` list every operation the command answers to, generic, override, own or alias, from one function the `|` completer also reads.

### The chain grammar

This spec overturns a deliberate choice. Today the chain after the first pipe
has its own grammar: `ScanExecution` stops at that pipe, `splitPipeAt` hands
the rest to `parsePipeOps`, and the comments on both say the filter grammar
owns it, backslashes in match patterns included. Under that grammar a quote is
an ordinary character, so a pipe or a space inside a quoted value splits it.
Three things need the choice overturned: Thomas's rule that the engine handles
the chain for every caller, which needs one reading of it; the journal row
`splitter-ignores-the-quoting-its-tokenizer-honors.md`; and the structured
list, which needs argument boundaries the old reading discards
(`appendFilter` re-splits a value with `strings.Fields`). The departure is
presented to Thomas with this table, and the pages that state the old grammar
are corrected in phase 8.

| Rule | The chain today | The chain under this spec |
|------|-----------------|---------------------------|
| Where an operation ends | at every pipe character | at a pipe character outside double quotes |
| Words inside one operation | split at whitespace, quotes kept as characters | split at whitespace outside double quotes; a quoted run is one word and its quotes are removed |
| Escapes inside quotes | none | the invocation's own: an escaped quote, an escaped backslash, n, r and t (`ScanInvocation`) |
| A backslash outside quotes | a literal character | a literal character, kept: `match` patterns and paths rely on it. This is the one difference from the command part, where it stays refused |
| A backslash directly before a quote, outside quotes (`match a\"b c"`) | two literal characters | no escape exists outside quotes: the backslash is a literal character and the quote opens a quoted run, so the word is `a\b c`. With no closing quote (`match \"x`) the line is refused as an unfinished quote. An operator who wants a literal quote writes it inside quotes, escaped |
| A quote in the middle of a word (`match ab"c d"e`) | literal characters, and the space splits the word | the quote opens a run inside the same word and ends no word, as `scanInvocation` reads the command part: one word, `abc de` |
| A single quote | a literal character | a literal character, as in the command part: only the double quote groups. The config editor's own grammar differs (Known Limitations) |
| An unfinished quote | part of a word | refused with its column, as `ScanExecution` refuses one in the command part |
| What an operation's argument is | one string, its words joined by one space | a list of decoded words. An operation that takes text receives them joined by one space, so an unquoted argument reads as today and a quoted one keeps its inner spaces and pipes. The declared list carries the word list |
| An alias expansion | parsed by `parsePipeOps` | parsed by the same grammar |

### The prefix rule

`foldFilters` takes a declared operation out of the chain wherever it stands.
Only a format operator stops it, and only for the four generic names. So
`show bgp rib | resolve | count` counts in the handler and then resolves the
count, which is not the line typed. The partition takes a declared operation
only while every operation before it was declared too. After the first
residual operation:

| Operation met | Result |
|---------------|--------|
| a generic name the command also declares (`count` after `\| resolve`) | stays residual and runs as the generic, over the answer in hand |
| a name only the command declares (`peer` after `\| table`) | refused by name: the operation belongs to the command and must come before the first generic one; the fix line is the operator's line reordered |
| any other operation | residual, as today |

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
| A plugin's Stage 1 | a `Pipes` entry naming `top-talkers` on its own command, with `argument` set to `none` and no expansion | no such field; an entry without an expansion is refused ("expands to nothing") | recorded as an operation of that command; `show x \| top-talkers` reaches the plugin's handler in its pipe list |
| A plugin's Stage 1 | a `Pipes` entry with both `expansion` and `argument`, or with neither | the second is refused ("expands to nothing") | both refused by name: an entry is an alias or an operation, and says which |
| `show command help "show bgp rib" \| json` | what does the RIB command take after a pipe | `pipe-filters`: the RIB's fifteen names; no generic operator; nothing says which names replace a generic | `pipe-operations`: one entry per name with `name`, `source`, `argument`, `available`, `description`: `count` (source `override`, argument `none`, "Count matching routes without serializing rows"), `histogram` (`command`), `display` (`generic`, argument `fields`), `json` (`generic`), and so on (illustrative) |
| `show command help "show interface" \| json` | the same for a command that declares nothing | no pipe key at all, so the answer suggests the command takes nothing | `pipe-operations`: the generics that apply to its answer, each `source: generic` (illustrative) |
| `show command help "show bgp" \| json` | a command with aliases | `pipe-aliases`: `summary`, `peers` with their expansions | the same aliases in `pipe-operations` with `source: alias` and `expansion`, beside the generics (illustrative) |
| `ze help command --json` | the published catalog row of `show bgp rib` | three keys: `operators` (generics with `available`), `pipes` (the RIB's names), `pipe-aliases`; `count` appears twice, once in each of the first two | one `pipe-operations` list, the same entries `show command help` answers; `count` appears once, as `override` (illustrative) |
| Completion | `show bgp rib \| <Tab>` | every catalog operator, then the RIB's names; `count` shows the generic's description | exactly that command's `pipe-operations`, same names and descriptions; `count` shows the RIB's |
| Completion | `show bgp rib status \| <Tab>` (one document) | offers `count`, `first`, `last`, which are then refused | offers only what applies to a document |
| Any surface | `show bgp rib received count` (the old argument form) | `{"count": 5}` | refused: `count` is a pipe operation of `show bgp rib`; fix `show bgp rib \| received \| count` |
| SSH, web, `ze cli` | `show log recent \| match "error\|warn"` | the chain splits at the quoted pipe: `match "error` runs and `warn"` is refused as an unknown operator | `match` receives `error\|warn` as one argument and keeps the rows whose text holds those ten characters. `match` is a case-insensitive substring, not a regular expression, so this is no alternation |
| SSH, web, `ze cli` | `show log recent \| match "link down"` | `match` receives `"link down"` with its quotes and matches nothing | `match` receives `link down` |
| Any surface | `show bgp rib \| prefix "10.0` (unfinished quote in the chain) | the quote is a character of the argument; an unfinished quote before the pipe is already refused with its column | refused the same way: the command has an unfinished quote at that column; fix: add the closing double quote |
| Any surface | `show bgp rib \| resolve \| count` | the RIB counts first, then `resolve` runs on the count | `resolve` runs on the routes, then the generic `count` counts them: the order typed |
| Any surface | `show bgp rib \| table \| peer 10.0.0.1` | the RIB filters by peer, then the table is rendered | refused: `peer` is an operation of `show bgp rib` and must come before `table`; fix `show bgp rib \| peer 10.0.0.1 \| table` |
| Any surface | `show bgp rib \| sent`, `\| sent-received`, `\| aspath 65001` | refused as unknown filters; reachable only as arguments (`show bgp rib sent`) | declared operations, listed by help and completion |
| REST or gRPC | command `show interface \| count` with the declared parameter `name` = `eth0` (illustrative) | the parameter is appended after the chain: `show interface \| count name eth0` | written before the chain: `show interface name eth0 \| count` |
| REST or gRPC | a parameter value `10.0.0.0/8\|count` | written quoted by `WriteInvocation`, one argument | the same: it stays one argument and opens no chain |
| The web | `monitor bgp \| match established` (a stream) | `serverStreamCommand` splits and renders; no declared operation can reach the handler | the same answer; the handler lookup partitions, and a monitor's declared operation reaches its handler |
| REST or gRPC stream | the same line | `apiStreamSource` splits nothing and hands the handler lookup the whole line, chain included, as the monitor's arguments; what the argument validator then answers is not read here, and phase 1 captures it | the events that hold `established`, as on the web |
| `ze cli -c` | `monitor bgp \| match established \| save events.log` | the client splits, sends `monitor bgp`, filters, formats and saves; accounting records `monitor bgp` | the client sends the typed line with the rendering key; accounting records the typed line; the daemon applies `match` and sends only matching events; the client formats and saves |
| `ze cli -c`, or any SSH client that sets the key | `monitor bgp \| match established` with the rendering key set by hand | no such key | the daemon still applies `match`: the key hands over format, paging and `save`, never selection |
| REST or a plugin | `show bgp \| table` (a format on a data caller) | refused as a chain | the data is the rendered table as a JSON string, the bytes `ze cli -c` prints (illustrative) |
| Startup | a command declaring an operation named `json` or `save` | accepted; the generic and the command both claim the word | refused when recorded: the catalog says the operation cannot be overridden; a `BUG:` panic in-tree, a Stage 1 error for a plugin |

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
| `docs/architecture/api/commands.md`, "The surface splits the pipe chain; the dispatcher refuses one" | The surface splits; the dispatcher refuses; MCP, REST and gRPC refuse | The engine parses the chain for every caller; declared operations reach the handler as a list; generic operations run on the answer; the rendering statement |
| `docs/architecture/api/commands.md`, the per-command registries table (the row of `RegisterPipeFilters`) | `RegisterPipeFilters` declares segments "which `foldFilters` rewrites into server-side arguments" | The one registry of declared operations, filled in-tree by `RegisterPipeFilters` and by a plugin through `Pipes`, read by the partition and the introspection |
| `docs/architecture/api/commands.md`, "Pipe aliases", "Where an alias resolves, and where it does not", and the `show command help` row of the discovery table | A plugin's `Pipes` entry is an alias; help answers `pipe-aliases` beside `pipe-filters` | A `Pipes` entry is an alias with an expansion and a declared operation without one; help answers `pipe-operations` |
| `docs/architecture/cli/command-verbs.md`, the owned-filter paragraph under the operator classes | "`foldFilters` rewrites `show bgp rib \| peer 10.0.0.1` into `show bgp rib peer 10.0.0.1`" | Subject in the path, operations after the pipe; generic operations by class; an override keeps the generic's meaning and shape; a different meaning takes its own name; help, the catalog and completion read one derivation. "An owned filter takes the name of the field it selects on" stays |
| `docs/features/formatting.md`, "Command-specific filters" | Command filters are "folded into the command itself" and "resolved into command arguments" | The handler receives them as operations; the RIB's overrides and own names; `show command help` lists them. "a command filter stops it being produced" is kept |
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
| `argument` | `none`, `optional`, `text`, `count`, `fields` or `path` | the catalog's `Arg` for a generic or an override; the declaration's own statement for a command operation (`text` or `none` from `TakesArg` in-tree, the `argument` field of a plugin's `PipeDecl`); `none` for an alias |
| `available` | `always`, `with-rows` or `when-streaming` | `operatorsFor`'s rule for a generic: `ClassGlobal` always; `ClassData` always when the declared shape supports it, `with-rows` when no shape is declared, absent when the declared shape cannot; `ClassStream` when streaming; `always` for an override, a command operation and an alias, which the command itself answers |
| `class` | `global`, `data` or `stream` | kept from `operatorsFor`, for a generic and an override: `internal/le/doc/yangcontract/command_surfaces.go` refuses an operator with no class and `internal/le/site/catalog.go` labels operators by it (`operatorClassLabels`). Absent for a command operation and an alias, which have no class |
| `overridable` | true where a command may declare the name | the catalog entry, generics only |
| `local-only` | the catalog's `LocalOnly` | generics only |
| `description` | one line | the declaration's for an override or a command operation; the catalog's for a generic; the alias's own |
| `expansion` | the chain an alias stands for | aliases only |

The address-field gate `operatorsFor` applies to `| resolve` and `| origin` is
kept. The command's own answer shape stays where it is published today, the
`answer-shape` key of the catalog entry, read by the same two tools; it is not
repeated per operation. Whether the `pipe-filters` and `pipe-aliases` keys are replaced or kept
beside the list is decided here, not asked: they are replaced, with every
reader converted, because two lists for one fact is the drift this list exists
to end and no release consumes them.

### How an override keeps the generic's shape

Decided from the code. The catalog is the one list of generic names, the
argument contract is a static fact of the declaration, and the answer exists
only at run time.

| Half | Where it is checked | Why there |
|------|---------------------|-----------|
| The name is a generic one | when the declaration is recorded: `RegisterPipeFilters` and the plugin entry point look the name up in the catalog (`knownPipeOps`) and mark the entry as an override | the catalog is the one list of generic names |
| The generic allows an override | when recorded: the catalog entry says whether it is overridable. Declaring a name whose entry says no is refused: a `BUG:` panic in-tree, a Stage 1 error for a plugin, naming the command and the operation | today the fact is a four-case switch in `foldFilters`; a fifth overridable generic would need an edit there, and nothing refuses a command that declares `json` |
| The argument contract | when recorded: an override's argument must agree with the catalog entry's `Arg`; a mismatch is a `BUG:` panic in-tree and a Stage 1 error for a plugin | a static fact, refused before any caller meets it |
| The answer shape | at answer time, by one function of the command package beside the partition's answer step, which `Dispatch`, `ServeLocal` and the stream sites all run; it is not a step of `Dispatch` alone. The check applies to the LAST declared operation of the list, only when that operation is an override, and only to an answer whose status is done. An error answer, an override followed by another declared operation, and a list ending in an own-named operation are not checked, because the answer then belongs to something else | a third-party plugin's tests are not ours to require; the check costs one look at an answer already received |
| The meaning | by review and by each override's own test, which compares it with the generic over the same rows (`TestRIBOverridesKeepGenericMeaning`) | meaning cannot be checked mechanically |

What the catalog states per entry, and what the check holds an override to:

| Generic | Overridable | Shape an override's answer is held to |
|---------|-------------|----------------------------------------|
| `count` | yes | one object holding an integer `count` |
| `first N`, `last N` | yes | the command's rows envelope, and on an answer that is not streamed at most N rows. A streamed answer is not counted by the engine |
| `match` | yes | the command's rows envelope. Nothing mechanical can tell a correct match from a wrong one, so `match` is held by its override's test only |
| `display`, `fill`, `resolve`, `origin` | yes | the command's rows envelope; which rows and fields is meaning, held by the override's own test |
| the formats, `no-more`, `save`, `log` | no | none |

Every data operator is overridable (Thomas, 2026-11-20); a new data operator
states its shape in its catalog entry, and TestEveryDataOperatorIsOverridable
refuses one that does not.

A declared answer shape (`RegisterShape`) was set aside as the carrier: it
states the command's shape, not one operation's answer. A test every override
must pass was kept as the meaning check and set aside as the only check.

### Declarations audited against the override design

Every in-tree `RegisterPipeFilters` call is in
`internal/component/bgp/plugins/cmd/rib/rib.go` `registerPipeFilters`; the two
empty sets are barriers. No in-tree plugin declares an operation through
`Pipes` today, because `PipeDecl` carries aliases only. `show bgp rib` declares
fifteen names and `show bgp rib best` twelve.

Three decisions follow from the rule that no reachable feature is dropped, and
are not put to Thomas:

- **The words the argument grammar accepts and no declaration names are
  declared.** `sent`, `sent-received` and `aspath` are reachable today only as
  arguments, and the argument form goes, so each becomes a declared operation
  of `show bgp rib` (`aspath` of `show bgp rib best` too), with its own
  description. `advertised` and `sent` stay two names for one scope, as
  `scopeKeywords` holds them.
- **`show bgp rib protocol` declares its own set.** Today it inherits the
  fifteen names of `show bgp rib` by prefix, by accident: it has no
  declaration. Its reader discards the scope, because a protocol RIB holds
  received routes only. Its set is stated explicitly: `peer`, `family`,
  `prefix`, `covering`, `exact`, `path`, `aspath`, `community`, `match`,
  `count`, `first`, `last`, `histogram`, `graph`. A scope word is not in it, so
  `show bgp rib protocol bmp | advertised` is refused with the valid set,
  where today the word is accepted and does nothing. The protocol name stays a
  command argument: it names the subject.
- **A scope is accepted at any position of the declared list.** `Leading` put
  it first for the fold; `parsePipelineArgs` already accepts a scope word
  anywhere before the terminal and refuses a second one, and the list keeps
  that.

| Command | Declared name | Generic of the same name | Verdict |
|---------|---------------|--------------------------|---------|
| `show bgp rib`, `show bgp rib best` | `count` | `applyCount`, `{"count":N}` over the rows | override: same meaning, same shape |
| same | `first`, `last` | `applyFirst`, `applyLast`, N rows | override: same meaning; whether the RIB and the generic refuse N of 0 alike is checked in phase 3 |
| same | `match` | `applyMatch`, case-insensitive substring over every value of a row | override: same meaning on a narrower field set. `(*matchFilter).matches` reads the prefix, the peer, the family and the route attributes, and not the row's `direction` field, so `\| match sent` could keep a row the generic keeps and the override drops. Unverified at the producer for every field: a finding for Thomas; phase 5 compares the two over the same rows |
| `show bgp rib`, `show bgp rib best` | `histogram`, `graph`; `reason` on `best` only | none | own names, own descriptions |
| `show bgp rib`, `show bgp rib best` | `peer`, `family`, `prefix`, `path`, `community` | none | own names |
| `show bgp rib` | `received`, `advertised` | none | own names; a scope, accepted at any position |
| `show bgp rib` | `covering`, `exact` | none | own names; source selectors read by the received walk (`rib_pipeline_covering.go`), refused on sent routes by the handler as today |
| `show bgp rib` (argument grammar, not a declaration) | `sent`, `sent-received` in `scopeKeywords` | none | declared by this spec as own names |
| `show bgp rib`, `show bgp rib best` (argument grammar, not a declaration) | `aspath` (`filterASPath`), the second spelling of `path` | none | declared by this spec as an own name. The review named it `as-path`; the code spells it `aspath` |
| `show bgp rib`, `show bgp rib best` (argument grammar, not a declaration) | `json` terminal in `terminalKeywords` | the `json` format, which the catalog does not allow a command to override | not declared: the name is the generic's. Read on 2026-11-20 (`newJSONTerminal`, `showRows`, `showProtocolPipeline`, `bestPipeline`). On `show bgp rib` the terminal answers the rows the bare command streams, under the same `routes` key and in the same order, so the word goes and nothing is lost. On `show bgp rib protocol` the bare command already runs that terminal. On `show bgp rib best` the terminal answers a second shape (`routes` rows with `peer`, `direction` and flat attributes) where the bare command answers `best-path` rows; the word goes there too, and that second shape with it, which is D-7 |
| `show bgp rib protocol` | none of its own; inherits the fifteen by prefix | as above | declares its own set, stated above |
| `show bgp peer rib` | none: the path holds no pipe set, and its scope is the YANG `leaf scope` | as `show bgp rib` | declares its own set, the names of `show bgp rib` as this spec leaves them, scope words included (second review) |

### How a plugin's operations are recorded and withdrawn

`RegisterPipeFilters` panics, holds no owner and has no withdrawal: right for a
registration in this repository, wrong for a declaration that arrives on the
wire. A plugin's operations enter `pipeFilterRegistry` through a second entry
point shaped like `RegisterPluginAliases`: it takes the owner, returns an
error, and records the whole batch or none of it.

| Collision | Answer |
|-----------|--------|
| the name is a generic the catalog does not allow a command to override | the batch is refused, naming the operation |
| the name is an alias registered on an overlapping command path, in-tree or by a plugin | refused, naming both, as `aliasShadowing` does for an in-tree filter today |
| the name is a global alias | refused, naming it |
| the command path already holds another owner's set, in-tree or another plugin's | refused, naming the path and its owner |
| the same name twice in one message | refused, as the alias registry does |

Withdrawal is by owner, at the three sites that withdraw a plugin's aliases
today: the two rollback arms after `registerPluginPipes` in
`internal/component/plugin/server/startup.go`, and the cleanup in
`internal/component/plugin/server/restart.go` that runs on a failed startup, a
removal from the config and a restart. A command path returns to what it held
before the plugin declared.

### Decisions the owner's rules already make

| # | Question | Decision | Source |
|---|----------|----------|--------|
| T-1 | A format operation (`\| table`, `\| text`, `\| yaml`) on a data caller: a plugin's `dispatch-command`, REST, gRPC | Rendered, and the text is carried as the answer's data: the bytes `ze cli -c` prints. It is never refused and never dropped | `ai/META-DECISIONS.md`, "Goal and scope": "Every operator command runs through the daemon on every command surface (MCP, REST, gRPC, SSH, web) with the same output as the local `ze` command" |
| T-6, accounting half | What the accounting record of a piped line holds | The line as typed, chain included, secret-masked as today; a typed accountant receives its tokens in order | RFC 8907 Section 8.3: "To support this mode of operation, TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized." The command entered is the line with its chain. The quote already sits above `Dispatch` |
| - | Whether `sent`, `sent-received`, `aspath`, and the set of `show bgp rib protocol` are declared | Declared, as "Declarations audited" states | no feature is dropped; an agreed design answers what follows from it |
| - | Whether a REST parameter value holding a pipe or a quote is refused | Not refused: `WriteInvocation` already writes it quoted, as one argument | the producing code; a refusal would drop a value that works today |

### Choices only Thomas can make (decided 2026-11-20)

Thomas approved the design as recommended on 2026-11-20 ("it looks good to me.
You can implement it."). Each choice below is decided at its recommendation:

| # | Decision (2026-11-20) |
|---|-----------------------|
| T-2 | A, as recommended: a surface inside the daemon (SSH exec, the web terminal, the daemon-hosted model) runs the shared partition and renders the residual chain; a client in another process takes over only the presentation suffix (format, paging, `save`), stated through the SSH session environment key, and the daemon applies every selecting or reducing operation |
| T-5 | C: every derived MCP tool and every REST and gRPC command route takes one optional, ordered `pipe` list, validated against the command's `pipe-operations`, written after the parameters |
| T-6, authorization half | A: the typed line, chain included, reaches the authorizer; a typed authorizer receives the command, its arguments, then the chain as tokens, for authorization only; read-only is judged on the command path |

WIP cap: Thomas authorised on 2026-11-20 claiming this spec above the cap
(ZE_SPEC_WIP_CAP=22), for this spec only.

The evidence each was decided on is kept below. The last column names what
would have changed under the alternative.

| # | Question | Evidence | Options | Recommendation | What changes under the alternative |
|---|----------|----------|---------|----------------|------------------------------------|
| T-2 | Where the generic operations run for SSH, the web CLI and `ze cli` | Today each surface runs them from its own `ProcessPipes*` call (`execMiddleware`, `executeTerminalOperationalResult`, `executeOperationalCommand`, `StreamMonitor`); the record-stream path renders rows as they arrive (`RenderRecords`); `\| save` and paging need the caller's side | A: at the surface, as today. B: inside the engine for every caller | A for a surface inside the daemon (SSH exec, the web terminal, the daemon-hosted model), which runs the SAME partition function as the engine (A-4). For a client in another process, A is narrowed for security: the daemon applies every selecting or reducing operation, and the client takes over only the presentation suffix (format, paging, `save`), stated through the SSH session environment. A client never decides which rows it receives | Under B the rendering statement and the session key are not built; the engine renders for every caller; `RenderRecords`, `\| display`, paging and `\| save` move behind the dispatcher, and `ze cli` needs a daemon form of `\| save`. AC-22 and AC-23 change to assert the engine renders |
| T-5 | How a derived MCP tool and a REST or gRPC parameter route express pipe operations, once both `leaf-list args` are gone | Derived tools take named parameters today (`prepareCommand`); a chain is ordered and may repeat a name (`\| peer a \| first 5`), which named parameters cannot express; `ze_execute` and REST execute already take a whole line | A: each declared operation becomes an optional named parameter of the derived tool. B: derived tools stay at the subject; a caller uses `ze_execute` or execute with the chain in the line. C: every derived MCP tool and every REST and gRPC command route takes one optional `pipe` list, ordered, each element an operation name and its argument words; the adapter validates each name against the command's `pipe-operations` and writes the chain after the parameters | C: it keeps the order typed, covers generic operations as well as declared ones, and the adapter holds no list of its own. A loses the order and cannot repeat an operation | Under A, AC-15 asserts optional named parameters and a fixed order, declared operations only. Under B, phase 7 builds nothing for derived tools and AC-15 asserts that the derived tool of `show bgp rib` offers no operation and its description names `ze_execute` |
| T-6, authorization half | What the authorizer sees for a piped line | A string authorizer receives the whole input today, a typed one the first token and the rest (`isAuthorized`); the TACACS+ authorizer sends the tokens as `cmd-arg` values (`tacacs/authorizer.go`), so a per-command policy on the server matches on them; today no piped line reaches either, because `Dispatch` refuses it, except on the stream route. There the whole line, chain included, reaches `IsAuthorized` at two of the three sites: `serverStreamCommand` (`cmd/ze/hub/server_stream.go`, before it splits) and `apiStreamSource` (`cmd/ze/hub/api.go`, which never splits). The SSH exec closure in `cmd/ze/hub/service_ssh.go` receives the command part `execMiddleware` split off, so its authorizer and accountant see no chain today | A: the typed line, chain included: a string authorizer receives the line, a typed one the command, its arguments, then the chain as tokens (a pipe token, the operation name, its argument words), for authorization only; the handler's arguments are unchanged; read-only is judged on the command path. B: the command part only | A: a TACACS+ per-command policy keeps seeing what the operator typed, and two of the three stream sites already behave so | Under B the carrier of the chain into `AuthorizeCommandArgs` is not built, the implementers and the wrappers are untouched, the web and the REST and gRPC stream sites are changed to authorize the command part as the SSH one does, and AC-27 asserts the authorizer receives no chain token. The accountant still receives the typed line on every site |

### Decisions of the second review (2026-11-20)

| Subject | Decision | Why |
|---------|----------|-----|
| `show bgp peer <selector> rib <scope>` | It declares its own operation set, the scope words included, the same names as `show bgp rib`; the `leaf scope` of `container rib` in `cmd/peer/yang/ze-peer-cmd.yang` goes; the argument form is refused with the pipe form as its fix (AC-34). The census gains the search `bgp peer \S+ rib (received\|advertised\|sent\|sent-received)`; known users: `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go`, `internal/le/interoplab/bgp/check_ospf_redistribute_default_diagnostics_test.go`, `internal/component/plugin/server/usage_model_test.go` | no-layering: one carrier for one fact; the path names the subject and stops |
| The stream route | The partition and the declared-list attachment live inside `pluginserver.GetStreamingHandlerForCommand`, which all four lookup sites call; `execMiddleware` passes the typed line through `StreamingExecutor`. R-11's test also fails on a handler lookup that bypasses that function | four sites, one function: two were missed by the first amendment |
| The rendering key | Presentation suffix only, as "The carrier on each route" states | the key is untrusted input |
| Authorization and accounting tokens | Taken from the partition's scan, never from a second `ScanInvocation` of the whole line, which refuses a chain backslash (`ErrBackslashInInvocation`) and would deny `\| match a\b` (AC-38). Under T-6 A the implementers read are the three named earlier, `fallbackArgs` in `tacacs/authorizer.go`, and the six typed wrappers in `cmd/ze/hub/aaa_lifecycle.go` | the chain grammar keeps a literal backslash; the authorizer must read the same grammar |
| The argument contract of a declared operation | One closed vocabulary, the catalog's argument-kind spellings (`none`, `optional`, `text`, `count`, `fields`, `path`), validated by the partition. The in-tree `PipeFilter` takes the same field in place of `TakesArg`, so a builtin and a plugin state the fact in one shape | one fact, one shape |
| The override answer-shape check | Placed beside the partition's answer step, in the command package, so `Dispatch`, `ServeLocal` and the stream sites all run it; not in `Dispatch` alone | the check must hold on every route the partition serves |
| `ze <verb>` and `RenderLocalAnswer` | `RenderLocalAnswer` is handed a path, not a line, so it carries no chain; a declared operation reaches a local-data handler only through `ServeLocal` | read at `local_data.go` |
| The config editor's own pipe grammar (`docs/guide/config-editor.md`, the paragraph on command input) | Out of scope: it filters the editor's display of the configuration (`show \| match`, `show \| errors`, `show \| changes`) with the config token codec (`cli.ParseCommandWords`, `SplitPipe`, `ParsePipeFilters`) and dispatches no operator command. Read on 2026-11-20: it already keeps a quoted pipe as a value, and it also groups with single quotes, which the operational grammar does not | another grammar, another subject; the difference is named in Known Limitations |
| `ai/rules/cli.md`'s example `show sysctl \| key <k>` | An example of the rule only; this spec converts no sysctl command | no scope growth |
| Who refuses a pipe operation typed as an argument (`show bgp rib received count`, `show bgp peer <selector> rib received`) | The engine's argument validator, for every command, never `bgp-rib`. Before it binds anything, `ValidateArgs` (`internal/component/command/argbind.go`, reached through `ValidateModelArgs` in `local_data.go`) refuses a word the command's `pipe-operations` list holds as an operation, naming it as a pipe operation of that command with the pipe form of the line as the fix. The set comes from the introspection function, with no list of its own. The RIB keeps no argument to refuse, and `refusePipelineArgs` loses its argument-form fix lines | read on 2026-11-20: the validator already speaks before any handler, and today it cannot give this answer. With definitions still open a leftover word is refused by `positionalError`, as the open leaf's own value refusal or "unexpected argument"; with no definition open, or none declared ("the tokens pass through unchanged"), the word passes to the handler. So after the leaves go the word would be refused as a bad peer address or passed on unjudged. A catch-all RIB argument kept only to refuse is a second carrier (no-layering), and a derived refusal serves every command that declares operations (registration over enumeration) |

### Departures from the design documentation, for Thomas

Each was presented by the main session with its before and after before the
spec moved to `ready` (`ai/META-DECISIONS.md`, "How agents work"). Thomas
approved all nine, D-1 to D-9, on 2026-11-20. He replaced D-5's limit (only
`count`, `first`, `last` and `match`) the same day: "I believe it should be
possible to overwrite more command as long as their data format is not
changed." Phase 8
corrects the documentation each one contradicts.

| # | Departure | Before, as the operator sees it | After | Documentation it contradicts |
|---|-----------|---------------------------------|-------|------------------------------|
| D-1 | The chain takes the invocation's quote grammar | `\| match "link down"` matches nothing; `\| match "a\|b"` is torn in two | the quoted value is one argument, quotes removed | `docs/architecture/api/commands.md` and the comments of `invocation_scan.go` and `splitPipeAt`: the filter grammar owns the chain |
| D-2 | The prefix rule | `show bgp rib \| table \| peer X` filters then renders; `show bgp rib \| resolve \| count` counts then resolves | the first is refused with the reordered line; the second resolves then counts | `docs/features/formatting.md`, "Command-specific filters" |
| D-3 | A scope on `show bgp rib protocol <p>` | `\| advertised` is accepted and does nothing | refused, naming the valid set | the inherited completion and help of that command |
| D-4 | The typed-selector sentence of `ai/rules/cli.md` | a selector is a keyword and value on the path | selection is a pipe operation the command declares | `ai/rules/cli.md`, "CLI Grammar: Keywords Before Values" |
| D-5 | Every data operator can be overridden, keeping its generic's data format; a presentation operator cannot. Decided by Thomas 2026-11-20 in the words above, replacing the four-name limit | nothing refuses a command declaring any generic name | a command may declare `match`, `count`, `first`, `last`, `display`, `fill`, `resolve` or `origin`, held to the generic's argument kind when recorded and to its answer shape on a done answer (`count` one object with an integer `count`; `first N`, `last N` the rows, at most N; the others the rows envelope). Declaring a format, `no-more`, `save` or `log` is refused: they are the data format, or the caller's display of a stream, which a handler cannot run | the META sentence ("keeping the generic's meaning and answer shape") now agrees; recorded in `docs/architecture/api/commands.md` Design decisions, 2026-11-20 |
| D-6 | T-1: a format on a data caller | refused, as every chain is | the rendered text, carried as a JSON string in the answer's data | Corrected on 2026-11-20: `docs/architecture/api/json-format.md` does not say a data caller receives structured data. Its "Response Format" types `response.data` as `any` ("Payload or error message") and its CLI envelope example shows an object, so a string is inside what the page states. What T-1 contradicts is `docs/architecture/api/commands.md`, "The surface splits the pipe chain; the dispatcher refuses one" (MCP, REST and gRPC refuse a chain), which phase 8 rewrites, and the habit `ai/doors/commands-cli.md` records, "A handler answers data, never text", which stays true: the engine renders, no handler does. Phase 8 adds one sentence to `json-format.md`: a format operation makes `data` the rendered text |
| D-7 | The RIB `json` terminal | `show bgp rib json` and `show bgp rib protocol <p> json` answer the document the bare command answers. `show bgp rib best json` answers a second shape: `{"routes": [...]}`, each row with `peer`, `direction` and flat attributes, where the bare `show bgp rib best` answers `best-path` rows with `best-peer`, `multipath-peers` and nested `attributes` | the word goes on all three. `show bgp rib \| json` and `show bgp rib best \| json` render the bare command's rows, so `best` keeps one shape, `best-path`. A-11 is settled: nothing is lost on the first two; on `best` the `routes` shape is dropped, the routes themselves are not. In-tree users of the word as an argument: the help text in `help_ai.go` and `TestBestJSONTerminalRowCarriesNoAge` | `ze-rib-api.yang`, "json returns serialized output", on `rpc show` and `rpc best` |
| D-8 | Help and catalog keys; RIB operations leave the YANG model | `pipe-filters`, `pipe-aliases`, `operators`, `pipes`; `leaf-list args` | one `pipe-operations` list; operations are declared in the registry, not as a leaf | `ai/doors/commands-cli.md`, "The argument is a leaf the model declares" |
| D-9 | `show bgp peer <selector> rib <scope>` | the scope is an enumerated leaf, completed and validated by the model | the scope is a pipe operation | `ze-peer-cmd.yang`, the same door sentence |

### Open after the second review

The continuation of 2026-11-20 brought the census, the files, the tests, the
phases, the risks and the acceptance criteria in line with the decisions
above. Two things stay open, each with its reason:

- **The docs half of A-12 and of the peer-scoped census.** Closed on 2026-11-20: the search ran over `docs/`, found the same 17 pages for the scope-word form and none for the peer-scoped form, and A-12 holds there. Which hits in `exabgp-bridge.md` and `update-syntax.md` belong to another command is read by phase 6f before either page is changed
- **`plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md`**: its `Depends` row naming this spec. The spec tooling refuses any edit of that spec until it carries `## Existing Mechanisms`, which is that spec's own work

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `APIEngine.Execute` and `Stream` judge read-only on the command part of a piped line | `internal/component/api/invocation.go` `prepareCommand`: `ScanExecution` stops at the first unquoted pipe, `invocationCommandMeta` matches the words before it, and the read-only flag is the matched declaration's | a chain changes the verdict, or a piped write passes a read-only caller | `TestExecutePipedReadOnlyCaller`, `TestStreamPipedReadOnlyCaller` | confirmed by reading on 2026-11-20: `APIEngine.Execute` and `APIEngine.Stream` (`internal/component/api/engine.go`) both call `prepareCommand`, which takes `meta.ReadOnly` from `invocationCommandMeta` over the words `ScanExecution` answers before the first unquoted pipe. `TestExecutePipedReadOnlyCaller` (`internal/component/api/engine_test.go`) green in d30af7c2f1; `TestStreamPipedReadOnlyCaller` still owes its run |
| A-2 | Every `OnExecuteCommand` handler can take the request with a pipe list without behaviour change | the non-test registrations read args and peer only. Recounted on 2026-11-20: 53 non-test files and 19 test files call `OnExecuteCommand(` (72 in all), not 46 and 8 | a handler that reads args positionally misreads | the scoped package tests of each plugin touched | unvalidated (needs the runs) |
| A-3 | REFUTED as first written (three commands), by the second review. Four commands reach the RIB argument grammar: `show bgp rib`, `show bgp rib best`, `show bgp rib protocol` (inherits the set by prefix) and `show bgp peer <selector> rib <scope>`, which `forwardRibRoutes` forwards to `cmdRibShow` and which holds no pipe set. The assumption now is that there is no fifth | `RegisterPipeFilters` call sites in `cmd/rib/rib.go` (read 2026-11-19); `cmdRibProtocol` is in no declaration; `rib_inject.go` `showProtocolPipeline` | another handler below `show bgp rib` reads folded words and breaks | grep of `RegisterPipeFilters` and of every command path under `show bgp rib` at phase 1 | confirmed on 2026-11-20: the four `RegisterPipeFilters` calls are all in `cmd/rib/rib.go` `registerPipeFilters`; the paths under `show bgp rib` (`cmd/rib/rib.go` constants) are `status`, `best`, `best status`, `rpf`, `protocol`, `help`, `commands`, `events`, all barred but `best` and `protocol`; the three argument readers have one caller each (`rib_pipeline.go` line 1369, `rib_pipeline_best.go` line 322, `rib_inject.go` line 236), and `forwardRibRoutes` serves both `ze-bgp:rib-routes` and `ze-bgp:peer-rib`. No fifth command |
| A-4 | The rendering surfaces and `Dispatch` produce the same partition from the same line | one function, deterministic, the same registry | SSH and the dispatcher disagree | `TestPartitionSameOnSurfaceAndDispatch` | confirmed on 2026-11-20: `TestPartitionSameOnSurfaceAndDispatch` (`internal/component/command/dispatch_line_test.go`) green. Every route ends in the one `partitionPipeChain` (`pipe_partition.go`): `Dispatch` and the cross-process renderers through `ReadDispatchLine`, the in-daemon rendering surfaces through the `ProcessPipes*` wrappers, the stream route through `ReadStreamLine` |
| A-5 | Applying a residual chain inside `Dispatch` can reuse `ApplyPipes` over the flattened answer | `CommandDispatcher.JSON` flattens with `ResponseJSON` | a streamed `sdk.Records` answer needs its own path | `TestDispatchAppliesResidualChainToStreamedRows` | confirmed on 2026-11-20: the dispatcher tests of `internal/component/plugin/server/dispatch_pipe_chain_test.go` are green; `Dispatch` applies the residual chain through the answer step of `ReadDispatchLine` (`ApplyAnswerStep`) |
| A-6 | `pipeFilterRegistry` can take an owner-scoped, error-returning declaration and its withdrawal as the plugin alias registry does | `declarationRegistry.declare` panics on a conflict today; `RegisterPluginAliases` checks before it writes and records what each owner created; `UnregisterPluginAliases` | a plugin's operations outlive the plugin, or a bad declaration panics the daemon | `TestPluginOperationWithdrawnOnStop`, `TestPluginOperationCollisionsRefused` | confirmed by reading on 2026-11-20 that the shape exists to copy: `RegisterPluginAliases` (`alias.go`) checks the batch under its lock and returns an error before it writes, and the three withdrawal sites are `startup.go` lines 960 and 969 and `restart.go` line 232; the tests still owe their run |
| A-9 | `command.ValidatedArgs` can carry the declared list to a streaming handler and a local-data handler without changing either signature | `internal/component/command/argbind.go` `ValidatedArgs` is a struct the command package builds and both handler kinds receive | the two handler signatures change, with every registration | reading `ValidatedArgs` and its constructors at phase 1 | confirmed on 2026-11-20: `ValidatedArgs` (`argbind.go`) is a struct of unexported fields the command package builds, and `StreamingHandler` (`plugin/server/handler.go`) takes it by value, so one more field reaches both handler kinds with no signature change |
| A-10 | The SSH exec channel can read a session environment key | `parseSessionRequest` reads `sess.Environ()` for the interactive session; `execMiddleware` holds the same session value | the rendering statement needs another carrier for the `ze cli` monitor | `TestExecSessionRendersStatement` | confirmed by reading on 2026-11-20: `execMiddleware` (`ssh.go`) holds the same `ssh.Session` whose environment `parseSessionRequest` (`session.go`) reads for the interactive session; the exec channel reads no key today, which is the work. The test still owes its run |
| A-11 | The RIB's `json` terminal answers the routes the bare command answers | read on 2026-11-20: `apply` builds `newJSONTerminal`, whose rows are `serializeRouteItem` plus `peer` and `direction` under `routes`; `showRows` streams the same rows under the same key; `showProtocolPipeline` runs the terminal for the bare command; `bestPipeline` answers `best-path` rows built by `bestResultFor` | on `show bgp rib best` the terminal is a second shape of the same routes, dropped with the word (D-7) | `TestRIBJSONTerminalEqualsBareCommand` in phase 5, before the word goes: equal documents for `show bgp rib` and `show bgp rib protocol`, the same family, prefix and winning peer per row for `show bgp rib best` | confirmed by reading for `show bgp rib` and `show bgp rib protocol`; broken as first written for `show bgp rib best`, where the shapes differ and the routes do not. The test still owes its run |
| A-12 | No in-tree line relies on a quote being a literal character in the chain, or on a declared operation jumping ahead of a residual one | searched on 2026-11-20 over `test/` (`.ci`) and the Go under `internal/test`, `internal/le/interoplab`, `internal/component/lg` and `cmd`: no line holds a quoted word directly after a pipe operation name; over `internal`, `test`, `cmd` and `pkg`: the one RIB line with a declared word after a residual one is `internal/component/command/pipe_test.go` (`show bgp rib \| count \| ndjson \| first 1`), a unit test of the fold that phase 3 replaces | a `.ci` or fixture changes its answer under the new grammar or the prefix rule | the same two searches over `docs/` at phase 1, which the hook refused in this pass | confirmed on 2026-11-20: over `docs/`, the only quoted word after a pipe operation is `docs/guide/config-editor.md` (the editor's own grammar, out of scope) and shell pipelines in `docs/guide/monitoring.md`; no RIB line holds a declared word after a residual one. The peer-scoped form is in no `docs/` page, so the 17 pages carry the scope-word form only |
| A-7 | `operatorsFor`'s availability rule agrees with what the chain validator admits | `internal/plugins/help/help_command.go` `operatorsFor`, `internal/component/command/pipe.go` `validateDeclaredShape` | help lists an operation the chain refuses | `TestPipeOperationsAgreeWithValidator` | unvalidated |
| A-8 | `operatorsFor` can move into `internal/component/command` | it reads `command.PipeOperatorCatalog()` and `command.Declared`; its local-handler check reads `command.HasLocal` and `HasLocalData`, and `cli.WireToPaths` is passed in by the caller | an import cycle keeps part of it in `plugins/help` | compiling the move in phase 7 | confirmed by reading on 2026-11-20: `operatorsFor` and `pathHasOnlyPlainLocalHandler` (`help_command.go`) read only `command.PipeOperatorCatalog`, `command.Declared`, `command.HasLocal`, `command.HasLocalData` and the `wireToPaths` map their caller passes in; the compile still owes its run in phase 7 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A quote-aware split changes an SSH line that relied on a quote being ignored | an existing `.ci` with a quote and a pipe goes red | read each such test against its expectation before changing it |
| R-2 | The unfinished-quote refusal, extended to the chain, reddens a line that worked by accident | a `.ci` or fixture with an odd quote after a pipe | fix the line |
| R-3 | A handler handed a declared operation ignores it | an answer unfiltered with status done | a wiring test per declaring command |
| R-4 | A residual chain on a large streamed answer from a data caller materializes it | memory on `show bgp rib \| match x` over REST | the cost SSH pays today; recorded, not new |
| R-5 | `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` (design) is written on `foldFilters` rewriting the line: its Key insights, its Current Behavior rows for `pipe.go` and `pipe_filter.go` (`Leading`), its Data Flow steps 4 to 6 and its "CLI ↔ daemon" boundary ("the rewritten command string, after `foldFilters`") all assume the fold | those sections name `foldFilters` | reworked onto the structured list before it is approved; this spec is its dependency, and its `Depends` row names this spec |
| R-6 | The SDK signature change misses a handler built outside the default flavor | a tagged build fails | `./le repo compiles check` after the SDK commit |
| R-7 | The published catalog key change breaks the wiki and website generators | `./le doc yang-contract` or the site catalog check goes red | both are readers in the census and convert in phase 7 |
| R-8 | The chain grammar changes a line that works today: a quote that was data becomes a delimiter | a `.ci`, fixture or docs example with a quote after a pipe changes its answer | A-12's search before phase 2; each hit is read against what its author meant |
| R-9 | The prefix rule refuses or reorders a line that worked: a declared RIB word typed after a generic one | the same search; an LG or interoplab line with `\| table \| peer` | the line is reordered in the conversion phase; the refusal's fix line is the reordered line |
| R-10 | The `ze cli` monitor client holds no external plugin's operations or aliases, so it and the daemon could read one line differently. The presentation suffix removes the client's partition: the client reads the catalog alone, on the names as typed, and refuses no name it does not know. What remains is a client and a daemon that disagree on the suffix | a monitor's events formatted twice, or not at all; a line with an alias before `\| save` rendered by nobody | `TestPresentationSuffixSameOnClientAndDaemon` runs the table of "The carrier on each route", the alias row included, through the one suffix function both sides call; the catalog is compiled into both from one source. Resolved by 07f5b7e39c: `presentationSuffixLength` (`command/stream_line.go`) is the one suffix function, read by `ReadStreamLine` in the daemon and `PresentStreamSuffix` in the client; `TestPresentationSuffixSameOnClientAndDaemon` |
| R-11 | A route is missed again: a caller splits a line without the partition, or looks a streaming handler up around the function that partitions. The first amendment missed two of the four lookup sites | a new `ProcessPipes*` or `ParsePipe` call site; a new reader of the streaming-handler registry | `TestEveryPipeSplitGoesThroughThePartition`, which fails on a non-test caller of the retired split and fold entry points, and on a non-test caller of `matchStreamingCommand` or reader of `streamingHandlers` outside `GetStreamingHandlerForCommand` and the registry's own functions. Resolved by 07f5b7e39c and a374677264: `TestEveryPipeSplitGoesThroughThePartition` has two halves, the streaming-registry half in `internal/component/plugin/server/streaming_lookup_test.go` and the split and fold half (in-package call graph, and importers of `command` that cut at a pipe) in `internal/component/command/pipe_split_partition_test.go` |
| R-12 | The rendering key is read as permission: a client sets it to receive rows a `\| match` the authorizer judged would have removed | a stream with the key set that carries a non-matching event | the daemon applies every operation before the presentation suffix whatever the key says (AC-36); the key is validated and honoured on the exec stream route only (AC-37). Resolved by 07f5b7e39c: the key is `ZE_CLI_RENDER=client`, read by `execStreamRendering` (`ssh/session.go`); `TestRenderingKeyNeverSkipsSelection`, `TestRenderingKeyUnknownValueRefused`, `TestRenderingKeyIgnoredOnOneShotExec` (`internal/component/ssh/exec_stream_test.go`) |
| R-13 | A chain backslash makes an allowed line unauthorized or its record a single token: `tokenize` refuses a backslash outside quotes, `isAuthorized` denies a line it refuses, and `BeginAccounting` records such a line as one token | `\| match a\b` denied, or accounted as one `cmd` value, where the bare line is allowed | authorization and accounting read the chain-grammar scan (AC-38) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Piped commands answer wrongly or are refused on some surface; the RIB answers its full table where a count was asked; a monitor loses its chain; a line with a quote after a pipe reads differently; a piped write passes a read-only caller, or an authorizer misses a chain it used to see on the stream route; the TACACS+ command log changes what it records; a plugin built on the old SDK does not start; a script or generator reading the old help or catalog keys finds nothing; the looking glass, 99 files of fixtures, `.ci`, interop checks and docs send a line the RIB now refuses |
| How is it reverted? | commit revert per phase; the SDK phase reverts with its plugin rebuilds; the conversion phase reverts with the RIB refusal it follows |
| Who else touches this path? | `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` (depends on this spec), `plan/release/fixes/spec-plugin-alias-reaches-the-client.md`, the zero-alloc RIB streaming work, every plugin with `OnExecuteCommand` or `Pipes`, the three typed authorizers and the six wrappers in `cmd/ze/hub/aaa_lifecycle.go`, the BGP peer command plugin (`ze-peer-cmd.yang`), the SSH server and the `ze cli` client, the looking glass, the catalog tools under `internal/le/` |

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
| A monitor line with a chain on the web, REST stream and gRPC stream | → | `serverStreamCommand` and `apiStreamSource` → `GetStreamingHandlerForCommand` → the partition → `ValidatedArgs` → the streaming handler, under `RunStreamPipes` | `TestStreamRoutePartitionsAndHandsDeclaredOperations` (`cmd/ze/hub/`) |
| A monitor line with a chain over SSH exec, with and without the rendering key | → | the stream arm of `execMiddleware` passes the typed line → the closure in `service_ssh.go` → `GetStreamingHandlerForCommand` | `test/plugin/monitor-pipe-chain-ssh.ci`, `test/plugin/monitor-rendering-key-keeps-selection.ci` |
| A monitor line with a chain typed in the daemon-hosted model | → | `extractMonitorCmdArgs` → `GetStreamingHandlerForCommand` | `TestModelMonitorHandsDeclaredOperations` (`internal/component/cli/`) |
| `show bgp peer <selector> rib \| received \| count` on SSH exec | → | `Dispatch` partition on `show bgp peer rib` → `forwardRibRoutes` with the pipe list → the RIB's stages | `test/plugin/rib-peer-pipe-scope.ci` |
| `ze cli -c` with a monitor line and `\| save` | → | `StreamMonitor` sends the typed line and the session key; the client formats and saves | `test/ui/cli-monitor-pipe-typed-line.ci` |
| A local-data command with a chain, in `ze cli -c` and through the daemon | → | `ServeLocal` → the partition → `ValidatedArgs`; `Dispatch` → `loadLocalData` | `TestServeLocalHandsDeclaredOperations` (`internal/component/command/`), `test/ui/local-data-pipe-operation.ci` |
| REST execute with parameters and a chain | → | `prepareCommand` → `buildInvocation` writes at the pipe offset | `TestBuildInvocationWritesParamsBeforePipe` (`internal/component/api/`) |
| The same piped line on SSH, the web terminal, `ze cli -c`, MCP, REST, gRPC and `dispatch-command` | → | each route into the one partition | `TestPipedLineParityAcrossSurfaces` (`cmd/ze/hub/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show bgp rib \| received \| count` on SSH exec | The RIB handler receives the command `show bgp rib` and the operation list `received`, `count` in that order; the answer prints the count; the accounting start record holds the line as typed | none |
| AC-2 | The same line from a plugin's `dispatch-command`, internal and forked | The answer is done with data `{"count": N}`, N the received route count | none |
| AC-3 | The same line on MCP `ze_execute`, REST execute and gRPC `Execute` | Each answers the count in its envelope, the value `ze cli -c` prints for the line with `\| json` | none |
| AC-4 | A piped line whose operations the command does not declare (`show bgp \| peers \| count`) from a plugin, MCP, REST and gRPC | The generic chain is applied by the engine and the answer equals what `ze cli -c` prints with `\| json` | none |
| AC-5 | A format operation (`\| table`, `\| text`, `\| yaml`) on a data caller: a plugin's `dispatch-command`, REST execute, gRPC `Execute` | The answer is done and its data is the rendered text, byte for byte what `ze cli -c` prints for the line (decided, T-1) | none: a format is rendered for every caller |
| AC-6 | A double-quoted value holding a pipe character or a space, in the command part and in the chain (`\| match "error\|warn"`, `\| match "link down"`), on every surface | The handler or the operation receives the decoded value as one argument, its quotes removed, and the chain has exactly the operations typed outside quotes | a backslash outside quotes in the chain stays a literal character, because `match` patterns rely on it |
| AC-7 | A line with an unfinished double quote, before or after the first pipe, on every surface | Refused with the error naming the column of the opening quote and the fix "add the closing double quote"; no handler runs. The command-part half already holds (`ScanExecution`); this spec adds the chain half | none |
| AC-8 | `show bgp rib received count`, `show bgp rib best count` and `show bgp rib protocol bmp peer <key>` (the argument form of each reader), on every surface | Refused by the engine's argument validator, before any handler or plugin runs, naming the word as a pipe operation of its command, with the pipe form of the same line as its fix (`show bgp rib \| received \| count`). The same refusal answers any command that declares an operation and is handed its name as an argument | the protocol name after `show bgp rib protocol` stays an argument: it names the subject |
| AC-9 | `bgp-rib` run external | AC-1 and AC-2 hold over the forked transport | none |
| AC-10 | A line with no pipe outside quotes, on any surface | Answers exactly as today | every existing answer |
| AC-11 | A REST or gRPC parameter value holding `\|` or `"` | Written by `WriteInvocation` as one quoted argument; the handler receives the value whole and the line gains no operation. `BuildCommand` is deleted, and `ValidateParams` keeps its whitespace refusal | a value holding whitespace stays refused, as today |
| AC-12 | Every in-tree argument-form RIB line in the census | Converted to the pipe form; its test or caller answers as before | none |
| AC-13 | An unknown operation on a command that declares a set | Refused as today by `unknownFilterError`, naming the valid set | none |
| AC-14 | A plugin's `Pipes` entry with an `argument` field and no expansion, on one of its own commands | Recorded into `pipeFilterRegistry` under the plugin; its command receives the operation through `execute-command`; withdrawn on a failed startup, a removal from the config and a restart; an entry with an expansion is an alias exactly as today; an entry with both fields, or with neither, is refused at Stage 1 by name | none |
| AC-15 | (T-5 = C) the derived MCP tool and the REST and gRPC route of `show bgp rib`, with the optional ordered `pipe` list holding `received` then `count` | Each answers the count `show bgp rib \| received \| count` answers; a name the command's `pipe-operations` does not hold is refused naming the valid list; with the list absent the answer is the bare command's | the element order is the chain order |
| AC-16 | A generic row operation (`count`, `first`, `last`, `match`, `display`) on a command that declares none and answers rows, from every caller | Each caller receives the generic's answer over that command's rows, identical across callers; a command answering one document gets the generic's refusal by name | none |
| AC-17 | A declaration whose name is an overridable generic's (any data operator: the RIB's `count`, `first`, `last`, `match`; `display`, `fill`, `resolve`, `origin` for a command that declares them), in-tree or through `Pipes` | The handler runs it. An argument that disagrees with the catalog is refused when recorded. When the override is the last declared operation and the answer is done, the answer is held to the catalog's shape: `count` one object with an integer `count`; `first N` and `last N` the rows envelope with at most N rows on an answer that is not streamed; every other data operator the rows envelope; an answer of another shape is refused by name | `match` is held by its test only (`TestRIBOverridesKeepGenericMeaning`); an error answer and an override followed by another declared operation are not checked |
| AC-18 | A declaration that means something else (`histogram`, `graph`, `reason`) | It carries a name the catalog does not hold, and help, the catalog and completion show its own description | none |
| AC-19 | `show command help "<name>"` and the `ze help command --json` row for a builtin with declared operations (`show bgp rib`), a command declaring none (`show interface`), a command with aliases (`show bgp`), and a plugin command with a `Pipes` operation | Both answer one identical `pipe-operations` list, each entry with `name`, `source`, `argument`, `available`, `description`, `local-only` for a generic, and `expansion` for an alias; the generics listed are exactly those the chain validator admits for that command; `pipe-filters`, `pipe-aliases`, `operators` and `pipes` are absent. The plugin command's row is asserted against the daemon's answer for an external plugin. Read offline, with no daemon, the catalog holds what the process's own registries hold: a compiled-in plugin's operation reaches it from `registry.Registration` `Pipes`, as its aliases do today (`aliasesFor`, `internal/plugins/help/help_command.go`), and the two lists are equal; an external plugin's command is absent offline, and its operations with it, because Ze starts no external plugin's program without a daemon | none: pre-release, every reader is converted. An external plugin's operations are absent offline together with its commands, never one without the other |
| AC-20 | `\|` then Tab after each command of AC-19, in the editor and in `ze cli` | The names and descriptions offered are exactly that command's `pipe-operations` list | none |
| AC-21 | Every in-tree reader of the old help and catalog keys in the census | Reads `pipe-operations`, with `class` kept for a generic and an override; `operatorsFor` and its copy in `internal/le/cli/catalog/catalog.go` are gone | none |
| AC-22 | A monitor line with a chain on the stream route, with no rendering key: the web, REST stream, gRPC stream and SSH exec | `GetStreamingHandlerForCommand` partitions; the streaming handler receives its declared operations in `ValidatedArgs`; the engine applies the whole residual chain through `RunStreamPipes`, a format included; accounting holds the typed line | a monitor that declares no operation answers as today; `\| save` stays refused, because the daemon never writes a caller's file |
| AC-23 | `ze cli -c` with a monitor line holding a chain and `\| save` | The client sends the typed line and the rendering key; the daemon accounts the typed line, hands the declared operations to the handler, applies every residual operation before the presentation suffix and answers data; the client applies the suffix alone: it prints each event in the format typed, or the default one, and writes the file. Without the key the daemon applies the whole chain | the client applies only the trailing run of presentation operations (formats, paging, `save`); an alias is never part of it |
| AC-24 | A local-data command with a declared operation and a chain, in `ze cli -c` and from a daemon surface | `ServeLocal` and `Dispatch` call the same partition; the handler receives the same declared list on both routes and the two answers are equal | a local-data command that declares nothing answers as today |
| AC-25 | A declared operation typed after a residual one (`show bgp rib \| resolve \| count`, `show bgp rib \| table \| peer 10.0.0.1`) | The generic-named one stays residual and runs as the generic over the answer in hand; the own-named one is refused by name with the reordered line as its fix | an operation typed before the first residual one is taken as today |
| AC-26 | A scope operation of `show bgp rib` at any position of the declared list (`\| family ipv4/unicast \| received \| count`); two scopes | The RIB answers as for the scope typed first in the list; two scopes are refused naming both | none |
| AC-27 | (T-6 = A) a piped line a per-command policy denies, on SSH, the web, `ze cli -c`, MCP, REST, gRPC, `dispatch-command` and the stream route | A string authorizer receives the typed line; a typed one the command, its arguments, then the chain as tokens; the denied line is refused with the permission-denied answer of its surface and no handler runs | the handler's arguments never hold a chain token |
| AC-28 | A read-only caller sending a write command with a chain (`request ... \| count`), on REST execute, REST stream, gRPC and every surface that admits a read-only caller | Refused with the permission-denied answer: read-only is judged on the command path and a chain never changes the verdict | a read-only caller's piped read command is answered |
| AC-29 | A line whose command part holds a secret and whose chain holds operations | The accounting record holds the typed line with the secret masked by `config.DisplayCommand` and the chain intact, after the masked part | none |
| AC-30 | A plugin's declared operation that collides: a generic the catalog does not allow to be overridden (a format, `no-more`, `save` or `log`); an alias on an overlapping path; a global alias; a path that holds another owner's set | The whole Stage 1 declaration is refused with an error naming the operation and what it collides with; nothing of the batch is recorded; the daemon keeps running. The same in-tree is a `BUG:` panic | none |
| AC-31 | `show bgp rib \| sent`, `\| sent-received`, `\| aspath <pattern>`, and each name of the explicit set of `show bgp rib protocol <protocol>` | Each answers what its argument form answers today; `show command help` lists them; a scope operation on `show bgp rib protocol` is refused naming the valid set | the route output of the removed `json` terminal is the bare command's on `show bgp rib` and `show bgp rib protocol`; on `show bgp rib best` the terminal's `routes` shape goes and the `best-path` rows stay (A-11, D-7) |
| AC-32 | A REST or gRPC command holding a chain, with structured parameters | The parameters are written before the first pipe and reach the handler as arguments; the chain is applied to the answer | none |
| AC-34 | `show bgp peer <selector> rib received` (the scope as a YANG leaf), and `show bgp peer <selector> rib \| received \| count` | The first is refused by the engine's argument validator naming `received` as a pipe operation of `show bgp peer rib`, with the pipe form as its fix; the second answers the count of that peer's received routes; `show command help` lists the command's own operation set, scope included | the peer selector stays on the path: it names the subject |
| AC-35 | A monitor line with a chain on each of the four stream lookup sites (web, REST and gRPC stream, SSH exec, the daemon-hosted model) | Each receives the declared list from `GetStreamingHandlerForCommand`; on SSH exec the accounting record and the authorizer hold the typed line, chain included | a monitor declaring nothing answers as today |
| AC-36 | A client sets the rendering key and sends `monitor x \| match up \| save f` | The daemon applies `match`: the client receives only matching events; the daemon writes no file; the client formats and saves | the client applies only the trailing run of presentation operations |
| AC-37 | The rendering key with a value other than the defined one; the key on one-shot exec | The first is refused by name before any command runs; the second changes nothing: the daemon renders the whole chain | none |
| AC-38 | A chain argument holding a backslash outside quotes (`\| match a\b`), authorized and accounted | The authorizer and the accountant receive tokens from the partition's scan, so the line is judged and recorded with the backslash intact, and an allowed line runs | a backslash outside quotes in the command part stays refused |
| AC-39 | (T-5 = C) a `pipe` list element whose name or argument word holds a pipe character or a quote | A name holding one is refused naming the valid list; an argument word holding one is written quoted and reaches the operation as one word; the chain gains no operation | none |
| AC-33 | The same piped lines sent on SSH, the web terminal, `ze cli -c`, MCP, REST, gRPC and `dispatch-command` | Each surface's answer holds the same data as the local `ze` command's for that line, for a declared chain, a generic chain, a format, a quoted value and a refused line | each surface keeps its own envelope |
| AC-40 | A web package test that commits a config holding a bgp block (`TestWebTerminalChangesDisplaysInactiveCandidateUntilCommit`, `TestBrowserCommandCommitAudit`, `TestWebRollbackStagesBeforeTheCommitHook`) | The engine stand-in `installPeerValidator` also registers a BGP peer summarizer and resets it in cleanup, so the commit is judged and not refused with "this binary was built without the BGP engine"; the fail-closed guard in `infra.SummarizeBGPPeers` is unchanged for a product binary | none |
| AC-41 | A dispatched line that is missing its selector (`TestSelectorRefusalPlacesTheSelectorWhereDispatchReadsIt`, `TestDispatchImplicitSelectorMissingValue`) | Refused with `missingSelectorError`'s text, naming where the selector goes, not the generic missing-argument text b5a7d0dfe7 let through | none |
| AC-42 | `bgp-gr` re-advertises a peer's stale routes by dispatching `clear bgp rib out-excluding-owner owner N family F` (`TestClearOutProxyPreservesCapturedOwnerExclusion`) | The command's container declares `owner` (uint64 1..max, mandatory, keyword-only) and `family` (address family, keyword-only), so both bind and the replay runs; it is not refused as "The original-owner replay exclusion is incomplete or has an unbound argument" | none |
| AC-43 | Every path the published OpenAPI document names (`TestOpenAPIPublishesNoPerCommandPath`, `TestRESTEveryPublishedPathIsServed`) | Each published route is one `registerRoutes` serves; no `/api/v1/execute/<command path>` operation is published, and each command is described as a request-body schema | none |
| AC-44 | `show bgp rib \| received \| match <community>` for a route carrying that large or extended community (`TestFilterMatchCrossField`, `TestMatchPoolCommunities`, `community-relation-tag.ci`) | The route is selected: `match` searches large and extended communities on the text the route row shows | none |
| AC-45 | `monitor ... \| match x` over a stream whose events are one document each (`TestStreamLineMatchKeepsWholeEvents`, `TestStreamLineMatchNeverSplitsAnEvent`, `monitor-pipe-chain-ssh.ci`) | A JSON event is kept whole when one of its values holds the text and dropped whole otherwise, never split or emptied; a text event is matched line by line | none |
| AC-46 | A `ze cli -c` monitor with `\| save f` stopped by SIGINT or SIGTERM (`TestStreamMonitorTerminateCommitsSaves`, `cli-monitor-pipe-typed-line.ci`) | The save commits the file, no `.ze-save` temporary is left, and the client exits 0 | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | an operator counts received routes over SSH | SSH → `execMiddleware` → `Dispatch` → partition → `forwardRibRoutes` → `execute-command` with the pipe list → `showPipeline` count terminal → formatter | `test/plugin/rib-pipe-filter.ci` |
| 2 | a plugin asks the engine for a count | `dispatch-command` → `Dispatch` → the same path → data `{"count": N}` | `test/plugin/dispatch-pipe-chain-applied.ci` |
| 3 | an AI agent counts routes over MCP | `ze_execute` → `serverDispatcher` → `Dispatch` → answer | `test/plugin/mcp-pipe-chain-applied.ci` |
| 4 | an operator matches log text that holds a pipe character or a space, in quotes | SSH → the one grammar keeps the quoted value whole and removes its quotes → `match`, a case-insensitive substring | `test/ui/pipe-quoted-value-is-an-argument.ci` |
| 7 | an operator watches a monitor through a chain, over the web and over `ze cli -c` with `\| save` | the stream route → the partition → the handler; the engine renders, or the client when its session says so | `test/plugin/monitor-pipe-chain-ssh.ci`, `test/ui/cli-monitor-pipe-typed-line.ci` |
| 8 | an operator runs a local-data command with a chain, locally and through the daemon | `ServeLocal` or `Dispatch` → the same partition → the same handler | `test/ui/local-data-pipe-operation.ci` |
| 9 | a REST client sends a command with parameters and a chain | `prepareCommand` → parameters before the pipe → `Dispatch` | `test/plugin/rest-pipe-chain-applied.ci` |
| 10 | an operator denied a command by policy types it with a chain | the route → accounting of the typed line → the authorizer → the permission-denied answer | `test/plugin/pipe-chain-unauthorized-refused.ci` |
| 11 | an operator counts the routes one peer sent, by peer name | SSH → `Dispatch` → partition on `show bgp peer rib` → `forwardRibRoutes` with the selector and the pipe list → `showPipeline` | `test/plugin/rib-peer-pipe-scope.ci` |
| 12 | a `ze cli -c` operator saves a filtered monitor to a file, and cannot use the session key to see more | the client sends the typed line and the key → the daemon applies `match` → the client formats and saves | `test/ui/cli-monitor-pipe-typed-line.ci`, `test/plugin/monitor-rendering-key-keeps-selection.ci` |
| 5 | a third-party plugin offers its own operation | Stage 1 `Pipes` → `registerPluginPipes` → `pipeFilterRegistry` → `Dispatch` → `execute-command` | `test/plugin/plugin-pipe-operation.ci` |
| 6 | an operator or an AI asks what a command takes after a pipe | `show command help` → `commandHelp` → the introspection; `\|` Tab → `completePipe` → the same | `test/plugin/command-help-pipe-operations.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitPipeLineQuoteAware` | `internal/component/command/pipe_test.go` | the first pipe outside quotes ends the command; a quoted pipe stays; each further unquoted pipe ends an operation | |
| `TestChainGrammar` | same | one row per rule of "The chain grammar": a quoted run is one word with its quotes removed; the escapes inside quotes; a backslash outside quotes stays literal; a multi-word quoted value stays one word of the declared list; an alias expansion reads by the same rules | |
| `TestSplitPipeLineUnfinishedQuoteInChain` | same | AC-7: an unfinished quote after the first pipe is refused with its column | |
| `TestPartitionDeclaredOperations` | same | declared operations leave the chain in order; the command part is unchanged text; the `pipeChainMeta` equals what `collectPipeMeta` answers for the whole chain | |
| `TestPartitionTakesDeclaredOnlyAsAPrefix` | same | AC-25: after the first residual operation a generic-named operation stays residual and an own-named one is refused with the reordered line | |
| `TestPartitionValidatesBeforeTaking` | same | `validateKnownPipeArguments` and `validateCountDocumentFollowers` refuse before any operation is taken | |
| `TestEveryPipeSplitGoesThroughThePartition` | same | R-11: no non-test caller of the retired split and fold entry points remains | |
| `TestOverridableComesFromTheCatalog` | same | AC-30: a declaration naming a generic the catalog does not allow to be overridden panics in-tree | |
| `TestPluginOperationCollisionsRefused` | `internal/component/command/pipe_filter_test.go` | AC-30, A-6: each collision of the table refuses the whole batch with an error and records nothing | |
| `TestServeLocalHandsDeclaredOperations` | `internal/component/command/local_data_test.go` | AC-24 | |
| `TestHasFormatPipeReadsThePartition` | `internal/component/command/pipe_test.go` | a format after a declared operation is found; a declared operation is never read as a format | |
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
| `TestOverrideAnswerShapeRefused` | `internal/component/command/pipe_test.go` | AC-17: a wrong shape from the last declared override on a done answer is refused; `first N` with more than N rows on an answer that is not streamed is refused | |
| `TestOverrideShapeCheckScope` | same | AC-17: an error answer, an override followed by another declared operation, and a streamed answer are not checked | |
| `TestOverrideShapeCheckedOnEveryRoute` | `cmd/ze/hub/pipe_parity_test.go` | AC-17: the same wrong-shaped override is refused through `Dispatch`, through `ServeLocal` and on the stream route | |
| `TestDeclaredArgumentContractValidatedByPartition` | `internal/component/command/pipe_test.go` | each argument kind of the closed vocabulary (`none`, `optional`, `text`, `count`, `fields`, `path`) on a declared operation, in-tree and from a plugin: a missing, surplus or ill-typed argument is refused by the partition before the handler runs | |
| `TestPipeFilterArgumentKindRecorded` | `internal/component/command/pipe_filter_test.go` | a `PipeFilter` and a plugin's `PipeDecl` `argument` record the same field; a spelling outside the vocabulary is refused when recorded | |
| `TestCatalogStatesPresentationOperators` | `internal/component/command/pipe_test.go` | the presentation fact holds for the formats, `no-more` and `save` and for no operator that selects or reduces | |
| `TestPresentationSuffixSameOnClientAndDaemon` | same | AC-36, R-10: each row of the suffix table, the alias row and the unknown-name row included, gives one split of the chain | |
| `TestAuthorizerTokensKeepChainBackslash` | `internal/component/plugin/server/command_test.go` | AC-38, R-13: `\| match a\b` reaches a typed authorizer as tokens with the backslash intact and an allowed line runs; a backslash outside quotes in the command part stays refused | |
| `TestAccountantTokensKeepChainBackslash` | same | AC-38: the typed accountant receives the same tokens, not the line as one token | |
| `TestStreamingLookupPartitions` | `internal/component/plugin/server/handler_test.go` | AC-35: `GetStreamingHandlerForCommand` handed a typed line answers `ValidatedArgs` carrying the declared list, and refuses an unfinished quote in the chain | |
| `TestExecStreamPassesTypedLine` | `internal/component/ssh/ssh_test.go` | AC-35: the `StreamingExecutor` receives the line as typed, chain included | |
| `TestSSHExecStreamAccountsAndAuthorizesTypedLine` | `cmd/ze/hub/service_ssh_test.go` | AC-35: the accountant and the authorizer of the SSH exec stream hold the typed line | |
| `TestAPIStreamAppliesChain` | `cmd/ze/hub/api_test.go` | AC-22, AC-35: a REST or gRPC stream line with a chain delivers only the events the chain keeps | |
| `TestModelMonitorHandsDeclaredOperations` | `internal/component/cli/model_monitor_test.go` | AC-35 on the daemon-hosted model | |
| `TestRenderingKeyNeverSkipsSelection` | `cmd/ze/hub/service_ssh_test.go` | AC-36, R-12, security negative: with the key set, `monitor x \| match up \| save f` delivers no event without `up`, and the daemon writes no file | |
| `TestRenderingKeyUnknownValueRefused` | `internal/component/ssh/session_test.go` | AC-37, security negative: a value other than the defined one is refused by name before any command runs, on the exec channel and the interactive one | |
| `TestRenderingKeyIgnoredOnOneShotExec` | same | AC-37: the key on a one-shot exec changes nothing, the daemon renders the whole chain | |
| `TestTypedWrappersPassChainTokens` | `cmd/ze/hub/aaa_lifecycle_test.go` | AC-27 under T-6 A: each of the six wrappers hands a typed authorizer the chain tokens unchanged, and a string one their canonical form with argument boundaries kept | |
| `TestTacacsFallbackArgsPassesChainTokens` | `internal/component/tacacs/authorizer_test.go` | AC-27 under T-6 A: on a server error the local fallback judges the same tokens the server was sent | |
| `TestPipeListElementCannotOpenAChain` | `internal/component/api/invocation_test.go` | AC-39, security negative: a name holding a pipe or a quote is refused naming the valid list; an argument word holding one is written quoted and adds no operation | |
| `TestDerivedToolPipeListElementCannotOpenAChain` | `internal/component/mcp/tools_test.go` | AC-39 on a derived MCP tool | |
| `TestPeerRibDeclaresItsOperations` | `internal/component/bgp/plugins/cmd/rib/rib_test.go` | AC-34: `show bgp peer rib` holds its own set, the scope words included, and `show bgp peer` below it inherits none | |
| `TestPeerRibScopeFromPipeOperations` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-34: the peer selector and a scope operation give the routes of that peer in that direction | |
| `TestValidateArgsRefusesPipeOperationWord` | `internal/component/command/argbind_test.go` | AC-8, AC-34: a word the command's `pipe-operations` holds as an operation is refused with the pipe form as its fix, on a command with a definition still open, on one whose definitions are all filled, and on one that declares none; a word that is no operation is judged as today | |
| `TestPeerRibRefusesScopeArgument` | `internal/component/plugin/server/command_test.go` | AC-34: `show bgp peer <selector> rib received` is refused by the dispatcher before `forwardRibRoutes` runs, naming `received` as a pipe operation, with the pipe form as its fix | |
| `TestRIBScopeKeywordsMatchTheDeclaredOperations` | `internal/component/bgp/plugins/rib/yang_enum_agreement_test.go` | replaces `TestRIBScopeKeywordsMatchTheYANGModel`: `scopeKeywords` resolves every declared scope operation and holds no other word (test-judge) | |
| `TestRIBRefusalsNameThePipeForm` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | no fix line the RIB prints is the argument form (`show bgp rib count`) | |
| `TestDispatchAccountsTypedLineWithMaskedSecret` | same | AC-29 | |
| `TestTypedAuthorizerReceivesChainTokens` | same | AC-27: the typed authorizer receives the chain after the arguments and the handler's arguments hold none | |
| `TestPipedLineUnauthorizedRefused` | same | AC-27: a denied piped line runs no handler | |
| `TestPluginPipeDeclWithArgumentIsAnOperation` | `internal/component/plugin/server/startup_test.go` | AC-14 | |
| `TestPluginPipeDeclBothOrNeitherRefused` | same | AC-14: an entry with `expansion` and `argument`, and one with neither, are refused by name | |
| `TestPluginOperationWithdrawnOnStop` | same | A-6, on each of the three paths a plugin leaves by | |
| `TestStreamRoutePartitionsAndHandsDeclaredOperations` | `cmd/ze/hub/server_stream_test.go` | AC-22 | |
| `TestPipedLineParityAcrossSurfaces` | `cmd/ze/hub/pipe_parity_test.go` | AC-33: one table of lines, sent on SSH, the web terminal, `ze cli -c`, MCP, REST, gRPC and `dispatch-command`, each compared with the local answer | |
| `TestPipedWriteRefusedForReadOnlyCallerOnEverySurface` | same | AC-28 | |
| `TestExecSessionRendersStatement` | `internal/component/ssh/session_test.go` | A-10, AC-23: the key is validated, read on the exec channel, and absent means the daemon renders | |
| `TestBuildInvocationWritesParamsBeforePipe` | `internal/component/api/invocation_test.go` | AC-32 | |
| `TestInvocationKeepsPipeAndQuoteInValue` | same | AC-11 | |
| `TestStreamPipedReadOnlyCaller` | `internal/component/api/engine_test.go` | A-1 on the stream endpoint | |
| `TestDerivedToolPipeList` | `internal/component/mcp/tools_test.go` | AC-15 | |
| `TestRIBScopeAtAnyPosition` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | AC-26 | |
| `TestRIBDeclaresEveryWordItAccepted` | same | AC-31: `sent`, `sent-received`, `aspath`, and the explicit set of `show bgp rib protocol` | |
| `TestRIBJSONTerminalEqualsBareCommand` | same | A-11, written and run before the terminal is removed: equal documents on `show bgp rib` and `show bgp rib protocol`; on `show bgp rib best` the same family, prefix and winning peer per row in both shapes | |
| `TestRIBBestAndProtocolStagesFromPipeOperations` | same | the second and third reader build the same stages from the list | |
| `TestSDKDeliversPipeOperations` | `pkg/plugin/sdk/sdk_callbacks_test.go` | the decoder passes the list to the handler | |
| `TestRIBStagesFromPipeOperations` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | the same stages from the list | |
| `TestRIBRefusesArgumentForm` | `internal/component/plugin/server/command_test.go` | AC-8: each argument form is refused by the dispatcher and the RIB handler is not called | |
| `TestRIBOverridesKeepGenericMeaning` | same | AC-17, `direction` included | |
| `TestExecutePipedReadOnlyCaller` | `internal/component/api/engine_test.go` | A-1, AC-28 | |
| `TestZeExecuteAppliesPipeChain`, `TestRESTExecuteAppliesPipeChain`, `TestGRPCExecuteAppliesPipeChain` | the mcp, rest and grpc packages | AC-3 | landed d30af7c2f1 |

Tests added during phases 3a to 3c and their fix rounds (all landed in d30af7c2f1):

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPartitionDeclaredOperations` (was `TestFoldFilters`, judge-approved) | `internal/component/command/pipe_test.go` | the declared list in chain order, the residual kinds, the whole-chain meta; the command part as typed | landed |
| `TestPartitionTakesDeclaredOnlyAsAPrefix`, `TestPartitionValidatesBeforeTaking`, `TestPartitionTakesParsedOverridesAndAliases` | `internal/component/command/pipe_partition_test.go` | AC-25; validation before taking; an alias and a parsed override reach the declared list | landed |
| `TestDeclaredArgumentContractValidatedByPartition`, `TestOverridableComesFromTheCatalog`, `TestOverrideArgContractRefusedAtRegistration`, `TestOverrideAnswerShapeRefused`, `TestOverrideShapeCheckScope`, `TestCatalogStatesPresentationOperators`, `TestHasFormatPipeReadsThePartition`, `TestValidateArgsRefusesPipeOperationWord` | same | AC-17, AC-30 (in-tree half), AC-8; the catalog's override and presentation facts | landed |
| `TestPartitionSameOnSurfaceAndDispatch`, `TestDispatchLineTypedTokens`, `TestDispatchLineHasNoAnswerStepWithoutChain` | `internal/component/command/dispatch_line_test.go` | A-4; the typed tokens the authorizer and the accountant read | landed |
| `TestDeclaredLineCarriesOnlyDeclaredOperations` | `internal/component/command/dispatch_line_declared_test.go` | what a cross-process renderer sends | landed |
| `TestDispatchCommandAppliesPipeChain` (with its socket arm), `TestDispatchReadsJoinedPipeAsChain`, `TestDispatchRefusesChainWithNoCommand`, `TestDispatchTrailingPipeAnswersTheCommand`, `TestDispatchQuotedPipeArgumentWithChain`, `TestDispatchPluginCommandReceivesCommandPartOnly`, `TestDispatchAccountsRefusedChain`, `TestPipeRefusalsMaskSecrets`, `TestDispatchCommandSocketRefusesPipeOperationWord`, `TestDispatchCommandAnswerRefusesPipeOperationWord`, `TestDispatchKeepsQuotedPipeArgument` (moved) | `internal/component/plugin/server/dispatch_pipe_chain_test.go` | AC-2 (bridge half), AC-4, AC-6, AC-8, AC-29 (refusal masking), RFC 8907 Section 8.3 accounting of a refused chain | landed |
| `TestRenderedAnswerMarshalsAsJSONString`, `TestDispatchJSONFormatAnswersTheDocument`, `TestDispatchCommandCarriesRenderedText` | `internal/component/plugin/server/dispatch_rendered_answer_test.go` | AC-5: a text format answers its rendered text, `\| json` the document | landed |
| `TestRESTExecuteAnswersRenderedText`, `TestGRPCExecuteAnswersRenderedText`, `TestZeExecuteAnswersRenderedText` | the rest, grpc and mcp packages | AC-5 on each data surface | landed |
| `TestTacacsFallbackArgsPassesChainTokens` | `internal/component/tacacs/authorizer_chain_test.go` | AC-27 under T-6 A | landed |
| `TestTypedWrappersPassChainTokens` | `cmd/ze/hub/aaa_lifecycle_test.go` | AC-27 under T-6 A | landed |
| `TestExecOneShotSendsTheTypedLine` | `internal/component/ssh/exec_typed_line_test.go` | AC-1, AC-33 rendering half | landed |
| `TestOperationalCommandSendsTheTypedLine` | `internal/component/cli/model_mode_typed_line_test.go` | same, the CLI model | landed |
| `TestWebSurfacesSendTheTypedLine` | `internal/component/web/typed_line_test.go` | same, the web terminal and tool page | landed |
| `TestEditorExecutorSendsTheDeclaredOperations` | `internal/component/config/cli/cmd_edit_declared_test.go` | the editor sends the declared line | landed |
| `TestPipedEngineDispatchSendsTheDeclaredOperations` | `internal/test/cli/cmd_engine_steps_declared_test.go` | the functional-test engine step | landed |
| `TestSSHDaemonExecutorRendersResidualChain`, `TestSSHStandaloneExecutorRendersResidualChain`, `TestAttachedCLIExecutorRendersResidualChain` | `cmd/ze/hub/executor_renders_chain_ssh_test.go`, `attached_cli_executor_test.go` | each rendering executor states that it renders | landed |
| `TestInvocationKeepsPipeAndQuoteInValue`, `TestBuildInvocationWritesParamsBeforePipe` | `internal/component/api/invocation_test.go` | AC-11, AC-32 | landed |
| `TestExecutePipedReadOnlyCaller` | `internal/component/api/engine_test.go` | A-1, AC-28 engine half | landed |
| `TestPipedLineParityAcrossSurfaces` (data surfaces, formats included) | `cmd/ze/hub/pipe_parity_test.go` | AC-33, data-surface half | landed |

Tests added in phase 4 (4a in 07f5b7e39c, 4b in a374677264):

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPresentationSuffixSameOnClientAndDaemon`, `TestStreamLineAppliesChainAndConsumesLog`, `TestStreamLineRefusesDaemonSave`, `TestStreamLineWithoutChain` | `internal/component/command/stream_line_test.go` | R-10, AC-22, AC-23 (the suffix split); `\| save` never runs in the daemon | landed 07f5b7e39c |
| `TestStreamingLookupPartitions`, `TestEveryPipeSplitGoesThroughThePartition` (streaming-registry half) | `internal/component/plugin/server/streaming_lookup_test.go` | AC-35, R-11; the stream event keeps the `pipe` metadata | landed 07f5b7e39c |
| `TestExecStreamPassesTypedLine`, `TestExecSessionRendersStatement`, `TestRenderingKeyNeverSkipsSelection`, `TestRenderingKeyUnknownValueRefused`, `TestRenderingKeyIgnoredOnOneShotExec`, `TestExecIgnoresInteractiveKeys` | `internal/component/ssh/exec_stream_test.go` | AC-35, AC-36, AC-37, A-10, R-12, AC-10 (the interactive keys stay ignored on exec) | landed 07f5b7e39c |
| `TestSSHExecStreamAccountsAndAuthorizesTypedLine` | `cmd/ze/hub/service_ssh_stream_test.go` (tags `ze_web ze_bgp ze_ssh`) | AC-35 on SSH exec | landed 07f5b7e39c |
| `TestModelMonitorHandsDeclaredOperations` | `internal/component/cli/model_monitor_test.go` | AC-35 on the daemon-hosted model | green with 07f5b7e39c |
| `TestEveryPipeSplitGoesThroughThePartition` (split and fold half), `TestReadCommandPartReadsTheOneGrammar`, `TestReadLocalLineHandsDeclaredOperationsAndRendersTheResidual` | `internal/component/command/pipe_split_partition_test.go` | R-11; `ReadCommandPart`, `ReadLocalLine` | landed a374677264 |
| `TestLocalDataDeclaredOperationsSameOnBothRoutes` | `internal/component/plugin/server/local_data_pipe_parity_test.go` | AC-24 on both routes | landed a374677264 |
| `TestRunHandsDeclaredOperationsNotArgv` | `internal/le/le/root/run_declared_test.go` | `le` hands its declared operations as a list, never appended to argv | landed a374677264 |
| `TestModelReadsTheCommandPartUnderTheOneGrammar` | `internal/component/cli/model_command_part_test.go` | the ping, traceroute and stream detectors read the one grammar | landed a374677264 |
| `TestLiveViewsHandDeclaredOperationsToTheirHandler` | `internal/component/cli/model_stream_declared_test.go` | the ping and traceroute live views carry the declared list (`PipeFlags.Declared`, the context) | landed a374677264 |

Tests added in phases 5 to 7a (5a and 5b in 3242d2dfb3; 5c, 5d, B1 and B2 in 615753f1c6; phase 6 and 7a in 3c642127ca) and with the D-5 widening:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExecuteCommandInputCarriesPipe`, `TestPipeDeclArgumentWire`, `TestDirectBridgeExecuteCommandCarriesPipe` | `pkg/plugin/rpc/execute_command_pipe_test.go` | the pipe list and the `argument` field on the wire | landed 3242d2dfb3 |
| `TestExecuteCommandHandlerReceivesPipeFromTheWire`, `TestExecuteCommandHandlerReceivesPipeOverTheBridge` | `pkg/plugin/sdk/sdk_execute_pipe_test.go` | stands for `TestSDKDeliversPipeOperations`: the SDK hands the list to the handler, forked and internal | landed 3242d2dfb3 |
| `TestPluginOperationCollisionsRefused`, `TestPluginOperationAccepted`, `TestPipeFilterArgumentKindRecorded`, `TestPluginOperationWithdrawnRestoresThePath` | `internal/component/command/pipe_filter_plugin_test.go` | AC-14, AC-30, A-6 | landed 3242d2dfb3 |
| `TestPluginPipeDeclWithArgumentIsAnOperation`, `TestPluginPipeDeclBothOrNeitherRefused`, `TestPluginPipeDeclRefusedAliasWithdrawsOperations`, `TestPluginOperationWithdrawnOnStop`, `TestRouteToProcessCarriesPipe` | `internal/component/plugin/server/startup_pipe_operation_test.go` | AC-14, A-6, the carrier half of AC-2 and AC-9 | landed 3242d2dfb3 |
| `TestDispatchCarriesPipeOperationsToThePluginRequest` | `internal/component/plugin/server/dispatch_pipe_chain_test.go` | the declared list reaches the plugin request; a plain line carries none | landed 3242d2dfb3 |
| `TestRIBJSONTerminalEqualsBareCommand` | `internal/component/bgp/plugins/rib/rib_json_terminal_test.go` | A-11, D-7 | landed 615753f1c6 |
| `TestRouteReadTakesScopeAtAnyPosition`, `TestRouteReadRefusesTwoScopes`, `TestRouteReadRefusesArgumentForm`, `TestPlainCommandRefusesAPipeOperation`, `TestProtocolRefusesScopeOperation`, `TestDeclaredFilterValueWords`, `TestForkedRIBReceivesTheDeclaredList` | `internal/component/bgp/plugins/rib/rib_declared_pipe_test.go` | AC-26, AC-8, AC-31 (protocol scope refused), AC-9 over the RPC pipe | landed 615753f1c6 |
| `TestRIBScopeKeywordsMatchTheDeclaredOperations` | `internal/component/bgp/plugins/rib/scope_operations_agreement_test.go` | replaces `TestRIBScopeKeywordsMatchTheYANGModel` (judge-approved) | landed 615753f1c6 |
| `TestPeerRibDeclaresItsOperations`, `TestRibDeclaresTheWordsItsReadersAccept`, `TestProtocolRibDeclaresItsOwnSet`, `TestProtocolRibRefusesScopeOperation`, `TestProtocolRibHandsItsOperationsToTheHandler` | `internal/component/bgp/plugins/cmd/rib/pipe_operations_test.go` | AC-31, AC-34, D-3 | landed 615753f1c6 |
| `TestPeerRibRefusesScopeArgument`, `TestPeerRibHandsItsOperationsPastTheSelector` | `internal/component/plugin/server/peer_rib_scope_test.go` | AC-34, both halves; B2's fix line keeps the selector | landed 615753f1c6 |
| `TestSelectorInsidePathReachesTheDeclaredOperations`, `TestCommandWithoutSelectorKeepsItsDeclaredOperations` | `internal/component/command/pipe_partition_selector_test.go` | B1: a path that carries a selector reaches its declarations | landed 615753f1c6 |
| `TestRESTRIBRouteSendsThePipeForm` | `internal/component/api/rest/server_test.go` | AC-12 on the REST convenience routes | landed 3c642127ca |
| `TestPipeOperationsForCommand`, `TestPipeOperationsAgreeWithValidator` | `internal/component/command/pipe_operations_test.go` | AC-19, A-7 | landed 3c642127ca |
| `TestCompletionMatchesPipeOperations`, `TestCompletionStageKeepsQuotedBar` | `internal/component/command/completer_test.go` | AC-20; a quoted bar in the stage being typed | landed 3c642127ca |
| `TestCommandHelpPipeOperations` | `internal/plugins/meta/cmd/help_test.go` | AC-19 on `show command help` | landed 3c642127ca |
| `TestCatalogPipeOperations` | `internal/plugins/help/help_command_test.go` | AC-19 on `ze help command --json` | landed 3c642127ca |
| `TestEveryDataOperatorIsOverridable`, `TestDataOperatorOverrideAnswerShapeChecked`, `TestDisplayOverrideRecordedWithItsArgumentContract` | `internal/component/command/pipe_partition_test.go` | AC-17 under the D-5 ruling | lands with the D-5 widening |
| `TestPluginDataOperatorOverrideAccepted` | `internal/component/command/pipe_filter_plugin_test.go` | AC-17, AC-30 for a plugin: `display` and `resolve` accepted, `no-more` refused and nothing recorded | lands with the D-5 widening |

The four D-5 rows above landed in 6537dc0f86. Tests added from 6537dc0f86 to 72502f3950:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodePipeListReadsStrictly`, `TestWritePipeListRefusesBeforeWriting` | `internal/component/command/pipe_list_test.go` | the one pipe-list declaration (T-5 C) | landed 6537dc0f86 |
| `TestPipeListWrittenAfterParamsInOrder`, `TestPipeListElementCannotOpenAChain`, `TestPipeListRefusedWithNoCommandMatched`, `TestOpenAPIExecutePublishesThePipeList`, `TestOpenAPIPublishesNoPerCommandPath` | `internal/component/api/invocation_pipe_list_test.go` and the api package | AC-15, AC-39, AC-43 | landed 6537dc0f86 |
| `TestDerivedToolPipeList`, `TestDerivedToolPipeListElementCannotOpenAChain`, `TestDerivedToolPublishesThePipeList` | `internal/component/mcp/tools_pipe_list_test.go` | AC-15, AC-39 on MCP | landed 6537dc0f86 |
| `TestRESTExecutePipeList`, `TestRESTEveryPublishedPathIsServed` | `internal/component/api/rest/` | AC-15, AC-43 | landed 6537dc0f86 |
| `TestGRPCExecutePipeList` | `internal/component/api/grpc/execute_pipe_list_test.go` | AC-15 on gRPC `Execute` | landed 6537dc0f86 |
| `TestFilterMatchCrossField` (two cases), `TestMatchPoolCommunities` | `internal/component/bgp/plugins/rib/` | AC-44 | landed 4faad7bbe7 |
| `TestSendWildcardSelectorReachesTheUpdateHandler` | `internal/component/plugin/server/send_test.go` | the dispatcher hands `send bgp * update` its selector | landed 5ebf3c1c8e |
| `TestStreamLineMatchKeepsWholeEvents`, `TestStreamLineMatchNeverSplitsAnEvent` | `internal/component/command/stream_line_test.go` | AC-45 | landed 72502f3950 |
| `TestStreamMonitorTerminateCommitsSaves` | `internal/component/cli/client/main_test.go` | AC-46 | landed 72502f3950; its unit run was blocked by another session's build break in `internal/component/l2tp/pppoe/dataplane/vpp/factory.go`, and the functional `cli-monitor-pipe-typed-line` covers the signal path |

The `plugin/server` tests of `pipeChainRefusal`,
`internal/component/command/pipe_fold_test.go` (`FoldPipeChain`), and the
fixtures, `.ci` and catalog tests that read the old help and catalog keys
assert behaviour this design removes or renames. They are replaced, not
weakened: the replacement asserts the applied answer, or the same entries in
`pipe-operations`. `TestRIBScopeKeywordsMatchTheYANGModel` and
`TestBestJSONTerminalRowCarriesNoAge`
(`internal/component/bgp/plugins/rib/rib_pipeline_best_age_test.go`) are two
more: the first reads a YANG leaf this design removes, the second a terminal
it removes. Each such change is judged by an independent
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
| `rib-argument-form-refused` | `test/plugin/rib-argument-form-refused.ci` | AC-8 and AC-34, for `show bgp rib`, `show bgp rib best`, `show bgp rib protocol` and `show bgp peer <selector> rib` | |
| `rib-peer-pipe-scope` | `test/plugin/rib-peer-pipe-scope.ci` | AC-34: `show bgp peer <selector> rib \| received \| count` answers that peer's received count, and `show command help` lists the command's own operations | |
| `monitor-pipe-chain-ssh` | `test/plugin/monitor-pipe-chain-ssh.ci` | AC-22, AC-35: a monitor line with a chain over SSH exec is rendered by the daemon, and its accounting record holds the typed line | |
| `monitor-rendering-key-keeps-selection` | `test/plugin/monitor-rendering-key-keeps-selection.ci` | AC-36, AC-37, security negative: an SSH client that sets the rendering key on `monitor x \| match up` still receives only matching events; a wrong value of the key is refused; the key on a one-shot exec changes nothing | |
| `pipe-chain-backslash-authorized` | `test/plugin/pipe-chain-backslash-authorized.ci` | AC-38: `\| match a\b` on a line the policy allows runs and is accounted with its chain, against the TACACS+ fixture | |
| `cli-monitor-pipe-typed-line` | `test/ui/cli-monitor-pipe-typed-line.ci` | AC-23: `ze cli -c` with a monitor, a chain and `\| save` writes the file with only the events the chain keeps, and the daemon's accounting record holds the typed line | |
| `local-data-pipe-operation` | `test/ui/local-data-pipe-operation.ci` | AC-24: the same local-data line answers the same in `ze cli -c` and over SSH | |
| `pipe-chain-unauthorized-refused` | `test/plugin/pipe-chain-unauthorized-refused.ci` | AC-27, AC-28: a denied piped line and a read-only caller's piped write are refused over SSH and REST, and the denied command's handler leaves no trace | |
| `pipe-chain-accounting-masks-secret` | `test/plugin/pipe-chain-accounting-masks-secret.ci` | AC-29, against the TACACS+ accounting fixture | |
| `rib-declared-order-refused` | `test/plugin/rib-declared-order-refused.ci` | AC-25: `\| table \| peer` is refused with the reordered line, `\| resolve \| count` counts after resolving | |

Functional test status on 2026-11-20:

| Test | Where it lives | Status |
|------|----------------|--------|
| `dispatch-pipe-chain-applied` (replaces `dispatch-pipe-chain-refused`, deleted with its fixture in 4faad7bbe7), `rest-pipe-chain-applied`, `rest-pipe-list`, `mcp-pipe-chain-applied`, `mcp-derived-tool-pipe-list` | `test/plugin/` | promoted in 5ebf3c1c8e after an 80-iteration stress run each |
| `command-help-pipe-operations`, `pipe-chain-unauthorized-refused`, `pipe-chain-backslash-authorized`, `plugin-pipe-operation`, `rib-argument-form-refused`, `rib-declared-order-refused`, `rib-peer-pipe-scope`, `rib-pipe-received-count-accounted` (the accounting half of the `rib-pipe-filter` wiring row) | `test/plugin/` | promoted in adfaec2d77 after an 80-iteration stress run each |
| `bgp-dynamic-filter-swap-live`, `bgp-filter-aspath-reload`, `bgp-filter-prefix-definition-reload` | `test/reload/` | converted to the pipe form, promoted in 5ebf3c1c8e |
| `test-pipe-first-last`, `rib-pipe-filter` and the other converted RIB `.ci` (11 named in 4faad7bbe7) | `test/plugin/` | green with their converted fixtures in 4faad7bbe7 |
| `monitor-pipe-chain-ssh`, `monitor-rendering-key-keeps-selection`, `pipe-chain-accounting-masks-secret` | `test/draft/plugin/` | pass as drafts (72502f3950); NOT yet promoted |
| `cli-monitor-pipe-typed-line`, `pipe-quoted-value-is-an-argument` | `test/draft/ui/` | `cli-monitor-pipe-typed-line` passes as a draft (72502f3950); `pipe-quoted-value-is-an-argument` has no recorded pass since its fix; neither is promoted |
| `local-data-pipe-operation` | not written | decided: AC-24 is proven by unit tests (Key Design Decisions) |
| `rib-advertised-user-path` | `test/plugin/` (live, red) and `test/draft/plugin/` | KNOWN OPEN, not this spec: the user update is refused "no peers match selector" in the route-origin path (reactor origin intake, `bgp-rib` `originBarrier`, a27a951239, another session); `TestSendWildcardSelectorReachesTheUpdateHandler` proves the dispatcher hands the selector; the live copy was red before this work |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| the scenarios whose checks send argument-form RIB lines (`internal/le/interoplab/bgp/check_special.go`, `check_extras.go`, `check_rfc4456.go`, `check_route_server_replacement.go`, `check_rpki_reload.go`, and the sixth file the 2026-11-19 search found), named in phase 6 from the checks' callers, with `test/interop/scenarios/bgp-paths-limit-receive-bird` whose `ze.conf` carries a line | `test/interop/scenarios/` | FRR, BIRD | goal validation: the converted pipe-form lines answer the counts and rows the checks assert, over routes a real peer sent | |

No wire-visible BGP behaviour changes, so no new protocol interop scenario is
owed (`ai/rules/interop-and-goal-validation.md`: a change to the command
surface with no protocol peer).

## Files to Modify
- `internal/component/command/pipe.go` - quote-aware split; the partition replaces `foldFilters`; `FoldPipeChain` deleted; the `ProcessPipes*` wrappers return the typed command part
- `internal/component/command/pipe_filter.go` - `Leading` goes; `TakesArg` becomes the argument-kind field of the closed vocabulary, with every reader of it (`internal/plugins/meta/cmd/help.go`, `internal/plugins/help/help_command.go`, the RIB declaration); override recognition and argument check; the owner-scoped recording and withdrawal; the introspection function
- `internal/component/command/pipe_catalog.go` - each entry states whether a command may override it, the answer shape an override is held to, and whether the operation is presentation only; the one suffix function the daemon and the `ze cli` client both call
- `internal/component/command/completer.go` - `completePipe` reads the introspection; `pipeExtras` goes
- `internal/plugins/meta/cmd/help.go` - `commandHelp` answers `pipe-operations`; `pipeFilterHelp` and `pipeAliasHelp` go
- `internal/plugins/help/help_command.go` - `commandEntry` publishes `pipe-operations`; `operatorsFor` moves to the introspection function
- `internal/le/cli/catalog/catalog.go`, `internal/le/cli/catalog/render.go`, `internal/le/cli/list/report.go`, `internal/le/doc/yangcontract/command_surfaces.go`, `internal/le/site/catalog.go` - read `pipe-operations`; the copy of the join goes
- `internal/component/plugin/server/startup.go` - `validatePipeDecls` accepts a `PipeDecl` with an `argument` field and refuses one with both fields or neither; `registerPluginPipes` records operations through the owner-scoped entry point; the two rollback arms withdraw them
- `internal/component/plugin/server/command.go` - the shared quote rule; `Dispatch` accepts the chain; `CommandContext` carries the list; `routeToProcess` writes it; the residual chain for a data caller; the call of the command package's override shape check, which lives beside the partition and not here; `isAuthorized` and `BeginAccounting` read their tokens from the chain-grammar scan; `pipeChainRefusal` and `zeCLIForm` deleted
- `internal/component/plugin/dispatch.go` - `CallerIdentity` carries the rendering statement
- `cmd/ze/hub/main_servers.go` - `serverDispatcher` carries it into `CommandContext`
- `pkg/plugin/rpc/types.go` - the pipe list on `ExecuteCommandInput`, each element a name and its argument words; the `argument` field on `PipeDecl`
- `pkg/plugin/sdk/sdk_callbacks.go` - the handler receives the list; every `OnExecuteCommand` registration follows (53 non-test, 19 test files, recounted 2026-11-20)
- `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/web/handler_tools.go`, `internal/component/web/page_tools.go`, `internal/component/cli/model_mode.go`, `internal/component/cli/client/main.go`, `internal/test/cli/cmd_engine_steps.go` - send the typed line and state that they render
- `internal/component/mcp/tools.go` - under T-5 C the optional ordered `pipe` list on every derived tool
- `internal/component/api/invocation.go` - `buildInvocation` writes parameters at the pipe offset; under T-5 C the `pipe` list is written after them
- `internal/component/api/engine.go`, `internal/component/api/requests.go` - read-only on the command part for `Execute` and `Stream`; `BuildCommand` and its tests deleted
- `internal/component/command/invocation_scan.go` - the scan continues past the first pipe under the chain grammar; the unfinished-quote refusal covers the chain
- `internal/component/command/render_records.go`, `pipe_records.go`, `render_counted_document.go`, `local_data.go`, `argbind.go` - the three renderers and `ServeLocal` call the partition; `ValidatedArgs` carries the declared list; `ValidateArgs`, reached through `ValidateModelArgs`, refuses a pipe operation typed as an argument, read from the introspection function, before the declared-leaf and the pass-through branches
- `internal/component/plugin/server/handler.go` - `GetStreamingHandlerForCommand` takes the typed line, calls the partition and attaches the declared list to the `ValidatedArgs` it answers
- `cmd/ze/hub/server_stream.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/service_ssh.go`, `internal/component/cli/model_monitor.go` - the four lookup sites hand that function the typed line; the three that account and authorize do so on the typed line, under T-6 A with the chain; `api.go` gains `RunStreamPipes`; `service_ssh.go` honours the rendering key for the presentation suffix only
- `cmd/ze/hub/aaa_lifecycle.go` - under T-6 A the six typed authorizer wrappers and `liveAAABundleAccountant` are read and tested with chain tokens; they pass tokens through, so the expected change is tests and comments only
- `internal/component/ssh/session.go`, `internal/component/cli/sshclient/terminal.go`, `internal/component/cli/sshclient/client.go` - the rendering key, written by the client's stream call (`StreamCommand`) and read on the exec channel
- `internal/component/cli/model_keys.go`, `model_ping.go`, `model_traceroute.go`, `internal/component/cli/client/answer.go`, `cmd/ze/ze_core_pipe.go`, `internal/le/le/root/leroot.go` - the remaining readers of a chain, moved onto the partition or the one grammar. `leroot.go` appends the folded words to its own argv today, so `le` commands receive their declared operations as a list too
- `internal/component/plugin/server/restart.go` - the withdrawal of a plugin's operations beside its aliases
- `internal/component/aaa/command_args.go`, `internal/component/tacacs/authorizer.go`, `internal/component/authz/register.go`, `internal/component/aaa/login_profiles.go` - under T-6 A the typed authorizer contract states that the chain follows the arguments, and each implementer is read and tested with it: the TACACS+ one sends them as `cmd-arg` fields and its `fallbackArgs` hands the same tokens to the local authorizer, the two store authorizers flatten them through `CanonicalCommand`, the generation wrapper passes them on
- `internal/component/bgp/plugins/cmd/rib/rib.go` - the forwarders pass the list; `registerPipeFilters` stays the RIB's declaration, gains `sent`, `sent-received` and `aspath`, the explicit set of `show bgp rib protocol`, and the set of `show bgp peer rib`
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `container scope` and `leaf scope` of `container rib` go; the description names the pipe form
- `internal/component/bgp/plugins/rib/rib_pipeline.go`, `rib_pipeline_best.go`, `rib_inject.go`, `rib_commands.go`, `rib_pipeline_covering.go`, `rib_topology.go` - stages from the list in the three readers; the argument grammar removed with its refusal; the `json` terminal removed; the fix lines `refusePipelineArgs` is given become the pipe form
- `internal/component/bgp/plugins/rib/yang_enum_agreement_test.go`, `rib_pipeline_best_age_test.go` - the two tests of what this design removes, replaced under `ze-test-judge`
- `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go`, `internal/le/interoplab/bgp/check_ospf_redistribute_default_diagnostics_test.go`, `internal/component/plugin/server/usage_model_test.go` - the three users of the peer-scoped scope argument, converted to the pipe form. The last asserts today that `received` binds as the command's own argument (`ownArgDefs: 1`), which the removed leaf ends, so its row is rewritten under `ze-test-judge`
- `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` (both `leaf-list args`, `rpc show` and `rpc best`), `internal/component/bgp/plugins/rib/yang/ze-rib.yang`, `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` - the leaf-lists and the argument wording go
- `internal/component/lg/prefix_lookup.go`, `handler_api.go`, `handler_ui.go`, `handler_graph.go` - the looking glass sends the pipe form
- the test readers of the old keys: `internal/test/fixture/plugin_fixture_11_alias.go`, `internal/test/fixture/plugin_fixture_command_catalog.go`, `internal/test/fixture/ui_fixture_pipe_review_entry_contracts.go`, `test/ui/alias-peers.ci`, `test/ui/alias-summary.ci`, `test/ui/lg-peer-table-flat-payload.ci`
- every census writer of an argument-form RIB line (the LG handlers, the REST convenience routes, the interoplab checks, `internal/plugins/help/help_ai.go`, the fixtures, the `.ci` bodies)
- `// Design:` headers: every file above whose header names none of the pipe pages gains the line naming the page that documents its part. Today that is `internal/component/plugin/server/command.go`, `internal/component/plugin/server/startup.go`, `cmd/ze/hub/main_servers.go`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `internal/component/ssh/ssh.go`, `internal/component/web/cli_terminal.go`, `internal/component/cli/model_mode.go`, `internal/component/mcp/tools.go`, `internal/component/api/engine.go`, `internal/component/api/requests.go`, `internal/component/bgp/plugins/rib/rib_pipeline.go`, `internal/plugins/meta/cmd/help.go` and `internal/plugins/help/help_command.go` (to `docs/architecture/api/commands.md`), and `internal/component/command/completer.go` and `pipe_catalog.go` (to `docs/architecture/cli/command-verbs.md`); `internal/component/command/pipe.go`, `pipe_filter.go`, `alias.go`, `internal/component/plugin/dispatch.go` and `internal/component/bgp/plugins/cmd/rib/rib.go` already name `docs/architecture/api/commands.md`
- docs, beside the 17 pages of the census: `docs/architecture/api/json-format.md` (one sentence: a format operation makes `data` the rendered text), `docs/architecture/aaa-tacacs.md` (the typed line and its chain tokens), `docs/features/cli-commands.md`,
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
- `test/plugin/monitor-pipe-chain-ssh.ci`
- `test/ui/cli-monitor-pipe-typed-line.ci`
- `test/ui/local-data-pipe-operation.ci`
- `test/plugin/pipe-chain-unauthorized-refused.ci`
- `test/plugin/pipe-chain-accounting-masks-secret.ci`
- `test/plugin/rib-declared-order-refused.ci`
- `test/plugin/rib-peer-pipe-scope.ci`
- `test/plugin/monitor-rendering-key-keeps-selection.ci`
- `test/plugin/pipe-chain-backslash-authorized.ci`
- `cmd/ze/hub/pipe_parity_test.go` - the table-driven cross-surface parity test
- the unit test files the TDD plan names that do not exist yet, each beside the file it tests; phase 1 lists which of them are new

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the changed files' `// Design:` headers: `docs/architecture/api/architecture.md` (REST and gRPC execute apply the chain: affected), `docs/guide/mcp/overview.md` (`ze_execute` with a chain: affected), `docs/architecture/api/ipc_protocol.md` (the `PipeDecl` and `execute-command` fields, and an argument-form RIB example: affected), `docs/architecture/aaa-tacacs.md` (declared by the AAA files: the accounting record holds the typed line with its chain, and under T-6 A the typed authorizer receives the chain tokens: affected), `docs/features/cli-commands.md` (declared by the `ze cli` and `ze pipe` files: the chain grammar, the monitor's typed line and the pipe form of the RIB examples: affected), `docs/architecture/system-architecture.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/web-interface.md` and `docs/architecture/config/yang-config-design.md` (re-read in phase 8; expected unaffected), and `ai/patterns/registration.md` (declared by `cmd/ze/hub/aaa_lifecycle.go` for the AAA backend registry: unaffected, because this spec adds no backend and changes no registration, only the tokens the wrappers pass on). The full list is derived by `./le spec citation anchors spec plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` in phase 8 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every argument-form example and every old help or catalog key example in the census docs |

## Implementation Steps

Each phase is sized for one agent of about 60 tool calls.

Progress:
- b6f2383371, phase 1: the unit wiring tests, red by design (`TestServeLocalHandsDeclaredOperations`, `TestModelMonitorHandsDeclaredOperations`, `TestZeExecuteAppliesPipeChain`, `TestRESTExecuteAppliesPipeChain`, `TestGRPCExecuteAppliesPipeChain`, `TestBuildInvocationWritesParamsBeforePipe`). The `cmd/ze/hub` wiring tests and the plugin16 fixture were held back while another session's IKE edit broke the hub and fixture build.
- 83dfe9c2e6: the TACACS+ mock prints the accounted command, so a `.ci` can assert the accounting record.
- 3d7157b2be, phase 2: one grammar for the command part and the chain (AC-6, AC-7).
- 67c4ea9356: the web engine stand-in registers the BGP peer summarizer (Task item 4, AC-40).
- d30af7c2f1, phases 3a to 3c (95 files): the one partition replaces the fold; the dispatcher accepts a piped line, accounts it as typed, authorizes its chain tokens and applies the residual chain for a data caller; rendering surfaces send the typed line; REST parameters before the chain; `BuildCommand` deleted. It also carries the phase 1 hub tests and fixtures held back earlier. The six phase 1 wiring tests above except `TestModelMonitorHandsDeclaredOperations` now pass.
- Known reds named in d30af7c2f1's body, none caused by this work except the last, which is phase 4's: plugin/server `TestSelectorRefusalPlacesTheSelectorWhereDispatchReadsIt` and `TestDispatchImplicitSelectorMissingValue` (argument-fault text from b5a7d0dfe7, Task item 5, AC-41, being fixed as a separate small commit); cmd/ze/hub configured-plugin conformance tests (c9fbf8964c tests a command no code declares yet); `TestBuildTag_Container_PresentBinaryLinksNoUnusedRuntime` (Linux-only symbols on a macOS host); internal/component/plugin (another session's L2TP build break); config/cli `TestRepositoryConfigsValidate` (other sessions' BGP configs); cli `TestModelMonitorHandsDeclaredOperations` (phase 4).
- 34cc4767ac: `clear bgp rib out-excluding-owner` declares `owner` and `family`, so graceful-restart replay runs again (Task item 6, AC-42).
- ccb004dcfb: the missing-selector refusal names the selector again (Task item 5, AC-41).
- 07f5b7e39c, phase 4a: the stream route. `GetStreamingHandlerRendered` is the one lookup and partitions through `ReadStreamLine`; the four lookup sites hand it the typed line; `execMiddleware` sends the typed line in a `StreamRequest`; the rendering key is `ZE_CLI_RENDER=client`, honoured on the exec stream route for the presentation suffix only; streamed events keep the `pipe` metadata; `ProcessStreamPipesDefaultFunc` removed.
- a374677264, phase 4b: `ReadCommandPart` and `ReadLocalLine`; `le` hands declared operations as a list; the ping and traceroute live views carry the declared list through `PipeFlags.Declared` and the context; `ze <verb>` carries no declared operation; both halves of `TestEveryPipeSplitGoesThroughThePartition`.
- Known reds now: the `internal/component/bgp/plugins/cmd/rib` tests that pin the removed fold and the RIB argument form, phase 5's to convert.
- 3242d2dfb3, phases 5a and 5b: the pipe list on `execute-command` (`rpc.PipeOperation`), the SDK handler receives it, every `OnExecuteCommand` registrant converted; `PipeDecl.Argument`; `RegisterPluginPipeFilters` and its withdrawal at the three sites.
- 615753f1c6, phases 5c and 5d, with B1 and B2: the RIB's three readers build their stages from the declared list and refuse the argument form; the `json` terminal is gone; the RIB declares every word it accepted, `show bgp rib protocol` its own set and `show bgp peer rib` its own set; `leaf scope` and both `leaf-list args` gone. B1: a path that carries a selector value (`show bgp peer edge1 rib`) reaches its declarations (`invocationReachesCommandPath`). B2: the argument-form refusal's fix line keeps the selector.
- 3c642127ca, phase 6 and 7a: every in-tree RIB user in pipe form; one `pipe-operations` list from `PipeOperationsForCommand`, read by `show command help`, `ze help command --json`, the `internal/le` catalogs and completion; `operatorsFor` and its copy gone.
- D-5 widened by Thomas on 2026-11-20: every data operator is overridable, keeping its generic's answer shape; formats, paging, `save` and `log` are not. The site catalog change in `internal/le/site` (the `pipe-operations` keys and the operator guide's "A command may replace it with its own.") is approved by Thomas and lands with that widening.
- 6537dc0f86: the D-5 widening (every data operator overridable), the site catalog reader with Thomas's approved sentence, phase 7b (the ordered `pipe` list on every derived MCP tool, REST execute and gRPC `Execute` and `Stream`, one declaration `PipeListSchema`), and the OpenAPI unserved-routes defect (Task item 7, AC-43).
- 4faad7bbe7: the rest of phase 6 (the fixtures and `help_ai` the first census missed), phase 8 (the pages, 13 `// Design:` headers, the record notes, the D-4 rule point file), and the RIB `match` communities defect (Task item 8, AC-44).
- 5ebf3c1c8e: functional tests on every surface (`dispatch-pipe-chain-applied` replacing `dispatch-pipe-chain-refused`, the REST and MCP pipe-list and pipe-chain tests) and three reload tests in the pipe form.
- adfaec2d77: eight functional wiring tests (help, AAA, plugin operation, the RIB refusals and scope, the accounted RIB count).
- 72502f3950: the stream `| match` defect (Task item 9, AC-45) and the `ze cli -c` monitor save on signal (Task item 10, AC-46).
- Open before `/ze-close`: promote the five drafts named in the functional status table; `test/plugin/rib-advertised-user-path.ci` is red for a cause outside this spec (the route-origin path of a27a951239). Owed by phase 8's report: `./le ai rules update condensed` after the D-4 point-file edit.

1. **Phase: Wiring (MANDATORY FIRST)** -- failing tests at every entry point
   - Tests: the Wiring Test rows
   - Files: the new `.ci` drafts, the three surface unit tests
   - Verify: each fails because the chain is refused or folded, or help answers the old keys
   - Also: re-derive the census, the `docs/` rows first, printing the matching lines of the 17 pages and telling the peer-scoped form and other commands' words apart; A-3 and A-9's searches and the `docs/` half of A-12's; capture what a REST stream answers today for a monitor line with a chain; list which named test files are new; check the census tests for `RFC requirement:` tags
2. **Phase: one grammar** -- the scan in `internal/component/command/invocation_scan.go` continues past the first pipe under "The chain grammar"; the unfinished quote refused in the chain; `parsePipeOps` and the alias expansion read by it. What already landed (`ScanInvocation`, `ScanExecution`, the command-part refusal) is kept, not rebuilt
   - Tests: `TestSplitPipeLineQuoteAware`, `TestChainGrammar`, `TestSplitPipeLineUnfinishedQuoteInChain`, AC-6, AC-7
   - Files: `internal/component/command/invocation_scan.go`, `pipe.go`, `alias.go`
3. **Phase: partition, overrides and dispatch** -- the one partition replaces `foldFilters` at its eight call sites, with the prefix rule and the validation it keeps; the catalog states overridability, the override shape and the presentation fact; `PipeFilter` takes the argument-kind field in place of `TakesArg` and the partition validates a declared operation's argument by it; the argument check when recorded and the shape check, a function of the command package beside the partition's answer step; authorization and accounting tokens from the chain-grammar scan; `ValidateArgs` refuses a pipe operation typed as an argument (`TestValidateArgsRefusesPipeOperationWord`); `Dispatch` accepts the chain and hands `pipeChainMeta` to `ApplyPipes` for a data caller; the in-process carriers; the surfaces send the typed line; the typed line to the accountant and, under T-6 A, the chain tokens to the authorizer; parameters written before the pipe; the refusal, `FoldPipeChain` and `BuildCommand` deleted
   - Tests: AC-1, AC-3, AC-4, AC-5, AC-10, AC-11, AC-13, AC-16, AC-17, AC-18, AC-25, AC-27, AC-28, AC-29, AC-32, AC-38; `TestDeclaredArgumentContractValidatedByPartition`, `TestPipeFilterArgumentKindRecorded`, `TestCatalogStatesPresentationOperators`, `TestTypedWrappersPassChainTokens`, `TestTacacsFallbackArgsPassesChainTokens`
   - Files: `internal/component/command/pipe.go`, `pipe_filter.go`, `pipe_catalog.go`, `render_records.go`, `pipe_records.go`, `render_counted_document.go`, `internal/component/plugin/server/command.go`, `internal/component/plugin/dispatch.go`, `cmd/ze/hub/main_servers.go`, the surface files, `internal/component/api/engine.go`, `invocation.go`, `requests.go`, and under T-6 A the AAA files, `cmd/ze/hub/aaa_lifecycle.go` and `internal/component/tacacs/authorizer.go`
4. **Phase: the routes outside `Dispatch`** -- `GetStreamingHandlerForCommand` partitions and attaches the declared list, and its four callers hand it the typed line; `execMiddleware` passes the typed line to the `StreamingExecutor`; the REST and gRPC stream site applies the chain; the local-data route calls the partition; `ValidatedArgs` carries the list; the session environment key, honoured on the exec stream route for the presentation suffix only, and the one suffix function the `ze cli` monitor and the daemon both call; the remaining chain readers (`HasFormatPipe`, the ping and traceroute monitors, `ze pipe`, `leroot.go`)
   - Tests: AC-22, AC-23, AC-24, AC-33, AC-35, AC-36, AC-37, `TestEveryPipeSplitGoesThroughThePartition`, `TestPresentationSuffixSameOnClientAndDaemon`, `TestOverrideShapeCheckedOnEveryRoute`
   - Files: `internal/component/plugin/server/handler.go`, `cmd/ze/hub/server_stream.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/service_ssh.go`, `internal/component/cli/model_monitor.go`, `internal/component/ssh/ssh.go`, `session.go`, `internal/component/cli/sshclient/`, `internal/component/cli/client/main.go`, `answer.go`, `internal/component/cli/model_keys.go`, `model_ping.go`, `model_traceroute.go`, `internal/component/command/local_data.go`, `argbind.go`, `cmd/ze/ze_core_pipe.go`
5. **Phase: the plugin carrier and declaration** -- the pipe list on `execute-command`, the SDK handler, every `OnExecuteCommand` rebuilt; `PipeDecl` with its `argument` field; the owner-scoped, error-returning entry point into `pipeFilterRegistry` and its withdrawal at the three sites; the RIB declares every word it accepted and the explicit set of `show bgp rib protocol`, its three readers build their stages from the list and refuse the argument form; `show bgp peer rib` declares its own set and `leaf scope` goes; both `leaf-list args` go; the `json` terminal goes
   - Stale comments and texts this phase rewrites in the RIB (`ai/rules/stale-comments.md`): the comment above `scopeKeywords` ("positional scope keywords (must appear first)") and its `enumeration: gated by` line, which names the test this phase replaces; the comment in `parsePipelineArgs` on `first` and `last` ("bypassed when these are folded into the command"); the comment in `jsonTerminal.drain` on an ordering that depends on whether `| json` was typed; the fix lines given to `refusePipelineArgs`; the description of `leaf scope` ("The word is typed first, before any pipe stage"); the two `rpc` descriptions in `ze-rib-api.yang`
   - Tests: AC-2, AC-8, AC-9, AC-14, AC-26, AC-30, AC-31, AC-34; `TestRIBJSONTerminalEqualsBareCommand` first; `TestRIBRefusalsNameThePipeForm`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `internal/component/plugin/server/startup.go`, `restart.go`, the 72 handler files (53 non-test, 19 test), the RIB files and YANG. The main thread cuts the handler rebuild into its own package (state file decomposition, 5b)
6. **Phase: convert every RIB user, one package per tree** -- each is one agent, lands on its own, and re-takes the proof record of any RFC-tagged test it changes
   - 6a product: the looking glass (`internal/component/lg`, code and its 9 test files), the REST convenience routes, `internal/plugins/help/help_ai.go`, the comments and YANG descriptions
   - 6b interoplab: the 6 files under `internal/le/interoplab`, `check_ospf_redistribute_default_diagnostics_test.go` (the peer-scoped form) and `test/interop/scenarios/bgp-paths-limit-receive-bird/ze.conf`; the interop runs are owed commands
   - 6c fixtures: the 21 files under `internal/test/fixture`, and `plugin_fixture_13_peer_lifecycle.go` (the peer-scoped form)
   - 6d `.ci`: the 12 under `test/plugin` and the 4 under `test/reload`
   - 6e unit tests: the 8 under `internal/component/bgp/plugins/rib`, `rfc8907_command_test.go`, `plugin_rpc_test.go` and `usage_model_test.go` (the peer-scoped form)
   - 6f docs: the 17 pages the census names, their matching lines read in phase 1
   - 6g site test data: `internal/le/site/testdata/`, regenerated from its source
   - Tests: AC-12; the interop checks
7. **Phase: introspection, and T-5 as approved** -- `operatorsFor` moves into `internal/component/command` and gains the declared operations and aliases, keeping `class`; `commandHelp`, `commandEntry` and `completePipe` read it; every reader of the old keys converted; the copy in `internal/le/cli/catalog/catalog.go` deleted; under T-5 C the ordered `pipe` list on derived tools and routes, validated against it
   - Tests: `TestPipeOperationsForCommand`, `TestPipeOperationsAgreeWithValidator`, `TestCompletionMatchesPipeOperations`, `TestCommandHelpPipeOperations`, `TestCatalogPipeOperations`, `command-help-pipe-operations.ci`, AC-15, AC-19, AC-20, AC-21
   - Files: `internal/component/command/pipe_filter.go`, `completer.go`, `internal/plugins/meta/cmd/help.go`, `internal/plugins/help/help_command.go`, the census readers, `internal/component/mcp/tools.go`, `internal/component/api/invocation.go`
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
| Correctness | Every route calls the one partition: `Dispatch`, the four streaming-handler lookups through `GetStreamingHandlerForCommand`, `ServeLocal`, the three renderers, `HasFormatPipe` (`TestEveryPipeSplitGoesThroughThePartition`); the `ze cli` monitor client calls the suffix function and no partition |
| Correctness | The override shape check and the declared-argument validation run on every route, not in `Dispatch` alone (`TestOverrideShapeCheckedOnEveryRoute`) |
| Correctness | The rendering key changes who formats, pages and saves, and nothing else: no selecting or reducing operation is skipped with it set (`TestRenderingKeyNeverSkipsSelection`) |
| Correctness | Authorization and accounting read the chain-grammar scan; no second invocation scan of the whole line remains on either path |
| Correctness | A declared operation is taken only as a prefix of the chain; nothing reorders what the operator typed |
| Correctness | No word the RIB accepted as an argument is lost: each is a declared operation or proven equal to the bare command (A-11) |
| Data flow | One parse function (A-4); help, the catalog, completion and the validator agree (A-7) |
| Operator errors and doctor codes | The unfinished-quote, argument-form, declared-after-residual, override-shape and collision refusals are `opererr` errors with an exact fix line |
| Rule: no-layering | `foldFilters`, `FoldPipeChain`, `pipeChainRefusal`, `zeCLIForm`, `Leading`, `leaf-list args`, the RIB argument reading, the keys `pipe-filters`, `pipe-aliases`, `operators` and `pipes`, `pipeFilterHelp`, `pipeAliasHelp`, `pipeExtras`, `operatorsFor` and its copy are deleted, not left beside the new path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No rewrite remains | a grep of `internal/` for `foldFilters`, `FoldPipeChain` and `pipeChainRefusal` answers nothing |
| The wire fields exist | `pkg/plugin/rpc/types.go` shows the pipe list on `ExecuteCommandInput` and the `argument` field on `PipeDecl` |
| Every route partitions | `TestEveryPipeSplitGoesThroughThePartition` passes; the stream, local-data and monitor `.ci` pass |
| The same answer on every surface | `TestPipedLineParityAcrossSurfaces` passes |
| Every surface applies the chain | the AC-3 tests and the new `.ci` pass |
| No argument-form RIB line remains in-tree | the census search answers nothing outside the refusal test |
| Help, the catalog and completion agree | `command-help-pipe-operations.ci` passes; a grep for the old keys answers nothing |
| The design is in the pages | the three pages carry the design text and no rewrite text; every changed file's `// Design:` header names its page |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a REST or gRPC parameter cannot open a chain: `WriteInvocation` quotes it (AC-11) and it is written before the pipe (AC-32); under T-5 C each element of the `pipe` list is validated against the command's `pipe-operations` and its words are written quoted, never spliced as text; a plugin's `Pipes` entry names only its own commands (`validatePipeDecls`) and a colliding one is refused whole (AC-30); a `pipe` list element cannot open a chain (AC-39, `TestPipeListElementCannotOpenAChain`); the session rendering key is untrusted input from any SSH client: it is validated like the two existing keys, a value other than the defined one is refused (AC-37), and it is honoured on the exec stream route only |
| Authorization | read-only judged on the command path; an operation never turns a write into a read. Negative tests on every surface: a denied piped line (AC-27) and a read-only caller's piped write (AC-28) are refused and run no handler. On the stream route the line is scanned first, because the authorizer's tokens come from that scan, and no handler runs and no event is produced before the authorizer answers. The scan is the chain-grammar one, so a chain backslash neither denies an allowed line nor hides a token from the policy (AC-38). The rendering key hands over presentation only: the daemon applies every selecting or reducing operation whatever the key says, so a client that sets it receives no row a `\| match` would have removed (AC-36, `TestRenderingKeyNeverSkipsSelection`). Under T-6 A the six wrappers and `fallbackArgs` hand the local policy the same tokens the server was sent |
| Accounting | the record holds the typed line with its chain, secret-masked by `config.DisplayCommand` as today (AC-29), on the one-shot route and the stream route; a refused line is accounted too (RFC 8907 Section 8.3) |
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
- A plugin's operations need no new declaration: `Pipes` already names a pipe word per command, and an entry that states its argument in place of an expansion is the operation itself.
- `Dispatch` is not the only door: the stream route, the local-data route and the `ze cli` monitor answer a piped line without it, so the mechanism belongs in the command package and the dispatcher is one of its callers.
- Part of defect 3 landed between the spec and its review (`invocation_scan.go`). A design spec ages against the tree, and its Current Behavior is re-read before it is presented.
- The fold's gate was the format operator, not order: a declared operation jumped ahead of any other residual operation, which nobody typed.
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
| One partition function in `internal/component/command`, called by `Dispatch` and by the three routes that never reach it | the partition inside `Dispatch` only; each surface parses and folds | the stream route, the local-data route and the `ze cli` monitor would keep rewriting; a function in the command package is reachable from the caller's own process, which `Dispatch` is not |
| The chain takes the invocation's quote grammar, keeping a literal backslash outside quotes | keep the separate, quote-blind chain grammar; apply the invocation grammar whole, refusing a backslash | one reading of a line for every caller, and argument boundaries for the structured list; refusing a backslash would break `match` patterns that work today |
| A declared operation is taken only as a prefix of the chain | take it wherever it stands (today); take it unless a format came first | the chain runs in the order typed; a hoisted operation computes over rows the earlier operations were meant to change |
| Overridability and the override's shape are catalog facts | the four-case switch in `foldFilters` | registration over enumeration: every data operator is overridable, its shape in its catalog entry (Thomas, 2026-11-20) |
| `PipeDecl` gains an explicit `argument` field; exactly one of it and `expansion` | no expansion means operation | the absence of a field is not a statement, and the guard "expands to nothing" would go |
| A plugin's operations enter `pipeFilterRegistry` through an error-returning, owner-scoped entry point | `RegisterPipeFilters` as it is | a wire declaration must not panic the daemon, and must leave with its plugin |
| The rendering statement of a client in another process is an SSH session environment key | a reserved word in the line; a new protocol field | the session environment already carries the client's mode and is validated by the server; the line stays as typed |
| A streaming handler and a local-data handler receive the list in `ValidatedArgs` | add `CommandContext` to `StreamingHandler` | no signature change and no edit to any registration |
| A REST parameter value holding a pipe or a quote stays one argument | refuse it (this spec's earlier AC-11) | `WriteInvocation` already quotes it; a refusal would drop a value that works |
| The zero value of the rendering statement means the engine applies the residual chain | the zero value means the caller applies it | a caller added later that forgets to say gets the chain applied, never dropped |
| An override's answer shape is checked at answer time by one function of the command package, run by `Dispatch`, `ServeLocal` and the stream sites; its argument contract is checked when recorded | the check inside `Dispatch` alone (the first amendment); a declared answer shape (`RegisterShape`); a test every override must pass, alone | `RegisterShape` states the command's shape, not one operation's; a test cannot be required of a third-party plugin |
| The RIB's argument words are refused, not accepted beside the pipe | keep both forms | no-layering, and no compatibility path for a form no release shipped |
| One spec for the three defects | three specs | they share the parse |
| A declared operation with no argument carries nil words (`argumentWords`, `pipe_partition.go`), on the argument branch and the parsed-words branch alike (implementation, 2026-11-20) | an empty non-nil list on one branch | one answer for one fact; nil round-trips JSON the same whether the phase 5 wire field is written null or omitted |
| A data caller's `\| json` answer carries the `pipe` metadata, the same document `ze cli -c` prints (implementation, 2026-11-20) | strip the metadata for a data caller | AC-3 asks for the bytes the local command prints, and `ApplyPipes` injects the metadata at the format operation on every rendering surface |
| An already-rendered text answer travels in an engine-only form (`renderedAnswer`, `plugin/server/command.go`) that marshals as a JSON string; `\| json` and `\| raw` answer the document (implementation, 2026-11-20) | wrap the text as `plugin.RawJSON` | `RawJSON` refuses a top-level scalar, so every marshal of the answer failed; only the engine builds the form, and it reaches only callers that do not render |
| `RefusePipeOperationWords` refuses only the command's own declared operations (overrides and own names), never a generic name or an alias (implementation, 2026-11-20) | refuse every name in the command's `pipe-operations` | a generic name is a valid argument value: a peer named `count` stays an argument on every command |
| AC-24 is proven by the unit tests `TestServeLocalHandsDeclaredOperations` and `TestLocalDataDeclaredOperationsSameOnBothRoutes`; `test/ui/local-data-pipe-operation.ci` is not written (implementation, 2026-11-20) | a functional `.ci` over a local-data command | no in-tree local-data command declares an operation, so a `.ci` has no real command to drive; the unit tests register one and run both routes |
| The rendering key is `ZE_CLI_RENDER` with the one value `client`; streamed events keep the `pipe` metadata, as the one-shot route does (implementation, 2026-11-20) | strip the metadata on the stream route | the stream route reads the same partition meta the one-shot route does, and every rendering surface already printed it |
| A renderer in another process sends `DeclaredLine` (the command part and its declared operations), not the typed line (implementation, 2026-11-20) | send the typed line | the cross-process renderer applies the residual chain itself, so sending it too would render it twice |

## Known Limitations
- An interactive `ze cli` session is a shell on the daemon (`runInteractiveSession` calls `sshclient.RunInteractive`; `ai/doors/commands-cli.md`, "Where it sits"), so its lines are parsed by the daemon's model with the daemon's registries, a plugin's alias and declared operation included. The earlier sentence here, that interactive `ze cli` parses the chain in the client, was wrong against today's code.
- What still reads a chain in the client's own process: the `ze cli -c` monitor (`StreamMonitor`), and a local-data command answered by `ServeLocal`. The monitor client reads the catalog alone, to find the presentation suffix, and refuses no name it does not know, so a monitor line naming an external plugin's alias or operation is sent as typed and applied by the daemon (R-10). `ServeLocal` serves compiled-in commands only, and a plugin declares pipe names on its own commands only (`validatePipeDecls`), so no external plugin's name can apply to a line `ServeLocal` answers. This spec therefore leaves no refusal for `plan/release/fixes/spec-plugin-alias-reaches-the-client.md` to remove; that spec is a skeleton dated 2026-09-05 whose premise about the interactive session is stale, and it is re-judged before it is designed.
- Two quote grammars remain after this spec. An operational line, command part and chain, groups with double quotes only, and a single quote is a literal character. The config editor's command input uses the config token codec, where single quotes also group and keep their contents raw (`docs/guide/config-editor.md`). The editor's pipe filters act on its display of the configuration and dispatch no operator command, so they are out of scope here; bringing the two grammars together is a design of its own and no spec holds it today.
- This spec converts no command but the RIB's. The example `show sysctl | key <k>` in the rewritten rule sentence shows the rule; the sysctl commands, and the peer commands' positional selector, keep their paths until a conversion spec moves them.

## Goal Validation (planned evidence)

One row per goal of the Task section. The closure half of the template is
appended by `/ze-close`; this table states at design time what will prove each
goal, so no goal reaches closure without a named proof.

| Goal (Task section) | Concrete evidence |
|---------------------|-------------------|
| A command path names the subject and stops; scope, presentation and format are pipe operations | `test/plugin/rib-argument-form-refused.ci` red before phase 5 and green after; the census search of the argument form answers nothing outside that test |
| A common operation is generic and applies to every command's answer, from every caller | `TestGenericRowOperatorsOnUndeclaredCommand`; `TestPipedLineParityAcrossSurfaces` over SSH, the web terminal, `ze cli -c`, MCP, REST, gRPC and `dispatch-command` |
| A command declares its own operations; the engine hands them to the handler for every caller; the line is never rewritten | `test/plugin/dispatch-pipe-chain-applied.ci` internal and forked; `test/plugin/monitor-pipe-chain-ssh.ci` and `test/ui/local-data-pipe-operation.ci` for the routes outside `Dispatch`; `test/plugin/rib-pipe-filter.ci` asserting the accounting record holds the typed line; a search for `foldFilters` and `FoldPipeChain` that answers nothing |
| An override keeps the generic's meaning and answer shape; another meaning takes its own name | `TestOverrideAnswerShapeRefused`, `TestOverridableComesFromTheCatalog`, `TestRIBOverridesKeepGenericMeaning` |
| A third-party plugin declares its own operation | `test/plugin/plugin-pipe-operation.ci` with the plugin run forked; `TestPluginOperationCollisionsRefused`; `TestPluginOperationWithdrawnOnStop` |
| Every operator command answers the same on every surface as the local `ze` command | `TestPipedLineParityAcrossSurfaces`; `test/plugin/mcp-pipe-chain-applied.ci`, `test/plugin/rest-pipe-chain-applied.ci` with explicit JSON assertions |
| For any command, help lists every operation it answers to | `test/plugin/command-help-pipe-operations.ci`: `show command help`, `ze help command --json` and completion hold the same list; `TestPipeOperationsAgreeWithValidator` |
| Defect 3: a quoted value is one argument everywhere on the line | `test/ui/pipe-quoted-value-is-an-argument.ci`, red today on `match "error\|warn"` |
| Security: a chain never widens what a caller may run | the negative tests `test/plugin/pipe-chain-unauthorized-refused.ci` and `TestPipedWriteRefusedForReadOnlyCallerOnEverySurface`, each failing with the surface's permission-denied answer; `test/plugin/pipe-chain-accounting-masks-secret.ci`; `test/plugin/monitor-rendering-key-keeps-selection.ci`, where a client that sets the key still receives only matching events; `TestPipeListElementCannotOpenAChain` |
| The converted RIB users still answer | the interop checks of phase 6b against FRR and BIRD, and the converted fixtures and `.ci` with their expected answers unchanged |

Evidence recorded on 2026-11-20, per goal:

| Goal | Evidence in the tree |
|------|----------------------|
| The path names the subject; scope, presentation and format are pipe operations | `test/plugin/rib-argument-form-refused.ci`, `rib-peer-pipe-scope.ci` (adfaec2d77); `TestValidateArgsRefusesPipeOperationWord`, `TestRouteReadRefusesArgumentForm`, `TestPeerRibRefusesScopeArgument` |
| A generic operation applies to every command's answer from every caller | `TestPipedLineParityAcrossSurfaces` (SSH side, MCP, REST, gRPC, `dispatch-command`, formats included); `dispatch-pipe-chain-applied.ci`, `rest-pipe-chain-applied.ci`, `mcp-pipe-chain-applied.ci` (5ebf3c1c8e) |
| Declared operations reach the handler from every caller; the line is never rewritten | `TestPartitionDeclaredOperations`, `TestDispatchHandsDeclaredOperationsToHandler`, `TestForkedRIBReceivesTheDeclaredList`, `TestStreamingLookupPartitions`, `TestLocalDataDeclaredOperationsSameOnBothRoutes`; `rib-pipe-received-count-accounted.ci` (the typed line accounted); `TestEveryPipeSplitGoesThroughThePartition` (both halves); `foldFilters` and `FoldPipeChain` are gone |
| An override keeps its generic's meaning and shape; another meaning takes its own name | `TestOverrideAnswerShapeRefused`, `TestEveryDataOperatorIsOverridable`, `TestDataOperatorOverrideAnswerShapeChecked`, `TestOverridableComesFromTheCatalog` |
| A third-party plugin declares its own operation | `test/plugin/plugin-pipe-operation.ci` (adfaec2d77); `TestPluginOperationCollisionsRefused`, `TestPluginOperationWithdrawnOnStop`, `TestExecuteCommandHandlerReceivesPipeFromTheWire` |
| Every surface answers the same as the local `ze` command | `TestPipedLineParityAcrossSurfaces`; the per-surface tests `TestZeExecuteAppliesPipeChain`, `TestRESTExecuteAppliesPipeChain`, `TestGRPCExecuteAppliesPipeChain`, `TestExecOneShotSendsTheTypedLine`, `TestWebSurfacesSendTheTypedLine`; the structured list `rest-pipe-list.ci`, `mcp-derived-tool-pipe-list.ci`, `TestGRPCExecutePipeList` |
| Help lists every operation a command answers to | `test/plugin/command-help-pipe-operations.ci` (adfaec2d77); `TestPipeOperationsAgreeWithValidator`, `TestCommandHelpPipeOperations`, `TestCatalogPipeOperations`, `TestCompletionMatchesPipeOperations` |
| Defect 3: a quoted value is one argument everywhere | `TestChainGrammar`, `TestDispatchQuotedPipeArgumentWithChain`, `TestCompletionStageKeepsQuotedBar`; `test/draft/ui/pipe-quoted-value-is-an-argument.ci` still a draft |
| Security: a chain never widens what a caller may run | `pipe-chain-unauthorized-refused.ci`, `pipe-chain-backslash-authorized.ci` (TACACS+, adfaec2d77); `TestTypedWrappersPassChainTokens`, `TestTacacsFallbackArgsPassesChainTokens`, `TestExecutePipedReadOnlyCaller`, `TestPipeRefusalsMaskSecrets`, `TestPipeListElementCannotOpenAChain`, `TestRenderingKeyNeverSkipsSelection`; `pipe-chain-accounting-masks-secret.ci` and `monitor-rendering-key-keeps-selection.ci` pass as drafts, not promoted |
| The converted RIB users still answer | the converted fixtures and `.ci` green in 4faad7bbe7 and 5ebf3c1c8e with their expected answers unchanged; the interop checks of phase 6b against FRR and BIRD are not recorded here |

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
