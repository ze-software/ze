# Spec: editor-modes

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - editor architecture
4. `internal/component/config/editor/model.go` - current editor model
5. `cmd/ze/cli/main.go` - current CLI TUI (command mode reference)

## Task

Add dual-mode support to the ze editor: **edit mode** (current config editing behavior) and **command mode** (operational commands with autocomplete). Users toggle between modes with `/edit` and `/command`. Each mode saves and restores its own screen state.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - editor architecture
  → Constraint: editor uses Bubble Tea Model with textinput + viewport
- [ ] `docs/architecture/api/architecture.md` - RPC command system
  → Decision: commands dispatched via `AllBuiltinRPCs()` + `BgpHandlerRPCs()`
  → Constraint: CLI uses NUL-framed JSON RPC over unix socket

**Key insights:**
- Two separate Bubble Tea TUIs already exist: config editor (`internal/component/config/editor/`) and operational CLI (`cmd/ze/cli/`)
- The CLI already has command tree completion (`buildCommandTree()` from `allCLIRPCs()`)
- The CLI connects to daemon via unix socket using `cliClient`
- Editor has YANG-driven autocomplete; CLI has RPC-tree-driven autocomplete

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/editor/model.go` - Bubble Tea Model: textinput, viewport, completer, validator. Single-mode config editor.
- [ ] `internal/component/config/editor/model_render.go` - View() rendering: header, viewport, prompt at bottom, dropdown overlay, help overlay
- [ ] `internal/component/config/editor/model_commands.go` - command dispatch: set, delete, edit, show, compare, commit, discard, etc.
- [ ] `internal/component/config/editor/completer.go` - YANG-driven completions: commands list + schema navigation
- [ ] `cmd/ze/cli/main.go` - separate CLI TUI: connects to daemon, command tree, suggestion cycling, executeCommand via RPC

**Behavior to preserve:**
- All existing config editor commands (set, delete, edit, show, compare, commit, discard, etc.)
- YANG-driven autocomplete in edit mode
- Config validation and error highlighting
- Context path navigation (edit, up, top)
- Viewport scrolling behavior
- Ghost text inline completion
- Dropdown behavior (the dynamic sizing + above-prompt positioning)

**Behavior to change:**
- Add mode concept (edit/command) with `/edit` and `/command` toggle commands
- Command mode gets operational command autocomplete (from RPC command tree)
- Screen state saved/restored when toggling between modes

## Data Flow (MANDATORY)

### Entry Point
- User types `/command` or `/edit` in the text input
- These are intercepted before normal command dispatch

### Transformation Path
1. User types `/command` → `handleEnter()` detects `/` prefix → mode switch
2. Current mode's viewport content and scroll position saved to mode state
3. Mode field updated on Model
4. New mode's saved viewport content and scroll position restored
5. Completions recalculated for new mode's command set
6. Prompt changes to reflect mode

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor ↔ CLI client | Command mode needs `cliClient` for RPC execution | [ ] |
| Mode state ↔ Model | Each mode saves/restores viewport, scroll pos | [ ] |

### Integration Points
- `cmd/ze/cli/main.go` `buildCommandTree()` — reuse for command mode completion
- `cmd/ze/cli/main.go` `cliClient` — reuse for command mode RPC execution
- `internal/component/config/editor/completer.go` — edit mode keeps existing completer
- Command mode needs its own completer or adapter over `Command` tree

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| User types `/command` + Enter | → | Mode switches to command, prompt changes | `TestModeSwitchToCommand` |
| User types `/edit` + Enter | → | Mode switches to edit, prompt changes | `TestModeSwitchToEdit` |
| Tab in command mode | → | Command tree completions shown | `TestCommandModeCompletions` |
| Enter in command mode | → | RPC dispatched to daemon | `TestCommandModeExecute` |
| `/command` then `/edit` | → | Edit viewport restored | `TestModeScreenRestore` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Type `/command` + Enter | Mode switches to command mode, prompt shows `ze> ` |
| AC-2 | Type `/edit` + Enter | Mode switches to edit mode, prompt shows `ze# ` (with context) |
| AC-3 | In command mode, press Tab with empty input | Shows top-level operational commands (peer, daemon, rib, system) |
| AC-4 | In command mode, type `peer ` + Tab | Shows peer subcommands (list, show, capabilities, etc.) |
| AC-5 | In command mode, type `peer list` + Enter | Command sent via RPC, response shown in viewport |
| AC-6 | Switch from edit→command→edit | Edit mode viewport content and scroll position restored |
| AC-7 | Switch from command→edit→command | Command mode viewport content and scroll position restored |
| AC-8 | In edit mode, all existing commands work unchanged | set, delete, edit, show, compare, commit, discard, etc. |
| AC-9 | `/edit` while already in edit mode | No-op or status message "already in edit mode" |
| AC-10 | `/command` while already in command mode | No-op or status message "already in command mode" |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModeSwitchToCommand` | `model_mode_test.go` | `/command` changes mode and prompt | |
| `TestModeSwitchToEdit` | `model_mode_test.go` | `/edit` changes mode and prompt | |
| `TestModeSwitchNoop` | `model_mode_test.go` | Switching to current mode is no-op | |
| `TestModeScreenRestore` | `model_mode_test.go` | Viewport content preserved across mode switches | |
| `TestCommandModeCompletions` | `model_mode_test.go` | Tab shows RPC command tree | |
| `TestCommandModeDispatch` | `model_mode_test.go` | Enter sends RPC and shows response | |
| `TestEditModeUnchanged` | `model_mode_test.go` | All existing edit commands still work in edit mode | |
| `TestCommandModeGhostText` | `model_mode_test.go` | Ghost text works for operational commands | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mode-switch-basic.et` | `test/editor/` | Type /command, verify prompt, type /edit, verify prompt | |
| `mode-switch-restore.et` | `test/editor/` | Edit config, /command, run peer list, /edit, verify config still shown | |

