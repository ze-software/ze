# Spec: the pipe chain is honored on every command surface, and a quoted pipe stays an argument

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped daemon behaviour: MCP, REST and gRPC answer a piped command without applying its chain, and every surface that does apply one splits a quoted pipe value as a chain. Both are wrong answers on shipped operator surfaces (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - (spec-plugin-dispatch-pipe-chain-reaches-the-handler, closed, landed the quote-aware `scanCommand` in `internal/component/plugin/server/command.go` and the refusal `Dispatch` answers for an unsplit chain, `pipeChainRefusal`) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two defects in how an operator's pipe chain is split from the command line.

1. **Three surfaces never apply the chain.** SSH (`execMiddleware`), the web
   CLI terminal and the `ze cli` TUI split the chain off the line with
   `command.ProcessPipesDefaultFormatChecked`, dispatch the command part, and
   render the answer through the returned formatter. MCP (`ze_execute`, and
   `run` for every derived tool), REST execute (`handleExecute`) and gRPC
   `Execute` hand the whole line to `Dispatch` through `serverDispatcher`.
   Today the pipe and its operators reach the handler as arguments, so the
   answer is unfiltered or refused for the wrong reason. Once the dependency
   lands, `Dispatch` refuses a pipe outside quotes, so these surfaces refuse
   the chain instead of applying it. The owner rule that every operator command
   runs through the daemon on every command surface (MCP, REST, gRPC, SSH, web)
   with the same output as the local `ze` command makes applying it owed work.
2. **The split is not quote-aware.** `ParsePipe` cuts at the first pipe
   character and `parsePipeOps` splits the rest at every pipe character,
   whether quoted or not, while the dispatcher's `tokenize` honors double
   quotes. A quoted argument that holds a pipe character (a literal pipe, or a
   regular expression alternation given to `match`) is torn into a chain on
   SSH, the web CLI and `ze cli`: the command loses its tail, and the rest is
   refused as an unknown operator, or applied when it happens to name one.

The two share a mechanism: applying the chain on the API surfaces through
today's split would carry defect 2 onto three more surfaces, so the split is
made quote-aware first and every surface then uses the one split.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - one command registry behind every interface
  → Constraint: a command is declared once and reaches every interface; a surface adds nothing of its own for one command
- [ ] `ai/doors/api-surfaces.md` - REST, gRPC and MCP are thin adapters over `serverDispatcher`
  → Constraint: the adapters own no command and read no protocol state; REST turns JSON parameters into command tokens (`api.BuildCommand`) before anything parses them
- [ ] `ai/doors/commands-cli.md` - the command model, `execMiddleware`, the pipe operators as a contract scripts read
  → Constraint: a handler answers data and output goes through `ApplyPipes`; a script reading a json pipe relies on the operators in `pipe_catalog.go`
- [ ] `ai/doors/plugin-infra.md` - `plugin/server` `Dispatcher`, which the dependency changes
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` wiring of `serverDispatcher` and `buildAPIEngine`
- [ ] `docs/architecture/api/commands.md` - "Generic pipes apply to the answer the command produced" and "Where an alias resolves, and where it does not"
  → Decision: for a command the daemon serves, the daemon runs the chain; the page names SSH exec and `ze cli -c` and is silent on MCP, REST and gRPC
- [ ] `docs/architecture/mcp/overview.md`, `docs/architecture/api/architecture.md` - the MCP and REST/gRPC answer envelopes a formatted answer must fit

**Key insights:**
- The web CLI terminal (`cli_terminal.go` `executeTerminalOperational`) already is the pattern for a surface that splits, dispatches through `CommandDispatcher.JSON`, then formats.
- The dependency builds a quote-aware scan in `plugin/server` beside `tokenize`; defect 2 needs that one quote rule reachable from `internal/component/command`, so its home is decided here, not duplicated.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - `ParsePipe` cuts at the first pipe character with `strings.Cut`; `parsePipeOps` splits the tail with `strings.SplitSeq`; `parsePipeChain` expands aliases and binds address fields; `processPipesChecked` and `processPipesDefaultFormat` fold filters, validate the chain for the surface and return the command part, a formatter and a refusal message
- [ ] `internal/component/plugin/server/command.go` - `tokenize` toggles a quote state on every double quote and splits on unquoted blanks, refusing a backslash
- [ ] `internal/component/ssh/ssh.go` - `execMiddleware` calls `ProcessPipesDefaultFormatChecked`, answers a refusal on stderr, dispatches the command part, writes the answer through the formatter
- [ ] `internal/component/web/cli_terminal.go` - `executeTerminalOperational` splits with `ProcessPipesDefaultFormatChecked`, calls `dispatch.JSON` with the command part, formats the output
- [ ] `internal/component/mcp/tools.go` - `ze_execute` resolves the command then calls `s.dispatch.JSON` with the whole line; `run` does the same for every derived tool
- [ ] `internal/component/api/rest/server.go` - `handleExecute` builds the request with `fromRESTExecuteRequest` and calls `APIEngine.Execute`
- [ ] `internal/component/api/grpc/server.go` - `Execute` builds the request with `fromProtoExecuteRequest` and calls `APIEngine.Execute`
- [ ] `internal/component/api/engine.go` - `APIEngine.Execute` judges read-only and authorization on the whole command string, then calls the executor and carries the typed response to the transport
- [ ] `cmd/ze/hub/main_servers.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/main.go` - `serverDispatcher` builds the `CommandContext` and calls `Dispatch` with the surface's line; `buildAPIEngine` wraps it for REST and gRPC; `main.go` builds the web, SSH, MCP and CLI dispatchers
- [ ] `internal/component/plugin/dispatch.go` - `CommandDispatcher.JSON` flattens the response with `ResponseJSON`

**Behavior to preserve:**
- A line with no pipe outside quotes answers exactly as today on every surface, in the same envelope
- SSH, web and `ze cli` answers for an unquoted chain are unchanged
- The REST and gRPC permission-denied status, and the MCP refusal envelope

**Behavior to change:**
- MCP, REST and gRPC apply the operator's chain and answer what `ze cli -c` answers for the same line
- A pipe character inside double quotes is an argument character on every surface

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Chain split, fold and formatter | `internal/component/command/pipe.go` `ProcessPipesDefaultFormatChecked`, `processPipesDefaultFormat` | Splits the chain, folds owned filters into the command, validates the chain, returns the command part and a formatter; used by SSH, web, the TUI and the engine-steps executor | extends: MCP, REST and gRPC call it at their edge, so every surface has one split |
| Chain parse | `pipe.go` `ParsePipe`, `parsePipeOps`, `parsePipeChain` | Cuts at pipe characters without regard to quotes | replaces the cut: the first pipe outside quotes ends the command, and each further pipe outside quotes ends an operator |
| Dispatcher tokenizer | `internal/component/plugin/server/command.go` `tokenize`, and the scan the dependency adds beside it | Honors double quotes; the dependency adds the offset of the first pipe outside quotes | extends: one quote rule, held where both `command` and `plugin/server` reach it (`plugin/server` already imports `command`); no second scanner |
| Pipe-chain refusal in `Dispatch` | dependency spec, AC-1 and AC-3 | Refuses an unsplit chain from any caller | does not change: after this spec only a plugin's `dispatch-command` and `system dispatch` reach it with a chain, which is the dependency's goal |
| Split-then-format surface | `internal/component/web/cli_terminal.go` `executeTerminalOperational` | Splits, calls `CommandDispatcher.JSON` with the command part, formats the output | extends: the pattern the MCP edge follows |
| Shared dispatcher | `cmd/ze/hub/main_servers.go` `serverDispatcher` | One closure per surface into `Dispatch`, answering a typed response | does not apply as the split site unless design shows it can return the formatted answer every envelope needs: it answers data, and SSH and web already split before it |
| REST parameter tokens | `internal/component/api` `BuildCommand` (`api-surfaces.md`) | Turns JSON parameters into command tokens | does not change; design checks whether a parameter value holding a pipe character is quoted so the new split keeps it an argument |
| Answer envelopes | `plugin/dispatch.go` `ResponseJSON`, `RenderedResponse`; REST `writeExecResult`; gRPC `execResultToProto` | Carry the flattened JSON answer | extends: design decides how a chain that renders text (table, text) is carried in each envelope |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator's command line on MCP (`ze_execute` or a derived tool), REST execute or gRPC `ZeService.Execute`; and a line with a quoted pipe value on SSH, web or `ze cli`

### Transformation Path
1. The surface's edge splits the line with the quote-aware split
2. The command part is authorized and dispatched as today
3. The answer goes through the formatter the split returned, into the surface's envelope

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| API client ↔ Engine | REST, gRPC, MCP line through `serverDispatcher` | No |

### Changed-Wire Census

**Search evidence:** to fill at design: scope `internal/`, `cmd/`, `test/`; terms `ProcessPipesDefaultFormat`, `ParsePipe(`, `parsePipeChain`, `ze_execute`, `handleExecute`, `Execute(`; positive control `internal/component/ssh/ssh.go` `execMiddleware`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `ProcessPipesDefaultFormatChecked` - called at the MCP, REST and gRPC edges

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `APIEngine.Execute` can judge read-only and authorization on the command part rather than the whole line | `engine.go` `commandReadOnly` reads the command string | a chain could change the authorization verdict | a test with a read-only caller and a piped read command | unvalidated |
| A-2 | Every MCP, REST and gRPC envelope can carry a text answer as well as JSON | the web CLI already returns formatted text | an operator asking for a table over REST gets JSON or a refusal | design reading of the envelopes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A quote-aware split changes an SSH line that relied on a quote being ignored | an existing `.ci` with a quote and a pipe goes red | read each such test against its expectation before changing it |
| R-2 | An odd number of quotes leaves the rest of the line quoted | a chain that silently becomes an argument | define the unbalanced-quote answer at design, the same on the split and on `tokenize` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Piped commands on MCP, REST and gRPC answer wrongly; a quoted pipe value is refused or split on SSH and web |
| How is it reverted? | single commit revert |
| Who else touches this path? | the dependency spec (`plugin/server` dispatch), `plan/release/fixes/spec-fixit-selector-narrows-through-a-pipe.md` (`foldFilters`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| MCP `ze_execute` with a piped line | → | the split at the MCP edge | [named at design] |
| REST execute with a piped line | → | the split at the REST edge | [named at design] |
| SSH exec with a quoted pipe value | → | the quote-aware split | [named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A piped read command on MCP `ze_execute` | The answer equals what `ze cli -c` prints for the same line | [design] |
| AC-2 | The same line on REST execute and on gRPC `Execute` | The answer carries the same result as AC-1 in the surface's envelope | [design] |
| AC-3 | A command with a double-quoted argument holding a pipe character, on SSH, web, `ze cli`, MCP, REST and gRPC | The handler receives the whole quoted value as one argument and no operator runs | [design] |
| AC-4 | A line with no pipe outside quotes, on any surface | Answers exactly as today | every existing answer, because the split returns the line whole |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/component/command/` | the quote-aware split | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/plugin/` | a piped command over MCP and REST answers filtered | |

## Files to Modify
- `internal/component/command/pipe.go` - quote-aware split
- `internal/component/mcp/tools.go` - split at the edge
- `internal/component/api/engine.go`, or the REST and gRPC edges - split at the edge (site decided at design)
- `docs/architecture/api/commands.md` - which surface runs the chain

## Files to Create
- [named at design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing wiring tests at the MCP and REST edges
   - Tests: the Wiring Test rows
   - Files: `internal/component/mcp/tools.go`, the REST and gRPC edge
   - Verify: the wiring tests fail because the edge does not split
2. **Phase: quote-aware split** -- the one quote rule shared by `ParsePipe` and `tokenize`
   - Tests: the unit test for the split, AC-3
   - Files: `internal/component/command/pipe.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: apply the chain at the API edges** -- AC-1, AC-2
   - Files: `internal/component/mcp/tools.go`, `internal/component/api/`
   - Verify: tests fail, implement, tests pass

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-12 | MCP `ze_execute`, REST `handleExecute`, gRPC `Execute` hand `Dispatch` the pipe chain unsplit |
| `splitter-ignores-the-quoting-its-tokenizer-honors.md` | 2026-11-12 | `ParsePipe` and `parsePipeOps` split a quoted pipe value as a chain |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One spec for both defects | Two specs | Applying the chain on the API surfaces through today's split carries the quote defect onto three more surfaces; one split serves both |

## Known Limitations
- none yet

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
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/command/`, `./le test unit pkg path ./internal/component/mcp/`, `./le test unit pkg path ./internal/component/api/`, and the named tests in `./le test functional plugin`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
