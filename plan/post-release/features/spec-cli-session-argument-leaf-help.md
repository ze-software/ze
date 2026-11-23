# Spec: cli-session-argument-leaf-help

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P3 |
| Release reason | Enhancement to interactive CLI session help and completion: shows argument-leaf `ze:help` text in `TreeCompleter.matchChildren` for compiled-in and external commands alike. Commands, arguments, and API parameters already work; this improves session usability after the first release. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

In an interactive CLI session (`ze cli` / SSH), `TreeCompleter.matchChildren` (`internal/component/command/completer.go`) offers argument leaves by name without showing each leaf's own `ze:help` text, for compiled-in and external plugin commands alike. Command-level help and description are shown in the menu, on `?`, and in `system command help`, and MCP/REST/gRPC publish argument-leaf help from `-api` RPC inputs, while no interactive session surface renders an argument leaf's `ze:help`.

Provenance: discovered during phase 11 of `spec-plugin-external-yang-at-startup` (AC-30 / row 22, commit `6448a74128`). Fixed in design separately because displaying argument-leaf help in the completion menu and `?` explanation box applies to every command in the tree and needs a UI/completion design of its own.

Goal: show each argument leaf's declared `ze:help` text in interactive session completion and `?` help for both compiled-in and external plugin commands, derived once from the YANG command tree.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`
  → Constraint: help and completion are derived once from the YANG model, never duplicated per interface or per plugin.
- [ ] `docs/architecture/cli/command-completion.md`
  → Constraint: Tab completion and `?` explanation behavior across operational and configuration modes.
- [ ] `docs/architecture/api/commands.md`
  → Constraint: two help texts (`ze:help` summary and `description` explanation) and parity between compiled-in and external commands.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/completer.go` - `TreeCompleter.matchChildren` offers string/typed argument leaves by name with no description/help text.
- [ ] `test/plugin/external-yang-command-help.ci` - documents in its header that no session surface renders an argument leaf's own `ze:help` for any command.

**Behavior to preserve:**
- Command-level `ze:help` summary and `description` explanation in Tab menu, `?`, and `system command help`.
- Argument completion order, enum value completion, and compiled-in/external parity.

**Behavior to change:**
- Surface argument-leaf `ze:help` text in interactive session completion and `?` help.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Operational command tree and completer | `internal/component/command/completer.go` `TreeCompleter.matchChildren`, `TreeCompleter.Explain` | Builds candidate items from the command tree and explains the selected candidate on `?` | Extends: carry argument-leaf `ze:help` into candidate items and explanations for all commands |
| YANG command tree builder | `internal/component/config/yang/command.go` `BuildCommandTree` | Reads `-cmd` modules and attaches `ze:help` to command and argument nodes | Extends (or reads directly if already populated on leaf nodes) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Interactive SSH / `ze cli` operational prompt pressing Tab or `?` after a command that declares argument leaves in YANG.

### Transformation Path
(fill during design: how `BuildCommandTree` exposes leaf `ze:help`, how `TreeCompleter.matchChildren` populates candidate help, and how the session UI renders it.)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design) | (fill during design) |

## Files to Modify
- `internal/component/command/completer.go` - populate argument-leaf `ze:help` in `TreeCompleter.matchChildren`
- `docs/architecture/cli/command-completion.md` - document argument-leaf help rendering
