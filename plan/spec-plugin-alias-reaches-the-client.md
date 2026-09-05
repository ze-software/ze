# Spec: a plugin's pipe alias reaches the CLI client

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A pipe alias a plugin declares lives in the DAEMON's alias registry. Two client
surfaces resolve a pipe chain in the CLIENT process instead, so on both of them
a declared name is neither offered by Tab nor resolvable, and the operator reads
`pipe error: unknown pipe operator: <name>`.

| Surface | Parses the chain | A plugin's alias works |
|---------|------------------|------------------------|
| `ze cli -c "<command>"`, and any SSH exec channel | the daemon | Yes |
| the TUI a plain ssh client with a pty reaches | the daemon | Yes |
| `ze cli` with no command argument | the CLIENT process | No |
| `cliClient.StreamMonitor`, for a streaming monitor command | the CLIENT process | No |

The aliases compiled into the client resolve there, which is what hid the gap:
Tab after the pipe character on `show bgp` offers `summary` and `peers` in the
same client that cannot offer a plugin's name.

`spec-plugin-registers-pipe-operations` built the declaration channel and closed
on 2026-09-05 with both client rows outstanding. It named this spec as their
owner. Nothing here changes the daemon side.

The repair is a wire surface that carries the daemon's alias table to the client
at session start, beside the runtime command list `buildRuntimeTree` already
fetches. Three decisions have to be made before any of it is written, which is
why this is a spec rather than an edit.

| Decision | The question |
|----------|--------------|
| The wire surface | Which RPC carries the table, and whether it rides on the existing runtime-tree fetch or takes one of its own |
| The client-side collision rule | The daemon refuses a colliding declaration and fails the plugin's start. A client that receives a name its compiled-in table already carries cannot fail a plugin, so it needs a rule of its own |
| A plugin that stops mid-session | The client holds a table fetched at session start. A plugin that stops takes its name out of the daemon's registry and the client keeps offering it |

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/api/commands.md` - "Where an alias resolves, and where
      it does not", and "Discovery: the running daemon is the only source"
- [ ] `docs/architecture/cli/command-completion.md` - the completion path in the
      client

## Current Behavior (MANDATORY)

- [ ] `internal/component/cli/model_mode.go` - `executeOperationalCommand`
      resolves the chain with `command.ProcessPipesDefaultFormatChecked` in the
      client process, before anything is sent
- [ ] `internal/component/cli/client/main.go` - `runInteractiveSession` builds
      the Bubble Tea model with `tea.NewProgram` in the client
- [ ] `internal/component/command/alias.go` - `AliasesForCommand`, the daemon-side
      answer a client would have to receive
- [ ] `internal/component/command/completer.go` - `completePipeForCommand`, which
      reads the registry of whichever process it runs in
- [ ] `cmd/ze/hub/session_factory.go` - `buildSessionModelFactory`, the
      daemon-hosted model that works today and is the shape the client needs

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator types a pipe character in `ze cli` with no command argument, or on a
streaming monitor command. The client's Bubble Tea model owns the line.

### Transformation Path

Today: the model calls `completePipeForCommand`, which reads the CLIENT
process's alias registry, which holds the compiled-in aliases alone.

Wanted: the client's registry also holds what the daemon declared, fetched at
session start.

### Boundaries Crossed

The CLI client process and the daemon, over whatever RPC the first decision
above picks.

### Integration Points

The runtime command tree the client already fetches at session start, and the
alias registry in `internal/component/command`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator types `show <plugin command> \| <alias>` in `ze cli` with no command argument | → | the client resolves the name from the table the daemon sent | a `.ci` under `test/ui/`, driving `ze cli` over a pty as `ui/display-fill-completion` does |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator types a plugin's declared alias in `ze cli` with no command argument | The chain resolves and the answer is the expansion's |
| AC-2 | An operator presses Tab after the pipe character on a plugin command in the same client | The declared name is offered beside the built-in operators |
| AC-3 | A plugin stops while the session is open | The client stops offering the name, by whatever mechanism the third decision picks |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The client fetches a runtime command tree at session start that a table can ride on | `buildRuntimeTree` | The table needs an RPC of its own | reading `buildRuntimeTree` | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A client-side table that disagrees with the daemon offers a name the daemon refuses | An operator completes a name and the command fails | The third decision above |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| the client-side registration refuses or accepts a name its compiled-in table carries | `internal/component/command/alias_test.go` | the second decision, once it is made | to write |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| a declared alias resolves in `ze cli` | `test/ui/` | AC-1 and AC-2 | to write |

## Files to Modify

- `internal/component/cli/client/main.go` - fetch the table at session start
- `internal/component/command/alias.go` - the client-side registration entry point

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | whichever RPC carries the table |
| Functional test for new RPC/API | Yes | a `.ci` under `test/ui/` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, whose "Plugin-Declared Pipe Aliases" row states the client gap |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the "Where an alias resolves" table |

## Implementation Steps

1. **Phase: the three decisions** - answer them with the owner before code
2. **Phase: the wire surface** - the daemon answers its alias table
3. **Phase: the client** - the table is fetched, registered and completed from

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-3 all demonstrated
- [ ] `./le verify worktree` passes

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