### Future
- Command mode with live daemon connection (requires running daemon in test)

## Files to Modify

- `internal/component/config/editor/model.go` - Add mode field, mode state struct, `/command` and `/edit` dispatch
- `internal/component/config/editor/model_render.go` - Mode-aware prompt rendering, mode indicator in header
- `internal/component/config/editor/model_commands.go` - Intercept `/` prefix commands before normal dispatch
- `internal/component/config/editor/completer.go` - Keep as edit-mode completer (no changes)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| CLI usage/help text | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Editor autocomplete | Yes | Command tree adapter for command mode |
| Functional test for new RPC/API | No | |

## Files to Create

- `internal/component/config/editor/model_mode.go` - Mode type, mode state, switch logic
- `internal/component/config/editor/model_mode_test.go` - Mode switching tests
- `internal/component/config/editor/completer_command.go` - Command mode completer (wraps RPC command tree)
- `internal/component/config/editor/completer_command_test.go` - Command mode completion tests

## Design Decisions

### Mode as enum, not interface
Two modes is too few for premature abstraction. Use a simple `EditorMode` enum (`ModeEdit`, `ModeCommand`) and switch statements. If a third mode appears, consider interface.

### Reuse `cliClient` from `cmd/ze/cli/`
The `cliClient` type handles socket connection and RPC framing. Command mode needs this. Options:
1. Move `cliClient` to a shared package (e.g., `internal/component/cli/client/`)
2. Pass a pre-built client into the editor Model

Option 2 is simpler — the editor doesn't need to know how the client is built. The caller (`cmd/ze/config/main.go`) can optionally provide a client. Command mode is unavailable if no client is provided (editor-only / standalone mode).

### Command tree completion
The CLI already builds a `Command` tree from `allCLIRPCs()`. The command mode completer wraps this tree to produce `[]Completion` using the same prefix-matching logic as the CLI's `updateSuggestions()`.

### Mode state struct
Each mode saves: viewport content, viewport scroll offset, status message. On switch, current state is saved, new state is restored.

### `/` prefix for mode commands
All mode commands start with `/` to avoid collision with config or operational commands. This is a familiar convention (IRC, Slack, etc.).

## Implementation Steps

1. **Create `model_mode.go`** — EditorMode type, ModeState struct, switch functions
2. **Write mode switch tests** → Verify FAIL
3. **Add mode field to Model** — default ModeEdit, intercept `/command` and `/edit` in handleEnter
4. **Run tests** → Verify PASS
5. **Create `completer_command.go`** — CommandCompleter wrapping RPC command tree
6. **Write completion tests** → Verify FAIL
7. **Wire command completions into model** — mode-aware `updateCompletions()`
8. **Run tests** → Verify PASS
9. **Add command execution** — command mode sends RPC via optional cliClient
10. **Mode-aware rendering** — prompt, header indicator
11. **Screen save/restore** — ModeState captures viewport content + offset
12. **Functional tests**
13. **`make ze-verify`**
14. **Critical Review + Audit**

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix syntax/types) |
| Test fails wrong reason | Step 2 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Implementation Summary

