# Spec: a plugin's dispatched pipe chain reaches the handler as arguments

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped daemon behaviour: a command a plugin dispatches with a pipe chain gets a silently wrong answer (the whole answer, unfiltered, or a refusal naming the pipe as an argument), which `ai/rules/principles.md` forbids. That is incorrect shipped behaviour (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`Dispatcher.Dispatch` (`internal/component/plugin/server/command.go`) gives the
pipe token no meaning. `tokenize` splits on spaces and tabs only, so a line such
as `show vpn ipsec sa` piped to `match x` resolves to `show vpn ipsec sa` with the
pipe token, `match` and `x` left over as arguments. A handler that reads its
arguments refuses them; a handler that ignores them, or reads them loosely,
answers the whole result unfiltered with a success status.

Every operator surface splits the chain off before it dispatches and renders the
answer through it: the SSH exec channel (`internal/component/ssh/ssh.go`), the
web CLI (`internal/component/web/cli_terminal.go`), `ze cli`
(`internal/component/cli/model_mode.go`) and the functional-test engine steps
(`pipedEngineDispatch`, `internal/test/cli/cmd_engine_steps.go`) all call
`command.ProcessPipesDefaultFormatChecked` (or its `Local` form). A plugin's
`dispatch-command` (`Server.dispatchCommandResponse`,
`internal/component/plugin/server/dispatch.go`) passes the line to `Dispatch`
whole, so the chain reaches the handler.

The goal: a pipe chain a plugin dispatches either works as it does on the
operator surfaces or is refused with an error that names the chain and the fix.
It is never answered as if it were not there.

The repair needs care: `tokenize` unquotes, so a quoted pipe (a value that is a
literal pipe character) becomes the same token as an unquoted one. A refusal
keyed on the token alone would refuse a legitimate quoted argument.

**Decision (design, 2026-11-12):** the dispatcher REFUSES a line that carries a
pipe outside quotes, in `Dispatch` itself, before any handler, plugin process or
subsystem sees it. The refusal names the command, the chain, why the chain is
not applied there, and the exact line to send instead (the folded filter form
when the whole chain is command-owned filters). Key Design Decisions carries the
alternatives and the producer functions that decide it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - a plugin can run in another process; the text command dispatcher is the slow path an external caller uses
  → Constraint: the fix behaves the same for an internal plugin (DirectBridge) and a forked one (RPC)
  → Decision: the check lives in `Dispatch`, which both transports reach through `dispatchCommandResponse`, so no transport gets its own copy
- [ ] `ai/doors/plugin-infra.md` - the dispatcher and `dispatch-command` belong to `plugin/server`
  → Constraint: one engine-op entry per RPC; the fix adds no second dispatch path
  → Decision: `opDispatchCommand` and its typed bridge slot are unchanged; only what `Dispatch` accepts changes
- [ ] `ai/doors/commands-cli.md` - pipe operators, pipe aliases, `ApplyPipes`
  → Constraint: a handler answers data, never text; output shaping goes through the pipe layer at the surface that renders
  → Decision: the dispatcher does not render. Applying the chain inside `Dispatch` would put rendered text where every caller reads data
- [ ] `ai/doors/api-surfaces.md` - REST, gRPC, MCP are thin adapters over `serverDispatcher`
  → Constraint: never add a command or a check in an adapter
  → Decision: the adapters stay unchanged; the check in `Dispatch` reaches them
- [ ] `ai/doors/le-tooling.md`, `ai/doors/hub-engine.md`, `ai/doors/system-plugins.md`, `ai/doors/bgp-engine.md` - the fixture and `.ci`, `serverDispatcher`, the ExaBGP bridge, the route push
  → Constraint: a compiled fixture returns an error when an assertion fails; the bridge's translated `send bgp ... update text` lines are a contract, and the route text grammar has no pipe
  → Decision: no change to the hub, the bridge or the route grammar
- [ ] `docs/architecture/api/commands.md` - "Pipe aliases: a name for an operator chain", "The pipe layer selects and re-sequences. It cannot compute", "Where an alias resolves, and where it does not"
  → Constraint: an alias expands in the process that parses the chain; the pipe layer only reshapes a returned payload
  → Decision: the refusal's suggested line is computed with the same parse, alias expansion and fold the operator surfaces use (`parsePipeOps`, `expandAliases`, `bindAddressFields`, `foldFilters`), never a second reading of the operator names
- [ ] `docs/architecture/api/process-protocol.md` - "Inter-Plugin Communication": `dispatch-command` answers a head, records and a terminator; the single-value form `DispatchCommandOutput.data` "carries raw JSON"
  → Constraint: the answer of `dispatch-command` is data. Its contract is unchanged for every line without a pipe outside quotes
  → Decision: a piped line gets the existing error answer form, with the refusal text as its message

**Key insights:**
- The dispatcher's comment on flag-shaped leftovers says a pipe filter is folded into trailing args client-side by `foldFilters`, so a pipe token reaching `Dispatch` was never meant to be an argument.
- In-tree plugins already send the folded form: `plugin16PipeFirstLast` (`internal/test/fixture/plugin_fixture_16.go`) dispatches `show bgp rib received first 3 count`, never a pipe. The folded form is the dispatch grammar; the pipe is the operator's spelling of it.
- `ParsePipe` (`internal/component/command/pipe.go`) cuts at the first pipe character with `strings.Cut` and is not quote-aware; `tokenize` is. So only the dispatcher's own tokenizer can tell a quoted pipe from a chain.
- MCP `ze_execute` (`internal/component/mcp/tools.go`), REST execute (`internal/component/api/rest/server.go` `handleExecute`) and gRPC reach `Dispatch` through `serverDispatcher` (`cmd/ze/hub/main_servers.go`) with the line unsplit too. Fixing `Dispatch` covers them in the same place.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `Dispatch` defers `BeginAccounting`, calls `tokenize`, matches the longest builtin prefix (`matchBuiltinTokens`), validates args (`command.ValidateArgs`), applies the selector guard, authorizes, refuses flag-shaped leftovers (`firstFlagToken`, `flagRefusal`), then calls the handler; with no builtin match it resolves a subsystem (`dispatchSubsystem`) or a plugin command (`dispatchPlugin`, which re-tokenizes the raw remainder and sends it to the plugin process with `execute-command`). `tokenize` rejects a backslash, toggles quoting on each double quote, drops the quote characters and splits on space and tab outside quotes
  → Constraint: `dispatchPlugin` and `dispatchSubsystem` re-tokenize substrings of the RAW input, so the quoting fact survives to them; a check in `Dispatch` before resolution covers all three branches
- [ ] `internal/component/plugin/server/dispatch.go` - `dispatchCommandResponse` builds a `CommandContext` (internal plugin identity, `ProcessSender`) and calls `Dispatch` with the line unchanged; on error it logs `dispatch-command failed` with `diagnostic.CodeDoctorPluginRequestRefused` and raises a server event. `dispatchCommand` (socket and DirectBridge JSON path) and `dispatchCommandAnswer` (DirectBridge answer path, `plugin.AnswerFor`) both call it
  → Decision: no change here; the refusal arrives as a dispatch error and takes the existing log, doctor code and error answer
- [ ] `internal/component/ssh/ssh.go` - the exec channel calls `command.ProcessRemoteStreamPipes` for streams and `command.ProcessPipesDefaultFormatChecked` before dispatch; its comment says the tokenizer gives the pipe no meaning
- [ ] `internal/test/cli/cmd_engine_steps.go` - `pipedEngineDispatch` splits whenever the line contains a pipe character, and its comment records the symptom
- [ ] `internal/component/command/pipe.go` - `ParsePipe` cuts on the first pipe character, `parsePipeOps` splits the rest on every pipe, `parsePipeChain` adds `expandAliases` and `bindAddressFields`, `foldFilters` rewrites command-owned filters (registered with `RegisterPipeFilters`, `pipe_filter.go`) into trailing server args and keeps every other operator in the chain; `processPipesDefaultFormat` then appends a default format operator and returns a formatter that calls `ApplyPipes` and yields rendered text
  → Constraint: the formatter's output is text in the format the chain or `ze.cli.format` names, never the raw data a dispatch caller reads
- [ ] `cmd/ze/hub/main_servers.go` - `serverDispatcher` hands web, SSH, MCP, REST, gRPC, LG and chaos one closure that calls `Dispatch` with the surface's line
- [ ] `internal/component/mcp/tools.go` - `ze_execute` and `run` pass the line to `CommandDispatcher.JSON` unsplit
- [ ] `internal/component/plugin/server/dispatch_registry.go`, `system.go` - `opUpdateRoute` and the typed route-select op build `send bgp <peer> <command>` and call `Dispatch`; `handleSystemDispatch` re-joins its already unquoted args with spaces and calls `Dispatch`

**Behavior to preserve:**
- A quoted pipe character stays an ordinary argument value
- A line with no pipe outside quotes dispatches exactly as today, with the raw answer
- Operator surfaces keep splitting the chain themselves, so their lines reach `Dispatch` with no pipe
- `dispatch-command-args` carries pre-tokenized args and never reaches the tokenizer, so a pipe value there stays data
- `BeginAccounting` and the typed authorizer still tokenize exactly as today (the RFC 8907 Section 8.3 record of every entered command is unchanged)

**Behavior to change:**
- A pipe chain on a plugin's `dispatch-command` (and any other caller that passes an unsplit line to `Dispatch`) is refused before any handler, plugin process or subsystem receives it, with an error naming the chain and the line to send instead

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Pipe chain split | `internal/component/command/pipe.go` `ProcessPipesDefaultFormatChecked`, `parsePipeOps`, `expandAliases`, `bindAddressFields`, `foldFilters` | Splits the chain off a line, folds command-owned filters into args, returns the command and a text formatter, or a refusal | extends: a new exported helper in the same file composes `parsePipeOps`, `expandAliases`, `bindAddressFields` and `foldFilters` over a command part and an operator text the dispatcher already separated, and answers the folded line and whether the whole chain folded. The formatter is not used: it renders text |
| Flag-leftover refusal | `internal/component/plugin/server/command.go` `firstFlagToken`, `flagRefusal` | Refuses a client-side flag that leaked into dispatch, with Status error and the refusal as `Response.Error` | extends: the pipe refusal takes the same shape (Response with Status error, the error returned beside it), built as an `opererr` error with problem, why and fix |
| Tokenizer | `internal/component/plugin/server/command.go` `tokenize` | Strips quotes and splits on whitespace, so quoted and bare pipes are indistinguishable after it | extends: its loop becomes one scanner that also reports the byte offset of the first pipe outside quotes; `tokenize` keeps its signature and output for every other caller, and `Dispatch` reads the offset. One quote rule, two consumers |
| Backslash refusal | `internal/component/plugin/server/command.go` `errBackslashInCommand` | A precedent for refusing a character class at tokenization | does not apply: a backslash is refused for every caller, accounting included; the pipe refusal must leave accounting's tokens unchanged, so it is read in `Dispatch`, not raised by `tokenize` |
| Doctor code for a refused plugin request | `internal/component/plugin/server/dispatch.go` `dispatchCommandResponse`, `diagnostic.CodeDoctorPluginRequestRefused` | Logs and raises every dispatch error a plugin's request meets | extends: a plugin that sends a chain is reported under this existing code, with no new code |
| Secret redaction | `config.DisplayCommand` (used by `unauthorizedError`) | Masks secret values in an echoed command line | extends: the refusal echoes the command and the suggested line through it |
| Operator-surface split | `internal/component/ssh/ssh.go` exec channel, `internal/component/web/cli_terminal.go`, `internal/component/cli/model_mode.go`, `pipedEngineDispatch` | Each splits before dispatch | does not apply: unchanged in behaviour; two comments that say the dispatcher gives the pipe no meaning are corrected |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin's `dispatch-command` RPC (socket or DirectBridge), carrying one command line as text
- Every other caller of `Dispatch` with a free line: `serverDispatcher` (MCP `ze_execute`, REST execute, gRPC), `Reactor.ExecuteCommand`, `handleSystemDispatch`

### Transformation Path
1. `Server.dispatchCommand` / `dispatchCommandAnswer` call `dispatchCommandResponse`
2. `Dispatcher.Dispatch` defers accounting, then scans the line once: tokens plus the offset of the first pipe outside quotes
3. When an offset is found, `Dispatch` builds the refusal from the command part (before the offset) and the operator text (after it), asking the new pipe helper for the folded line, and returns a Response with Status error and the error. No resolution, authorization, handler, subsystem or plugin process runs
4. Otherwise the existing path runs on the same tokens, unchanged
5. `dispatchCommandResponse` logs the error with `CodeDoctorPluginRequestRefused`; the transport writes the existing error answer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | `dispatch-command` text line in, error answer out (existing form) | No |
| API client ↔ Engine | REST / MCP / gRPC line through `serverDispatcher` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/` (Go, `.ci`, scripts). Terms: `DispatchCommand(`, `DispatchCommandResult(`, `dispatch-command`, `ProcessPipesDefaultFormat`, a pipe character inside a dispatched literal. Positive control: `internal/test/fixture/plugin_fixture_16.go` `plugin16PipeFirstLast` is found by the `DispatchCommand` term and sends the folded form `show bgp rib received first 3 count`. Findings: 76 non-test lines call `DispatchCommand`/`DispatchCommandResult`; none passes a literal holding a pipe; no `.ci` and no script under `test/` sends `dispatch-command` with a pipe; the dynamic producers are the ExaBGP bridge (`internal/plugins/exabgp/bridgerun/script.go`, translated ExaBGP grammar, which has no pipe operator), `ze bgp plugin cli` (`internal/component/bgp/cli/cmd_plugin.go` `runInteractive`, an operator-typed line) and the route push (`send bgp <peer> <route text>`, a grammar with no pipe). The line format and the answer framing do not change; only the answer to a line holding a pipe outside quotes changes from a handler's answer to the existing error answer.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `dispatch-command` request line | writer | `pkg/plugin/sdk/sdk_engine.go` `DispatchCommand`, `DispatchCommandResult`; every in-tree plugin and fixture; third-party plugins | grep above: no in-tree writer sends a pipe | preserve: format unchanged; a third-party plugin that sent a chain now reads a refusal instead of a wrong success |
| `dispatch-command` request line | reader | `internal/component/plugin/server/dispatch_registry.go` `opDispatchCommand` → `dispatchCommand` / `dispatchCommandAnswer` → `dispatchCommandResponse` → `Dispatch` | read at source | change: `Dispatch` refuses a pipe outside quotes |
| `dispatch-command` answer | writer | `dispatch_registry.go` `recordAnswer` (socket), `plugin.AnswerFor` (DirectBridge) | read at source | preserve: the existing error answer carries the refusal text |
| `dispatch-command` answer | reader | `pkg/plugin/sdk/sdk_engine.go` `DispatchCommand` (status, data, err); `internal/test/fixture` `Dispatch`; `ze bgp plugin cli` prints `error:` and the text | read at source | preserve: framing unchanged; readers already handle an error answer |
| REST / MCP / gRPC execute line | writer | API clients, `internal/component/mcp/tools.go` `ze_execute`, `internal/component/api/rest/server.go` `handleExecute` | read at source: no split before `serverDispatcher` | preserve format; a piped line now gets the refusal instead of an unfiltered answer |
| REST / MCP / gRPC execute line | reader | `cmd/ze/hub/main_servers.go` `serverDispatcher` → `Dispatch` | read at source | change: `Dispatch` refuses a pipe outside quotes |
| REST / MCP / gRPC execute answer | writer | `internal/component/plugin/dispatch.go` `ResponseJSON`; `internal/component/plugin/types.go` `RefusalLine`; MCP `run` and `ze_execute` (`ErrResult`, `refusalResult`) | read at source | preserve: the existing error envelopes carry the refusal text |
| REST / MCP / gRPC execute answer | reader | API clients outside the repository | the error envelope is the existing one | preserve: error form unchanged |

### Integration Points
- `Dispatcher.Dispatch` - where the refusal lands, ahead of resolution
- `tokenize` - its loop becomes the shared scanner that reports the pipe offset
- `internal/component/command/pipe.go` - new exported fold helper for the refusal's suggested line
- `Server.dispatchCommandResponse` - unchanged caller; its log and doctor code carry the refusal

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the refusal sits in `Dispatch`, which every caller already reaches; no transport or surface gains a check of its own |
| No unintended coupling (components stay isolated) | Yes | `plugin/server` already imports `internal/component/command`; the helper is in the command model, which knows no owner |
| No duplicated functionality (extends existing, does not recreate) | Yes | one quote rule (the scanner `tokenize` uses); one pipe parse, alias expansion and fold (`parsePipeOps`, `expandAliases`, `bindAddressFields`, `foldFilters`) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the scan is the existing single pass; the refusal path allocates only on a refused line |
| Registration over hardcoding, outbound | Yes | the folded suggestion reads the pipe-filter and alias registries; no command is named in the dispatcher |
| Registration over hardcoding, inbound | Yes | no new registry and no enumeration of operators: any pipe outside quotes is refused, whatever follows it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No plugin in the tree relies on a pipe token reaching its handler as an argument | `foldFilters` comment in `Dispatch`; census grep of 76 non-test `DispatchCommand` call lines, `test/` `.ci` and scripts | A refusal breaks that plugin | Changed-Wire Census search; in-tree fixtures send the folded form (`plugin16PipeFirstLast`, `plugin_fixture_13_rib.go`) | confirmed 2026-11-12: no literal pipe in any dispatched line; dynamic producers (ExaBGP bridge, route push) use grammars with no pipe; `ze bgp plugin cli` is operator-typed and was silently wrong before |
| A-2 | No operator surface hands `Dispatch` a line with a pipe outside quotes after its own split | `ParsePipe` cuts at the first pipe, so the command part it returns holds none; `foldFilters` appends values taken from between pipes | SSH or web would start refusing | `TestDispatchAllowsNonFlagArgs` and the existing pipe `.ci` suites (`test-pipe-first-last`, `rib-pipe-filter`, `plugin-pipe-alias`) stay green | confirmed by reading `ParsePipe`, `parsePipeOps`, `foldFilters` |
| A-3 | The daemon process holds the pipe-filter and alias registries the suggestion reads | `RegisterPipeFilters`, `RegisterPluginAliases` fill the daemon's registries (`commands.md`, "Where an alias resolves") | the suggestion falls back to the generic fix, never a wrong line | `TestDispatchPipeRefusalNamesFoldedForm` | confirmed by reading `pipe_filter.go` and `alias.go` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A refusal keyed on the token refuses a quoted pipe argument | `TestDispatchKeepsQuotedPipeArgument` goes red | the offset comes from the quote-aware scanner, never from the tokens |
| R-2 | Making `tokenize` itself fail would change RFC 8907 accounting (`BeginAccounting` falls back to one token on a tokenize error) and the typed authorizer (`isAuthorizedCommandArgs` denies on a tokenize error) | `TestDispatchPipeRefusalIsAccounted` reads one token | `tokenize` keeps its output; only `Dispatch` reads the offset |
| R-3 | `handleSystemDispatch` re-joins already unquoted args, so `system dispatch` with a quoted pipe value is re-read unquoted and refused | a `system dispatch` test with a quoted pipe | accepted: that path already loses quoting for spaces; the refusal is honest where the old answer was wrong. Named in Known Limitations |
| R-4 | The suggested folded line is wrong when a quoted pipe precedes the chain | `TestDispatchPipeRefusalNamesFoldedForm` case with a quoted pipe before the chain | the helper receives the command part and the operator text the scanner separated, so `ParsePipe`'s quote-blind cut never runs on the command part |
| R-5 | The echoed line carries a secret | `TestDispatchPipeRefusalRedactsSecrets` | command part and suggestion pass through `config.DisplayCommand` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin or API command line with a pipe is refused; a quoted pipe value refused would break that caller (guarded by AC-2) |
| How is it reverted? | single commit revert |
| Who else touches this path? | every surface that calls `Dispatch` (SSH, web, MCP, REST, gRPC, plugins, reactor `ExecuteCommand`, `system dispatch`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin `dispatch-command` over the socket with a pipe chain | → | `dispatchCommandResponse` → `Dispatcher.Dispatch` refusal | `TestDispatchCommandRefusesPipeChain` (`internal/component/plugin/server/dispatch_test.go`) |
| plugin `dispatch-command` over DirectBridge with a pipe chain | → | `dispatchCommandAnswer` → `Dispatcher.Dispatch` refusal | `TestDispatchCommandAnswerRefusesPipeChain` (`internal/component/plugin/server/dispatch_test.go`) |
| a forked plugin against a running daemon | → | the whole path, RIB populated | `test/plugin/dispatch-pipe-chain-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A plugin dispatches a known command followed by a pipe chain outside quotes (`show bgp rib received`, a pipe, `count`) | The plugin receives an error answer; no handler, subsystem or plugin process runs. The message is an `opererr` error: the problem names the command part and the chain; the why says the dispatcher answers data and a pipe chain is applied by the surface that renders the answer; the fix names the exact line to send. When every operator folds into the command's own filters, the fix is the folded line (`show bgp rib received count`) | none: the unfiltered answer is the defect |
| AC-1b | The chain holds an operator that does not fold (a format, `display`, `match` on a command that owns no such filter) | Same refusal; the fix is the command part alone, the one exact line to dispatch and select from its data, and the why names the `ze cli -c` form with the original line (single-quoted for a shell) that applies the chain for an operator | none |
| AC-2 | A plugin dispatches a command whose argument is a quoted pipe character, or a quoted value holding one | The handler receives the pipe character (or the whole value) as its argument, as today | the quoted value, because it is data |
| AC-3 | Any other caller hands `Dispatch` an unsplit line with a pipe outside quotes (MCP `ze_execute`, REST execute, gRPC, `Reactor.ExecuteCommand`) | The caller receives the same refusal as AC-1 | none: the check is in `Dispatch`, the one entry they share |
| AC-4 | A line with no pipe outside quotes, from any caller | Dispatches exactly as today, with the raw answer | every existing dispatch, because the scan changes no token |
| AC-5 | The refused line, with an accountant configured | Accounting START and STOP are recorded with the same tokens `tokenize` produces today (RFC 8907 Section 8.3) | the accounting record, because it is owed for every entered command |
| AC-6 | The refused line from a plugin | `dispatch-command failed` is logged with `CodeDoctorPluginRequestRefused` and the server event is raised | the log line, because a plugin that sends a chain is a plugin defect doctor reports |
| AC-7 | The refused line carries a secret value | The echoed command part and the suggested line show the masked value `config.DisplayCommand` writes | none |
| AC-8 | A pipe outside quotes joined to a word on either side with no space, or with no space before the operator | The caller receives the AC-1 refusal, naming the chain from the first pipe outside quotes, because every operator surface reads it as a chain | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestScanCommandReportsUnquotedPipe` | `internal/component/plugin/server/plugin_rpc_test.go` | the scanner's offset: none for no pipe, the first pipe outside quotes, a quoted pipe ignored, a quoted pipe before an unquoted one; tokens equal `tokenize`'s | |
| `TestDispatchRefusesUnquotedPipeChain` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-1, AC-1b, AC-8 on a registered builtin: handler not called, Status error, problem / why / fix present and naming the chain | |
| `TestDispatchPipeRefusalNamesFoldedForm` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-1 fix line from registered pipe filters (registry snapshot restored in cleanup), and R-4 | |
| `TestDispatchKeepsQuotedPipeArgument` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-2: the handler's validated tokens hold the pipe and the quoted value | |
| `TestDispatchPipeRefusalReachesNoPluginProcess` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-1, AC-3: a plugin-registered command target (the `registerExecuteCommandTarget` helper) gets zero `execute-command` calls | |
| `TestDispatchPipeRefusalIsAccounted` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-5: START and STOP recorded with the unchanged tokens | |
| `TestDispatchPipeRefusalRedactsSecrets` | `internal/component/plugin/server/command_pipe_refusal_test.go` | AC-7 | |
| `TestDispatchCommandRefusesPipeChain` | `internal/component/plugin/server/dispatch_test.go` | wiring, socket path, AC-1, AC-6 | |
| `TestDispatchCommandAnswerRefusesPipeChain` | `internal/component/plugin/server/dispatch_test.go` | wiring, DirectBridge answer path, AC-1 | |
| `TestFoldPipeChain` | `internal/component/command/pipe_fold_test.go` | the helper: whole fold, partial fold, an alias expanding to foldable filters, an invalid operator | |
| existing `TestTokenize`, `TestDispatcherTokenize`, `TestTokenizeRejectsBackslash`, `TestDispatchAllowsNonFlagArgs`, `TestDispatchRejectsFlagShapedArgs` | `internal/component/plugin/server/` | AC-4, unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | the change reads no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dispatch-pipe-chain-refused` | `test/plugin/dispatch-pipe-chain-refused.ci`, fixture `plugin/dispatch-pipe-chain-refused` registered in `internal/test/fixture/plugin_fixture_16.go` | Modelled on `test/plugin/test-pipe-first-last.ci` and `plugin16PipeFirstLast` (a forked plugin over a populated adj-rib-in). The fixture dispatches `show bgp rib received` piped to `count` and expects the error naming the folded line; dispatches the folded line and expects count 5 (positive control); dispatches `show system platform` piped to `json` and expects the generic fix | |

## Files to Modify
- `internal/component/plugin/server/command.go` - the scanner under `tokenize`, the pipe check in `Dispatch`, the refusal builder beside `flagRefusal`
- `internal/component/command/pipe.go` - the exported fold helper over `parsePipeOps`, `expandAliases`, `bindAddressFields`, `foldFilters`
- `internal/component/ssh/ssh.go` - correct the comment saying the tokenizer gives the pipe no meaning
- `internal/test/cli/cmd_engine_steps.go` - correct the `pipedEngineDispatch` comment that describes the old symptom
- `internal/test/fixture/plugin_fixture_16.go` - the new fixture
- `docs/architecture/api/process-protocol.md` - "Inter-Plugin Communication": a pipe outside quotes is refused, with the fold
- `docs/architecture/api/commands.md` - "Command Dispatch": the dispatcher refuses an unsplit chain; the operator surfaces split
- `docs/plugin-development/handlers.md` - "DispatchCommand": send the folded filter form, never a pipe
- `plan/journal/silent-fall-through.md` - the owned row, resolved at closure

## Files to Create
- `internal/component/plugin/server/command_pipe_refusal_test.go` - the Dispatch-level unit tests
- `internal/component/command/pipe_fold_test.go` - the helper's tests
- `test/plugin/dispatch-pipe-chain-refused.ci` - the functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no RPC or leaf changes |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | the folded form is the existing grammar |
| Editor autocomplete | N-A | no completion change |
| Functional test for new RPC/API | Yes | `test/plugin/dispatch-pipe-chain-refused.ci` |
| Pipe completeness | Yes | operator surfaces keep `ProcessPipes`; the dispatcher refuses, it does not render |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Doctor code per problem class | Yes | the refusal from a plugin is reported under the existing `CodeDoctorPluginRequestRefused` in `dispatchCommandResponse` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | a refusal of a wrong answer, no feature level |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` "Command Dispatch" |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | framing unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` "Inter-Plugin Communication", `docs/plugin-development/handlers.md` "DispatchCommand" |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8907 accounting is preserved, not changed |
| 10 | Test infrastructure changed? | No | a fixture added in the existing registry |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/api/process-protocol.md` (declared by `command.go`): updated, row 8. `docs/architecture/system-architecture.md` (declared by `ssh.go`): unaffected, only a comment in `ssh.go` changes and the exec channel's split is unchanged. `docs/architecture/testing/ci-format.md` (declared by `cmd_engine_steps.go`): unaffected, only the `pipedEngineDispatch` comment changes and the engine-step format is unchanged. Re-run `./le spec citation anchors spec plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md` at implementation for the advisory source anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `dispatch-command` examples in `process-protocol.md` and `handlers.md` carry no pipe; check they stay correct |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- about 40 tool calls
   - Tests: `TestDispatchCommandRefusesPipeChain`, `TestDispatchCommandAnswerRefusesPipeChain`, the `dispatch-pipe-chain-refused` fixture and `.ci`
   - Files: `dispatch_test.go`, `plugin_fixture_16.go`, `test/plugin/dispatch-pipe-chain-refused.ci`
   - Verify: the tests fail because the handler runs and answers success
2. **Phase: Refusal** -- about 60 tool calls
   - Tests: `TestScanCommandReportsUnquotedPipe`, `TestFoldPipeChain`, every test in `command_pipe_refusal_test.go`
   - Files: `command.go` (scanner, check, refusal builder), `pipe.go` (fold helper)
   - Verify: unit tests fail, implement, pass; the wiring tests and the `.ci` go green; the existing pipe `.ci` suites named in A-2 stay green; record the red of the `.ci` with the check removed
3. **Phase: Docs and comments** -- about 25 tool calls
   - Files: `process-protocol.md`, `commands.md`, `handlers.md`, the `ssh.go` and `cmd_engine_steps.go` comments, the flag-check comment in `Dispatch` if it reads wrong
   - Verify: `./le doc check all` on the edited pages (main thread)

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | a forked plugin, an internal plugin and an API caller all meet the refusal |
| Correctness | the check runs after the accounting defer and before `matchBuiltinTokens`, the subsystem lookup and `dispatchPlugin`; the offset is read from the quote-aware scan, never from the tokens |
| Correctness | `tokenize` output is byte-identical for every input; `BeginAccounting` and `isAuthorizedCommandArgs` are untouched |
| Correctness | the suggested folded line is produced by `foldFilters` itself, and the helper offers a fold only when no operator remains and none is invalid |
| Data flow | no rendering in `Dispatch`; no new op in `engineOps`; no surface gains its own check |
| Operator errors and doctor codes | the refusal is an `opererr` error with its fix on the last line, both fix forms exact and copyable; the plugin case logs under `CodeDoctorPluginRequestRefused` |
| Rule: stale comments | the `ssh.go` and `pipedEngineDispatch` comments no longer say the dispatcher gives the pipe no meaning |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| the pipe check in `Dispatch` | `TestDispatchRefusesUnquotedPipeChain` |
| the quote-aware offset | `TestScanCommandReportsUnquotedPipe`, `TestDispatchKeepsQuotedPipeArgument` |
| the fold helper | `TestFoldPipeChain` |
| both transports | `TestDispatchCommandRefusesPipeChain`, `TestDispatchCommandAnswerRefusesPipeChain` |
| end to end | `./le test functional plugin dispatch-pipe-chain-refused` |
| the page edits | `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | the refusal runs before authorization and must reveal nothing authorization would hide: it echoes only the caller's own line and the folded grammar the help already publishes |
| Error leakage | the echoed line passes through `config.DisplayCommand` (AC-7) |
| Authorization fail-open | no path that refused before now runs a handler; the refusal never returns a success Response |
| Accounting | a refused line is still accounted (AC-5) |
| Resource exhaustion | the fold helper runs once per refused line on a line already bounded by the request framing |

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

- The folded filter form is the dispatch grammar and the pipe is the operator's spelling of it, so a dispatch caller is pointed at the form it should have sent, not given a second grammar.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-12 | `Dispatcher.Dispatch` passes a plugin's pipe chain to the handler as arguments |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Refuse a pipe outside quotes in `Dispatch` | Apply the chain as SSH does (`ProcessPipesDefaultFormatChecked` then the formatter) | `processPipesDefaultFormat` always appends a format operator and its formatter returns rendered text through `ApplyPipes`. `dispatch-command` answers data: `DispatchCommandOutput.data` carries raw JSON and the record form carries rows (`recordAnswer`, `plugin.AnswerFor`). Applying it would turn the answer of a piped line into text, break every reader that decodes `data`, and force a record answer to be flattened. It would also put rendering in the dispatcher, which the commands door forbids (a handler answers data; shaping is at the surface edge) |
| Refuse rather than fold-and-dispatch | Fold command-owned filters into args (`foldFilters`) and dispatch, refusing only what does not fold | `foldFilters` moves only command-owned `match` / `count` / `first` / `last` and named filters, and only before a format; the same operator stays a rendering step on another command. One line would then mean dispatch on one command and refusal on its neighbour, and a dispatch caller would have two grammars for one request. The refusal names the folded line, so the caller gets the exact form, and the dispatch grammar stays one form |
| The check lives in `Dispatch`, not at the plugin caller | A check in `dispatchCommandResponse` only | MCP `ze_execute`, REST execute, gRPC, `Reactor.ExecuteCommand` reach `Dispatch` unsplit too (`serverDispatcher`, `tools.go`, `server.go`); one check covers every caller, and the plugin-infra door forbids a second path per op |
| The offset comes from the scanner `tokenize` uses | A second scan in `Dispatch`; raising an error from `tokenize` | A second scan is a second quote rule. An error from `tokenize` would change `BeginAccounting` (it falls back to one token) and the typed authorizer (it denies on a tokenize error), so the accounting record for RFC 8907 Section 8.3 would change |
| Any pipe outside quotes is refused, joined to a word or not | Refuse only a token that is exactly a pipe | `ParsePipe` cuts at any pipe character, so every operator surface reads a word, a pipe and a word with no spaces as a chain; refusing only the bare token would let the dispatcher read as data what every other surface reads as a chain |

## Known Limitations
- `handleSystemDispatch` re-joins its args unquoted, so `system dispatch` cannot carry a quoted pipe value (R-3); the same path already loses quoting for spaces.
- MCP `ze_execute`, REST execute and gRPC are operator surfaces that do not split the chain at their edge; after this spec they refuse a piped line instead of answering it wrong. Applying the chain there is surface work outside this spec's goal.

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
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Targeted tests pass: `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/component/command/`, and `./le test functional plugin dispatch-pipe-chain-refused`, with `test-pipe-first-last`, `rib-pipe-filter` and `plugin-pipe-alias` as regression

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only

---

## Implementation Summary

### What Was Implemented
- `scanCommand` (`internal/component/plugin/server/command.go`): one quote-aware pass answering the tokens `tokenize` answered before and the offset of the first pipe outside double quotes; `tokenize` now calls it and drops the offset
- `Dispatcher.Dispatch`: after the accounting defer and before builtin, subsystem or plugin resolution, a line with a pipe outside quotes is answered `plugin.OperatorErrorResponse(pipeChainRefusal(...))` with the error beside it
- `pipeChainRefusal`: an `opererr` error; problem names the command part and the chain, why says the dispatcher answers data and the rendering surface applies a chain, fix is the folded line (`command.FoldPipeChain`) or, when the chain does not fold whole, the command part alone with the `ze cli -c` form in the why (`zeCLIForm`, shell single-quoted)
- `command.FoldPipeChain` (`internal/component/command/pipe.go`): `parsePipeOps`, `expandAliases`, `bindAddressFields`, `foldFilters` over the separated command and chain; whole only when no operator remains
- Fixture `plugin16DispatchPipeChainRefused` and `test/plugin/dispatch-pipe-chain-refused.ci`
- Commits: 2616478e64 (refusal, tests, fixture, docs), 8647c73422 (`.ci`), 9731b192f4 (journal rows and the follow-up spec), plus the closure fix below

### Bugs Found/Fixed
- Closure review: the AC-1b fix line was prose naming two alternatives (`dispatch "X" and select from its data, or run ze cli -c '...'`), which `ai/rules/user-facing-errors.md` forbids (the fix is the exact command alone). Now the fix is the command part and the `ze cli -c` form sits in the why. Covered by `TestDispatchRefusesUnquotedPipeChain`, `TestDispatchPipeRefusalNamesFoldedForm`, `TestDispatchPipeRefusalRedactsSecrets`, `TestDispatchCommandRefusesPipeChain`, `TestDispatchCommandAnswerRefusesPipeChain` and the fixture, each now asserting the fix by equality
- Closure review: the `ze cli -c '<line>'` form broke for a line holding a single quote; `zeCLIForm` writes it `'\''`. Covered by the new subtest `single quote in the line`

### Documentation Updates
- `docs/architecture/api/commands.md` "The surface splits the pipe chain; the dispatcher refuses one" and the surface-table note (2616478e64; closure edit: fix is the command alone, the why names `ze cli -c`)
- `docs/architecture/api/process-protocol.md` "Inter-Plugin Communication" (2616478e64; same closure edit)
- `docs/plugin-development/handlers.md` "DispatchCommand" (2616478e64, unchanged at closure: it says only that the error names the line to send)
- `./le doc check all`: no finding on these three pages (the run fails on other sessions' pages only)

### Deviations from Plan
- AC-1b reworded at closure: the `ze cli -c` form moved from the fix line to the why, so the fix stays one exact command (`ai/rules/user-facing-errors.md` outranks the AC's wording; no information was dropped)

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The design put two alternatives on the opererr fix line | The fix line is the exact command alone; `./le arch operator-error check` does not judge the fix text, so it passed | closure review against `ai/rules/user-facing-errors.md` | fixed in the closure commit |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A piped dispatched line works as on operator surfaces or is refused naming the chain and the fix | Done | `command.go` `Dispatcher.Dispatch`, `pipeChainRefusal` | refused |
| A quoted pipe stays an argument | Done | `command.go` `scanCommand` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDispatchRefusesUnquotedPipeChain`, `TestDispatchPipeRefusalNamesFoldedForm`, `.ci` | |
| AC-1b | Changed | same tests, fixture `show system platform \| json` case | Deviations |
| AC-2 | Done | `TestDispatchKeepsQuotedPipeArgument` | |
| AC-3 | Done | `TestDispatchPipeRefusalReachesNoPluginProcess`; check sits in `Dispatch`, the entry `serverDispatcher` calls | |
| AC-4 | Done | existing `TestTokenize`, `TestDispatcherTokenize`, `TestScanCommandReportsUnquotedPipe` | |
| AC-5 | Done | `TestDispatchPipeRefusalIsAccounted` | |
| AC-6 | Done | `TestDispatchCommandRefusesPipeChain` socket case | |
| AC-7 | Done | `TestDispatchPipeRefusalRedactsSecrets` | |
| AC-8 | Done | `TestDispatchRefusesUnquotedPipeChain` joined cases | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestScanCommandReportsUnquotedPipe` | Done | `plugin_rpc_test.go` | |
| `command_pipe_refusal_test.go` (six tests) | Done | `internal/component/plugin/server/` | |
| `TestDispatchCommandRefusesPipeChain`, `TestDispatchCommandAnswerRefusesPipeChain` | Done | `dispatch_test.go` | |
| `TestFoldPipeChain` | Done | `pipe_fold_test.go` | |
| `dispatch-pipe-chain-refused` | Done | `test/plugin/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify and Files to Create | Done | `git show --stat 2616478e64 8647c73422`; journal row marked fixed at closure |

### Audit Summary
- **Total items:** 9 ACs, 2 requirements, 11 tests
- **Done:** all but AC-1b
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-1b (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A piped line a plugin dispatches is refused with the chain and the fix, never answered as if the chain were absent | functional | `test/plugin/dispatch-pipe-chain-refused.ci`: a forked plugin over a populated adj-rib-in reads the refusal whose fix is exactly `show bgp rib received count`, which then counts 5; red with the refusal removed (the pipe reached bgp-rib as "unknown keyword: \|"), 80 stress runs green (8647c73422 body) |
| A quoted pipe stays data | unit | `TestDispatchKeepsQuotedPipeArgument` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| MCP, REST and gRPC apply the chain at their edge instead of refusing it | surface work outside this spec's goal (Known Limitations) | `plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` |
| `ParsePipe` splitting a quoted pipe on SSH, web and `ze cli` | met while designing; journal row | `plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record`, verdict CLEAN, `hunks` for `command.go`, `commands.md`, `silent-fall-through.md` |
| `./le spec review check` | pass (run at closure, before `./le commit create`) |
| Rounds | 3: round 1 two ISSUEs, round 2 one ISSUE (comment only), round 3 clean |
| Reviewer lenses used | logic+wiring, user-facing errors, security, docs drift, style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-1b fix line is prose with two alternatives, not the exact command (`ai/rules/user-facing-errors.md`) | `command.go` `pipeChainRefusal` | fix is the command part; `ze cli -c` form moved to the why |
| 2 | ISSUE | `ze cli -c '<line>'` not shell-safe when the line holds a single quote | `command.go` `pipeChainRefusal` | `zeCLIForm` escapes `'` as `'\''`; subtest added |
| 3 | ISSUE (round 2) | `zeCLIForm` doc comment named the wrong escape: gofmt rewrites a doubled `'` in doc-comment text into a typographic closing quote, so the comment read `'\` followed by U+201D | `command.go` `zeCLIForm` comment | comment states the escape in words with no doubled quote; comment only, no behavior change |

### Run 2
Over round 1's fixes: `pipeChainRefusal`, `zeCLIForm`, `dispatch_test.go`, `command_pipe_refusal_test.go`, `plugin_fixture_16.go`, `commands.md`, `process-protocol.md`, spec, fragment, journal row, sibling spec. Main-thread reruns after round 1: `./le test unit pkg path ./internal/component/plugin/server/` exit 0, `./le --name session test bgp plugin --pattern dispatch-pipe-chain-refused` PASS. The test edits replace `Contains` on the fix with `Equal` on the one command and move the `ze cli -c` check to the why: they prove more. Findings: 1 ISSUE (row 3), 0 BLOCKER. Style pass: no panic, no allocating formatter, textbuf used.

### Run 3
Over the row 3 comment edit: `gofmt -l` clean, no typographic quote left in the changed Go files. 0 BLOCKER, 0 ISSUE.

NOTEs: the empty-command fix keeps the placeholder `ze cli -c '<command> \| ...'` (no command exists to name; placeholders are precedent in `register_plugin_restart.go`); the refusal runs before authorization and tells an unauthorized caller only its own line and the folded grammar help already publishes; `FoldPipeChain` repeats `parsePipeChain`'s two-call body after its own parse, by design (the quote-blind `ParsePipe` must not run).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/dispatch-pipe-chain-refused.ci`, `command_pipe_refusal_test.go`, `pipe_fold_test.go` | yes | `git show --stat 2616478e64 8647c73422` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-8 | refusal in `Dispatch` before resolution | `command.go` lines 1081-1099 read at closure; after the round 1 fixes `./le test unit pkg path ./internal/component/plugin/server/` exit 0 and `./le --name session test bgp plugin --pattern dispatch-pipe-chain-refused` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| forked plugin `dispatch-command` | `test/plugin/dispatch-pipe-chain-refused.ci` | read: `run "le test fixture plugin/dispatch-pipe-chain-refused"`, fixture dispatches through the SDK over TLS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | census grep in the spec; no in-tree dispatched literal holds a pipe |
| A-2 | confirmed | `ParsePipe` cuts at the first pipe; pipe `.ci` tests green except `rib-pipe-filter` (foreign 820476c6ee, no piped command) |
| A-3 | confirmed | `.ci` fix line equals the folded form bgp-rib's registered filters produce |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| commands.md, process-protocol.md: fix is folded line or command alone, why names `ze cli -c` | `pipeChainRefusal` | yes |
| No for categories 1-3, 5-7, 9-15 | no config, CLI, wire or feature-level change | yes |
