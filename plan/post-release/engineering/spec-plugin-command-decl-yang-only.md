# Spec: plugin-command-decl-yang-only

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | Post-release cleanup refactor: removes duplicated `ShortHelp`, `Description`, and `Args` fields from Stage 1 `CommandDecl` across ~54 Go plugin files so YANG `-cmd` modules are the sole source of command help and argument declarations. Today YANG already wins when both exist. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Today Stage 1 `rpc.CommandDecl` (`pkg/plugin/rpc/types.go`) carries `ShortHelp`, `Description`, and `Args` alongside `Name`, even though every compiled-in command and every external plugin with a `-cmd` module declares its help text and argument leaves in YANG. When both exist, YANG already wins (`registerProcessCommands`, `internal/component/plugin/server/doctor_delivery.go`), leaving the Stage 1 fields as a second declaration across ~54 Go files.

Provenance: left out by the owner's approval of phase 11 in `spec-plugin-external-yang-at-startup` (2026-11-17: wanted later, not declined; listed in its `Work Not Done` table).

Goal: remove help text and argument names from Stage 1 `CommandDecl` (and update `ze-plugin-engine.yang`, the SDK, all ~54 in-tree plugins, the Python library, and `docs/plugin-development/commands.md`) so YANG `-cmd` modules are the single source of truth for command help and arguments.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/commands-cli.md`
  → Constraint: `pkg/plugin/rpc` fields are wire contract; removing fields requires updating every in-tree plugin, `ze-plugin-engine.yang`, the Python library, and plugin development docs together.
- [ ] `docs/architecture/api/process-protocol.md`
  → Constraint: Stage 1 `declare-registration` wire fields.
- [ ] `docs/plugin-development/commands.md`
  → Constraint: documents how Stage 1 `ShortHelp`/`Description` fall back for commands without a `-cmd` module today.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl` holds `Name`, `ShortHelp`, `Description`, and `Args`.
- [ ] `internal/component/plugin/server/doctor_delivery.go` - `registerProcessCommands` lets YANG help win over Stage 1 `ShortHelp`/`Description`, using Stage 1 only when no YANG node declares them.

**Behavior to preserve:**
- Every operator-facing command retains its `ze:help` summary, `description`, completion, and argument validation from YANG.
- Hidden internal plugin commands without operator YANG nodes keep working (design must settle whether hidden internal commands remain exempt or gain minimal declarations).

**Behavior to change:**
- Delete `ShortHelp`, `Description`, and `Args` from `rpc.CommandDecl`, `ze-plugin-engine.yang`, and all ~54 plugin call sites once every operator command is backed by YANG.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| YANG `-cmd` command declarations | `internal/component/config/yang/command.go` `BuildCommandTree`, `command_argdefs.go` `PathToArgDefs` | Single source of help, completion, and argument definitions for compiled-in and external `-cmd` modules | Extends: becomes the sole source once Stage 1 help/arg fields are removed |
| Stage 1 declaration check | `internal/le/plugin/declarations/` (`./le plugin declarations check`) | Verifies `Registration.Commands` matches live Stage 1 `CommandDecl` | Extends: drops the retired fields from its comparison |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Stage 1 `declare-registration` RPC from internal and external plugins.

### Transformation Path
(fill during design: audit all ~54 plugins for commands lacking a YANG `-cmd` node, then remove `ShortHelp`, `Description`, and `Args` from `CommandDecl` and `PluginRegistration`.)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design) | (fill during design) |

## Files to Modify
- `pkg/plugin/rpc/types.go` - remove `ShortHelp`, `Description`, `Args` from `CommandDecl`
- `internal/core/ipc/yang/ze-plugin-engine.yang` - mirror removal on the wire model
- `internal/component/plugin/registration.go`, `internal/component/plugin/server/startup.go`, `doctor_delivery.go` - remove Stage 1 help maps
- In-tree plugin `commandDecls()` call sites and `examples/plugin/python/ze_plugin.py`
- `docs/plugin-development/commands.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/README.md`