### What Was Implemented
- Dual-mode editor: ModeEdit (config editing) and ModeCommand (operational commands)
- Mode switching via `/command` and `/edit` with per-mode state save/restore (viewport, scroll, history, historyTmp)
- CommandCompleter wrapping RPC command tree for operational command completion
- Command execution via injected executor function (daemon socket RPC)
- No-daemon UX: warning on mode switch, error on command execute
- List key rejection in YANG validation and completion filtering
- Union type hints in error messages
- Mode-aware prompt rendering and header indicator

### Bugs Found/Fixed
- List key (`address`) was settable inside peer block and offered as completion
- `local-address` accepted garbage (YANG type was string, changed to union of ip-address + auto)
- `isTemplate` branch in `buildPrompt` was dead code (identical to non-template branch)

### Documentation Updates
- Help overlay: added "Modes:" section with `/command` and `/edit`
- CLI usage text: added "Mode switching:" section

### Deviations from Plan
- Spec said modify `model_commands.go` to intercept `/` prefix — actually done in `model.go:handleEnter()` (simpler, before command dispatch)
- `completer.go` was modified (list key filtering, typeHint improvements) — spec said no changes needed

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Mode concept (edit/command) | ✅ Done | model_mode.go:14-21 | EditorMode enum |
| `/command` and `/edit` toggle | ✅ Done | model.go:500-512 | handleEnter intercept |
| Command mode autocomplete | ✅ Done | completer_command.go | CommandCompleter wraps RPC tree |
| Screen state save/restore | ✅ Done | model_mode.go:53-87 | modeState struct |
| Mode-aware prompt | ✅ Done | model_render.go:485-487 | `ze> ` for command, `ze# ` for edit |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestModeSwitchToCommand, mode-switch-basic.et | |
| AC-2 | ✅ Done | TestModeSwitchToEdit, mode-switch-basic.et | |
| AC-3 | ✅ Done | TestCommandModeCompletionsWired, TestCommandModeCompletions | |
| AC-4 | ✅ Done | TestCommandModeSubcommandCompletions | |
| AC-5 | ✅ Done | TestCommandModeDispatch | Via commandResultMsg through Update |
| AC-6 | ✅ Done | TestModeScreenRestore, mode-switch-restore.et | |
| AC-7 | ✅ Done | TestModeScreenRestore | |
| AC-8 | ✅ Done | TestEditModeUnchanged, mode-switch-restore.et | |
| AC-9 | ✅ Done | TestModeSwitchNoop, mode-switch-basic.et | |
| AC-10 | ✅ Done | TestModeSwitchNoop, mode-switch-basic.et | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestModeSwitchToCommand | ✅ Done | model_mode_test.go:13 | |
| TestModeSwitchToEdit | ✅ Done | model_mode_test.go:28 | |
| TestModeSwitchNoop | ✅ Done | model_mode_test.go:39 | |
| TestModeScreenRestore | ✅ Done | model_mode_test.go:61 | |
| TestCommandModeCompletions | ✅ Done | completer_command_test.go:40 | |
| TestCommandModeDispatch | ✅ Done | model_mode_test.go:129 | |
| TestEditModeUnchanged | ✅ Done | model_mode_test.go:209 | |
| TestCommandModeGhostText | ✅ Done | completer_command_test.go:100 + model_mode_test.go:188 | |
| mode-switch-basic.et | ✅ Done | test/editor/mode/ | Functional |
| mode-switch-restore.et | ✅ Done | test/editor/mode/ | Functional |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| model.go | ✅ Modified | Mode fields, handleEnter intercept, mode-aware completions |
| model_render.go | ✅ Modified | Mode-aware prompt, header indicator, help overlay |
| ~~model_commands.go~~ | 🔄 Changed | Not modified — intercept done in model.go instead |
| completer.go | ✅ Modified | List key filtering, typeHint union support (unplanned improvement) |
| model_mode.go | ✅ Created | EditorMode, modeState, SwitchMode, executeOperationalCommand |
| model_mode_test.go | ✅ Created | 11 unit tests for mode switching |
| completer_command.go | ✅ Created | CommandCompleter for operational commands |
| completer_command_test.go | ✅ Created | 6 unit tests for command completions |
| cmd/ze/config/cmd_edit.go | ✅ Modified | wireCommandExecutor, buildEditorCommandTree, usage text |

### Audit Summary
- **Total items:** 25 (5 requirements + 10 AC + 10 tests)
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (model_commands.go → model.go)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit**
