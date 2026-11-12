# Spec: a plugin's dispatched pipe chain reaches the handler as arguments

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped daemon behaviour: a command a plugin dispatches with a pipe chain gets a silently wrong answer (the whole answer, unfiltered, or a refusal naming the pipe as an argument), which `ai/rules/principles.md` forbids. That is incorrect shipped behaviour (`plan/README.md`, `release/fixes/`), not tooling |
| RFC gaps | - |
| Depends | - |
| Phase | - |
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

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - a plugin can run in another process; the text command dispatcher is the slow path an external caller uses
  → Constraint: the fix behaves the same for an internal plugin (DirectBridge) and a forked one (RPC)
- [ ] `ai/doors/plugin-infra.md` - the dispatcher and `dispatch-command` belong to `plugin/server`
  → Constraint: one engine-op entry per RPC; the fix adds no second dispatch path
- [ ] `ai/doors/commands-cli.md` - pipe operators, pipe aliases, `ApplyPipes`
  → Constraint: a handler answers data, never text; output shaping goes through the pipe layer
- [ ] `docs/architecture/api/commands.md` - "Pipe aliases: a name for an operator chain", "The pipe layer selects and re-sequences. It cannot compute"
- [ ] `docs/architecture/api/process-protocol.md` - the `dispatch-command` runtime RPC and its answer

**Key insights:**
- The dispatcher's comment on flag-shaped leftovers says a pipe filter is folded into trailing args client-side by `foldFilters`, so a pipe token reaching `Dispatch` was never meant to be an argument.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `Dispatch` tokenizes, matches the longest builtin prefix, validates args, refuses flag-shaped leftovers (`firstFlagToken`), then calls the handler; `tokenize` strips quotes and splits on whitespace only
- [ ] `internal/component/plugin/server/dispatch.go` - `dispatchCommandResponse` builds a `CommandContext` for the plugin and calls `Dispatch` with the line unchanged
- [ ] `internal/component/ssh/ssh.go` - the exec channel splits the chain with `command.ProcessPipesDefaultFormatChecked` before dispatch
- [ ] `internal/test/cli/cmd_engine_steps.go` - `pipedEngineDispatch` splits the same way, and its comment records the symptom
- [ ] `internal/component/command/pipe.go` - `ProcessPipesDefaultFormatChecked` and `ProcessPipesDefaultFormatLocal`

**Behavior to preserve:**
- A quoted pipe character stays an ordinary argument value
- A line with no pipe dispatches exactly as today, with the raw answer
- Operator surfaces keep splitting the chain themselves

**Behavior to change:**
- A pipe chain on a plugin's `dispatch-command` (and any other caller that passes an unsplit line to `Dispatch`) is no longer handed to the handler as arguments

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Pipe chain split | `internal/component/command/pipe.go` `ProcessPipesDefaultFormatChecked` | Splits the chain off a line, returns the command and a formatter, or a refusal | extends: the plugin dispatch path uses it, or the dispatcher refuses what it would have split; decided at design |
| Flag-leftover refusal | `internal/component/plugin/server/command.go` `firstFlagToken`, `flagRefusal` | Refuses a client-side flag that leaked into dispatch | extends: the same shape of refusal is the candidate for an unsplit pipe, if refusal is chosen |
| Tokenizer | `internal/component/plugin/server/command.go` `tokenize` | Strips quotes and splits on whitespace, so quoted and bare pipes are indistinguishable after it | extends: a refusal needs the quoting fact, which `tokenize` discards today |
| Operator-surface split | `internal/component/ssh/ssh.go` exec channel, `internal/component/web/cli_terminal.go`, `internal/component/cli/model_mode.go`, `pipedEngineDispatch` | Each splits before dispatch | does not apply: unchanged; the defect is the caller that does not split |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin's `dispatch-command` RPC (socket or DirectBridge), carrying one command line as text

### Transformation Path
1. `Server.dispatchCommand` / `dispatchCommandAnswer` call `dispatchCommandResponse`
2. `Dispatcher.Dispatch` tokenizes and resolves the command
3. The handler runs with the leftover tokens as its arguments

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | `dispatch-command` text line | No |

### Changed-Wire Census

**Search evidence:** to be filled at design. No wire format is expected to change: the line stays text, and only how the daemon reads a pipe in it changes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- `Dispatcher.Dispatch` - where a refusal or split lands
- `Server.dispatchCommandResponse` - the plugin caller

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
| A-1 | No plugin in the tree relies on a pipe token reaching its handler as an argument | `foldFilters` comment in `Dispatch` | A refusal breaks that plugin | grep of every dispatch-command producer | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A refusal keyed on the token refuses a quoted pipe argument | a test with a quoted pipe value goes red | carry the quoting fact out of `tokenize` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin command line with a pipe is refused or reshaped |
| How is it reverted? | single commit revert |
| Who else touches this path? | every surface that calls `Dispatch` (SSH, web, MCP, REST, gRPC, plugins) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin `dispatch-command` with a pipe chain | → | `Dispatcher.Dispatch` | named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A plugin dispatches a known command followed by an unquoted pipe chain | The answer is filtered by the chain as on SSH, or the plugin receives a refusal naming the chain and the fix; which one is fixed at design | none: the unfiltered answer is the defect |
| AC-2 | A plugin dispatches a command whose argument is a quoted pipe character | The handler receives the pipe character as its argument, as today | the quoted value, because it is data |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named at design | `internal/component/plugin/server/` | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/plugin/` | a plugin dispatches a piped command | |

## Files to Modify
- `internal/component/plugin/server/command.go` - `Dispatch`, `tokenize`
- `internal/component/plugin/server/dispatch.go` - `dispatchCommandResponse`, if the split lands at the caller
- `docs/architecture/api/process-protocol.md` - how a pipe on `dispatch-command` is read

## Files to Create
- named at design

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- a failing test through the plugin dispatch path
2. **Phase: refusal or split** -- decided at design

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-12 | `Dispatcher.Dispatch` passes a plugin's pipe chain to the handler as arguments |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

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
- [ ] The targeted tests for what changed pass, run in the main checkout. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Targeted tests pass: `./le test unit pkg path ./internal/component/plugin/server/` and the `.ci` named at design under `./le test functional plugin`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
