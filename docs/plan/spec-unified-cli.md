# Spec: unified-cli

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/editor/model.go` — current TUI model (moves to `internal/component/cli/`)
4. `internal/component/config/editor/model_mode.go` — mode switching (Edit/Command)
5. `cmd/ze/cli/main.go` — current standalone CLI TUI (to be unified)
6. `internal/component/ssh/session.go` — current SSH session model (to be unified)
7. `pkg/plugin/rpc/text_mux.go` — TextMuxConn for plugin CLI

## Task

Unify the three separate TUI implementations (`ze config edit`, `ze cli`, SSH session) into a single shared model. Move `internal/component/config/editor/` to `internal/component/cli/`. Add Ctrl+Arrow page scrolling. Add `ze bgp plugin cli` as an interactive plugin simulator with autocomplete and configurable 5-stage negotiation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — overall system architecture
  → Constraint: plugin communication is JSON/text over socket pairs
- [ ] `docs/architecture/api/process-protocol.md` — 5-stage plugin protocol
  → Constraint: text mode auto-detected from first byte on Socket A
  → Decision: stages use newline-separated lines terminated by blank line
- [ ] `docs/architecture/api/text-format.md` — post-stage-5 event format
  → Constraint: text events are one line per event, `bye` signals shutdown
- [ ] `docs/architecture/config/yang-config-design.md` — YANG-driven completion
  → Constraint: editor completion uses YANG tree for config paths

### Related Learned Summaries
- [ ] `docs/learned/356-editor-modes.md` — mode switching design (Edit/Command)
  → Decision: mode state saved/restored on switch, separate histories
- [ ] `docs/learned/380-ssh-server.md` — SSH server implementation
  → Constraint: SSH uses Wish middleware chain, SessionModel per connection
- [ ] `docs/learned/383-command-package-extraction.md` — command tree extraction
  → Decision: `internal/component/command/` provides shared command tree building

**Key insights:**
- Editor model already supports Edit/Command modes with state save/restore
- `ze cli` reimplements the Command mode TUI separately (779 lines)
- SSH `SessionModel` is a stripped-down version (205 lines, no completion, no modes)
- All three use Bubble Tea with textinput + viewport
- Plugin text protocol uses `TextMuxConn` with `#N method args` framing after stage 5
- Plugin SDK: `NewTextPlugin()` / `NewTextFromEnv()` handle the 5-stage protocol

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/editor/model.go` (946L) — full TUI: textinput, viewport, completion, dropdown, ghost text, validation, modes, history, paste mode, quit confirmation. Package `editor`.
- [ ] `internal/component/config/editor/model_mode.go` (141L) — `EditorMode` enum (ModeEdit/ModeCommand), `SwitchMode()`, `modeState` save/restore, `editModeCommands` map, `executeOperationalCommand()`.
- [ ] `internal/component/config/editor/model_commands.go` (513L) — 20+ command handlers: set, delete, edit, show, compare, commit, discard, rollback, history, load, run, help, errors, etc.
- [ ] `internal/component/config/editor/model_render.go` (546L) — rendering: prompt, viewport, dropdown, validation highlights, diff gutter, help overlay.
- [ ] `internal/component/config/editor/editor.go` (1019L) — config state: tree manipulation (SetValue, DeleteValue, ContentAtPath), file I/O, pending edits, archive, backup/restore.
- [ ] `internal/component/config/editor/completer.go` (917L) — YANG-driven completions for config paths.
- [ ] `internal/component/config/editor/completer_command.go` — operational command completions from RPC tree.
- [ ] `internal/component/config/editor/validator.go` (375L) — real-time YANG validation.
- [ ] `internal/component/config/editor/model_load.go` (791L) — `load` command: absolute, relative, terminal paste mode.
- [ ] `internal/component/config/editor/diff.go` (228L) — diff gutter annotation.
- [ ] `internal/component/config/editor/reload.go` — daemon reload notification on commit.
- [ ] `internal/component/config/editor/init.go` — RPC registration for editor mode.
- [ ] `cmd/ze/cli/main.go` (779L) — standalone CLI TUI. Own model struct with textinput, viewport, suggestions, history. Connects to daemon via Unix socket. Has tab completion from RPC tree, pipe operators. Package `cli`.
- [ ] `cmd/ze/config/cmd_edit.go` — entry point for `ze config edit`. Creates `editor.Model`, wires command executor via socket, sets up completions. Bridges editor commands to daemon RPCs.
- [ ] `internal/component/ssh/session.go` (205L) — minimal SSH TUI. Own `SessionModel` with textinput, viewport, history. No completion, no modes, no scrolling keys beyond defaults. Receives `CommandExecutor` injection.
- [ ] `internal/component/ssh/ssh.go` (266L) — SSH server lifecycle, Wish middleware, session creation with `teaHandler`.
- [ ] `cmd/ze/plugin/main.go` (76L) — plugin dispatch: `test` subcommand and registry lookup.
- [ ] `cmd/ze/plugin/test_cmd.go` (199L) — `ze plugin test` debugging tool (schema/tree/JSON display, not interactive).
- [ ] `pkg/plugin/rpc/text_mux.go` — `TextMuxConn` for concurrent text RPCs with `#N` framing.
- [ ] `pkg/plugin/sdk/sdk_text.go` — text plugin SDK: `NewTextPlugin()`, `Run()`, 5-stage startup, event loop.
- [ ] `pkg/plugin/sdk/sdk_engine.go` — engine-side SDK methods: `UpdateRoute`, `DispatchCommand`, `SubscribeEvents`, etc.
- [ ] `internal/component/plugin/server/startup_text.go` — engine-side text handshake (external process path).

**Importers of `internal/component/config/editor`** (all must be updated):
- `cmd/ze/config/cmd_edit.go`
- `cmd/ze/config/cmd_diff.go`
- `cmd/ze/config/cmd_history.go`
- `cmd/ze/config/cmd_set.go`
- `cmd/ze/config/cmd_rollback.go`
- `cmd/ze/config/cmd_completion.go`
- `cmd/ze-test/editor.go`
- `internal/component/config/editor/testing/headless.go`
- `internal/component/config/editor/testing/expect.go`
- `internal/component/config/editor/testing/expect_test.go`
- `internal/component/bgp/plugins/cmd/peer/save.go`

**Behavior to preserve:**
- All editor features: YANG completion, validation, diff gutter, ghost text, dropdown, paste mode, commit/rollback, archive notification, reload notification, pending edit recovery
- Mode switching: `edit`/`command` toggle with state save/restore
- `ze cli` socket communication and pipe operator support
- SSH authentication, session limiting, Wish middleware chain
- Shift+Arrow viewport scrolling (line-by-line)
- PgUp/PgDown page scrolling
- Command history (Up/Down) per mode
- Tab completion in both edit and command modes
- `ze plugin test` existing debugging tool

**Behavior to change:**
- Move package from `internal/component/config/editor/` to `internal/component/cli/`
- `ze cli` uses the unified model in Command mode instead of its own model
- SSH sessions use the unified model instead of `SessionModel`
- Add Ctrl+Arrow for page scrolling (alongside existing Shift+Arrow and PgUp/PgDown)
- New `ze bgp plugin cli` subcommand for interactive plugin simulation

## Data Flow (MANDATORY)

### Entry Point — `ze config edit`
- User runs `ze config edit [config-file]`
- `cmd/ze/config/cmd_edit.go` creates `cli.Model` with `cli.ModeEdit` start
- Model gets `Editor` (config tree) + `Completer` (YANG) + `Validator` + command executor (socket)
- Bubble Tea runs model in alt screen

### Entry Point — `ze cli`
- User runs `ze cli [--socket path]`
- `cmd/ze/cli/main.go` creates `cli.Model` with `cli.ModeCommand` start
- Model gets NO `Editor` (nil), NO `Completer` (YANG not needed), NO `Validator`
- Gets command executor (socket) + `CommandCompleter` (RPC tree)
- Edit commands unavailable (editor is nil → commands return "not in edit mode")
- Bubble Tea runs model in alt screen

### Entry Point — SSH session
- SSH client connects → Wish middleware → `teaHandler`
- Creates `cli.Model` with `cli.ModeCommand` start (no config file access currently)
- Model gets command executor (injected), no Editor/Completer/Validator
- Edit mode deferred: requires config file injection infrastructure for SSH
- Bubble Tea runs model in Wish tea middleware

### Entry Point — `ze bgp plugin cli`
- User runs `ze bgp plugin cli [--socket path] [config-file]`
- Routing: `main.go` → `bgp.Run()` → `"plugin"` case → `cmdPlugin()` → `"cli"` case
- Prompt: "Auto-negotiate (a) or manual (m)?"
- Auto: perform 5-stage handshake automatically, then enter interactive mode
- Manual: show each stage, let user edit/send, no timeout, perform for real
- After stage 5: interactive command mode with autocomplete for plugin SDK methods
- Model gets plugin command executor (TextMuxConn) + plugin command completer

### Transformation Path
1. User keystroke → `tea.KeyMsg`
2. `handleKeyMsg()` → priority dispatch (quit, dropdown, help, paste, viewport scroll, history, general)
3. Command entry → `dispatchCommand()` → command handler
4. Command result → `commandResultMsg` → state update
5. `View()` → render viewport, prompt, completions, validation
6. Terminal output via Bubble Tea

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI model ↔ config tree | `Editor` methods (SetValue, DeleteValue, etc.) | [ ] |
| CLI model ↔ daemon | Command executor function `func(string) (string, error)` | [ ] |
| SSH server ↔ CLI model | Wish tea middleware wraps model in `tea.Program` | [ ] |
| Plugin CLI ↔ engine | TextMuxConn over Unix socket or net.Pipe | [ ] |

### Integration Points
- `cli.NewModel(opts)` — constructor with options for starting mode, editor, completer, validator, executor
- `cli.Model` replaces `editor.Model` (same interface), `cmd/ze/cli model` struct, and `ssh.SessionModel`
- `cmd/ze/cli/main.go` becomes thin: parse flags, create socket, build model, run
- `internal/component/ssh/session.go` becomes thin: create model with executor, return
- New `cmd/ze/bgp/cmd_plugin.go`: sub-dispatch for `ze bgp plugin`, `cli` subcommand connects to engine, negotiates, runs model

### Architectural Verification
- [ ] No bypassed layers — all entry points create the same model type
- [ ] No unintended coupling — config-specific code stays in `Editor`, model is generic
- [ ] No duplicated functionality — eliminates two redundant TUI implementations
- [ ] Zero-copy preserved — TUI is display layer, no wire encoding involved

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config edit` | → | `cli.Model` in ModeEdit | `test/editor/commands/show-full.et` (existing .et test) |
| `ze cli` command | → | `cli.Model` in ModeCommand | `test/plugin/cli-log-show.ci` (existing, updated) |
| SSH session | → | `cli.Model` in ModeEdit | `test/parse/ssh-config-valid.ci` (existing, updated) |
| ~~`ze bgp plugin cli` auto~~ | → | ~~5-stage negotiation + cli.Model~~ | **BLOCKED**: AC-9 blocked — no daemon-side plugin connection path |
| ~~`ze bgp plugin cli` manual~~ | → | ~~interactive stages + cli.Model~~ | **BLOCKED**: AC-10 blocked — same |
| Ctrl+Arrow scroll | → | viewport page up/down | `TestCtrlArrowPageScroll` in `cli/model_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config edit foo.conf` | Opens unified `cli.Model` in Edit mode, same features as current editor |
| AC-2 | `ze cli` | Opens unified `cli.Model` in Command mode, edit commands unavailable |
| AC-3 | `ze cli` → type `edit` or `set` | Returns "edit mode not available" (no config file loaded) |
| AC-4 | `ze config edit` → type `command` | Switches to Command mode (same as current behavior) |
| AC-5 | SSH session connects | Opens unified `cli.Model` in Command mode with executor (edit mode deferred — requires config file injection infrastructure for SSH) |
| AC-6 | Shift+Arrow in any mode | Scrolls viewport one line up/down (preserved) |
| AC-7 | Ctrl+Arrow in any mode | Scrolls viewport one page up/down (new) |
| AC-8 | PgUp/PgDown in any mode | Scrolls viewport one page up/down (preserved) |
| AC-9 | ~~`ze bgp plugin cli --socket /path auto`~~ | ~~Performs 5-stage negotiation automatically~~ — **BLOCKED**: daemon has no "connect and become a plugin" path; plugins are launched by the engine, not connected by external clients. Requires daemon-side infrastructure (accept-plugin-connection RPC or similar). |
| AC-10 | ~~`ze bgp plugin cli --socket /path manual`~~ | ~~Shows each stage, user edits/confirms~~ — **BLOCKED**: same as AC-9. Requires daemon-side infrastructure for external plugin connections. |
| AC-11 | Plugin CLI interactive mode | Tab completion for plugin SDK methods (update-route, dispatch-command, subscribe-events, etc.) |
| AC-12 | All existing editor tests pass | `go test ./internal/component/cli/...` passes (relocated package) |
| AC-13 | All existing CLI tests pass | `go test ./cmd/ze/cli/...` passes (using unified model) |
| AC-14 | `make ze-verify` passes | No regressions from unification |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModelStartsInEditMode` | `internal/component/cli/model_test.go` | Default start mode is Edit | |
| `TestModelStartsInCommandMode` | `internal/component/cli/model_test.go` | Command-only mode (no editor) | |
| `TestEditCommandsUnavailableWithoutEditor` | `internal/component/cli/model_test.go` | set/delete/commit return error when editor is nil | |
| `TestModeEditBlockedWithoutEditor` | `internal/component/cli/model_test.go` | Typing "edit" or "set x" in command-only mode returns error, stays in ModeCommand | |
| `TestViewRendersWithoutEditor` | `internal/component/cli/model_test.go` | View() renders without panic when editor nil, shows "Ze CLI" header | |
| `TestCtrlArrowPageScroll` | `internal/component/cli/model_test.go` | Ctrl+Up/Down pages viewport | |
| `TestShiftArrowLineScroll` | `internal/component/cli/model_test.go` | Shift+Up/Down scrolls one line (preserved) | |
| `TestPluginCommandCompleter` | `cmd/ze/bgp/cmd_plugin_test.go` | Plugin SDK methods complete correctly | |
| All existing `model_test.go` tests | `internal/component/cli/model_test.go` | All current editor model tests pass in new location | |
| All existing `editor_test.go` tests | `internal/component/cli/editor_test.go` | All current editor tests pass in new location | |
| All existing `completer_test.go` tests | `internal/component/cli/completer_test.go` | All current completer tests pass in new location | |
| All existing `validator_test.go` tests | `internal/component/cli/validator_test.go` | All current validator tests pass in new location | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing edit tests | `test/parse/edit-*.ci` | Config editing works after relocation | |
| Existing CLI tests | `test/plugin/cli-*.ci` | CLI commands work with unified model | |
| Existing SSH tests | `test/plugin/ssh-*.ci` | SSH sessions work with unified model | |
| ~~`plugin-cli-auto`~~ | ~~`test/plugin/plugin-cli-auto.ci`~~ | ~~`ze bgp plugin cli auto`~~ — **BLOCKED**: AC-9 blocked (no daemon-side plugin connection path) | |
| ~~`plugin-cli-manual`~~ | ~~`test/plugin/plugin-cli-manual.ci`~~ | ~~`ze bgp plugin cli manual`~~ — **BLOCKED**: AC-10 blocked (same) | |

### Future (if deferring any tests)
- Plugin CLI chaos/timeout tests — deferred to spec-bgp-chaos-integration (advanced fault injection)

## Files to Modify

### Phase 1: Relocate package
- `internal/component/config/editor/*.go` → `internal/component/cli/*.go` — rename package `editor` to `cli`
- `internal/component/config/editor/testing/*.go` → `internal/component/cli/testing/*.go` — test infrastructure
- `cmd/ze/config/cmd_edit.go` — update import path
- `cmd/ze/config/cmd_diff.go` — update import path
- `cmd/ze/config/cmd_history.go` — update import path
- `cmd/ze/config/cmd_set.go` — update import path
- `cmd/ze/config/cmd_rollback.go` — update import path
- `cmd/ze/config/cmd_completion.go` — update import path
- `cmd/ze-test/editor.go` — update import path
- `internal/component/bgp/plugins/cmd/peer/save.go` — update import path
- `.golangci.yml` — update path exclusion pattern
- `scripts/add-design-refs.go` — update path mapping

### Phase 2: Unify `ze cli`
- `cmd/ze/cli/main.go` — delete model struct, use `cli.Model` in Command mode. Keep `Run()`, socket connection, RPC imports, flag parsing. Remove ~400 lines of model code. Keep `cliClient` and `AllCLIRPCs()`/`BuildCommandTree()` (used by `ze show`/`ze run`).
- `internal/component/cli/model.go` — add constructor option for command-only mode (editor=nil), add `outputBuf` field, add `hasEditor()` helper, add nil guards in `Update()` WindowSizeMsg, `handleEnter()` exit/quit and mode-switch-to-edit (block if no editor), `Dirty()` accessor, `updateCompletions()` (guard m.completer), `scheduleValidation()` (return nil if no editor)
- `internal/component/cli/model_commands.go` — guard all edit commands with nil-editor check, guard `showConfigContent()`, `runValidation()`
- `internal/component/cli/model_mode.go` — add pipe processing in `executeOperationalCommand()` via `command.ProcessPipesDefaultTable()`
- `internal/component/cli/model_render.go` — guard `m.editor.Dirty()` in `View()` header (show "Ze CLI" when no editor), command-only mode uses accumulating output, help overlay excludes edit commands when no editor
- `internal/component/cli/model_load.go` — defensive nil guard in `handleConfirmCountdown()`

### Phase 3: Unify SSH
- `internal/component/ssh/session.go` — delete `SessionModel`, create `cli.Model` with executor
- `internal/component/ssh/ssh.go` — update `teaHandler` to create `cli.Model` instead of `SessionModel`

### Phase 4: Ctrl+Arrow page scrolling
- `internal/component/cli/model.go` — add `tea.KeyCtrlUp`/`tea.KeyCtrlDown` handlers for page scroll

### Phase 5: Plugin CLI
- `cmd/ze/bgp/cmd_plugin.go` — new sub-dispatch for `ze bgp plugin` with `cli` subcommand
- `cmd/ze/bgp/main.go` — add `"plugin"` case in switch that delegates to `cmdPlugin(args[1:])`
- `internal/component/cli/completer_plugin.go` — plugin SDK method completions

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | Yes | `cmd/ze/bgp/main.go` — add `"plugin"` case routing to `cmd_plugin.go` |
| CLI usage/help text | Yes | `cmd/ze/bgp/cmd_plugin.go` — usage function for `ze bgp plugin` and `ze bgp plugin cli` |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | YANG-driven (unchanged) |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-cli-auto.ci` |

## Files to Create
- `internal/component/cli/completer_plugin.go` — plugin SDK method completions (update-route, dispatch-command, subscribe-events, etc.)
- `cmd/ze/bgp/cmd_plugin.go` — `ze bgp plugin` sub-dispatch with `cli` subcommand: flag parsing, negotiation mode selection, socket connection, TextMuxConn setup, model creation
- ~~`test/plugin/plugin-cli-auto.ci`~~ — **BLOCKED**: AC-9 blocked (no daemon-side plugin connection path)
- ~~`test/plugin/plugin-cli-manual.ci`~~ — **BLOCKED**: AC-10 blocked (same)

## Implementation Steps

### Phase 1: Relocate (mechanical)

1. **Move files** — `internal/component/config/editor/` → `internal/component/cli/`, rename package from `editor` to `cli`
2. **Move test infra** — `internal/component/config/editor/testing/` → `internal/component/cli/testing/`
3. **Update all imports** — all files importing old path get new path
4. **Update tooling** — `.golangci.yml`, `scripts/add-design-refs.go`
5. **Run tests** → `make ze-verify` — all existing tests pass unchanged
6. **Critical Review** — package rename is purely mechanical, no behavior change

### Phase 2: Unify `ze cli`

1. **Write unit tests** — `TestModelStartsInCommandMode`, `TestEditCommandsUnavailableWithoutEditor`, `TestModeEditBlockedWithoutEditor`, `TestViewRendersWithoutEditor`
2. **Run tests** → verify FAIL
3. **Implement** — all changes below in one pass:
   - Add command-only constructor: `cli.NewModel()` accepts options struct; when `Editor` is nil, model starts in Command mode
   - Add `hasEditor()` helper: single predicate used by all nil guards
   - Guard render path: `View()` header shows "Ze CLI [command]" when no editor, skips `m.editor.Dirty()`. Help overlay excludes edit commands when no editor
   - Block mode transition: `handleEnter()` lines 588/594 — if `!hasEditor()`, return "edit mode not available" error, stay in ModeCommand
   - Guard edit commands: `dispatchCommand()` returns error for set/delete/commit/etc. when editor nil
   - Guard ancillary paths: `updateCompletions()` skips `m.completer` calls when nil. `scheduleValidation()` returns nil cmd when no editor. `handleConfirmCountdown()` returns early if no editor
4. **Run tests** → verify PASS
5. **Rewrite `cmd/ze/cli/main.go`** — delete model struct (~400 lines), use `cli.NewModel()` with command executor and `CommandCompleter`. Keep `Run()`, flags, socket, RPC imports.
6. **Run functional tests** → existing `ze cli` tests pass
7. **Critical Review**

### Phase 3: Unify SSH

1. **Write unit test** — `TestSSHUsesUnifiedModel` (verify model creation with executor, no editor)
2. **Run test** → verify FAIL
3. **Update `ssh.go`** — `teaHandler` creates `cli.NewModel()` with executor injection, ModeEdit start, editor if config available
4. **Delete `SessionModel`** — remove from `session.go` (or repurpose file for SSH-specific helpers)
5. **Run tests** → verify PASS
6. **Run functional tests** → existing SSH tests pass
7. **Critical Review**

### Phase 4: Ctrl+Arrow page scrolling

1. **Write unit test** — `TestCtrlArrowPageScroll`
2. **Run test** → verify FAIL
3. **Add key handlers** — in `handleKeyMsg()`, handle Ctrl+Up → `viewport.PageUp()`, Ctrl+Down → `viewport.PageDown()`
4. **Run test** → verify PASS
5. **Critical Review** — verify Shift+Arrow (line) and PgUp/PgDown still work

### Phase 5: Plugin CLI

1. **Design plugin completer** — `PluginCompleter` knows plugin SDK methods: `update-route`, `dispatch-command`, `subscribe-events`, `unsubscribe-events`, `decode-nlri`, `encode-nlri`, `decode-mp-reach`, `decode-mp-unreach`, `decode-update`, plus their argument patterns
2. **Write unit tests** — `TestPluginCommandCompleter`
3. **Run tests** → verify FAIL
4. **Implement `completer_plugin.go`** — completions for plugin SDK methods
5. **Write `cmd/ze/bgp/cmd_plugin.go`**:
   - `cmdPlugin(args)` dispatches on `args[0]`: `"cli"` → `cmdPluginCLI(args[1:])`
   - `cmdPluginCLI(args)`: parse flags (`--socket`, config file optional), prompt auto/manual
   - **Auto mode:** Connect to daemon socket, perform 5-stage handshake via `sdk.NewTextPlugin().Run()`, enter interactive mode with `cli.NewModel()` in Command mode
   - **Manual mode:** Connect to daemon socket, disable timeout, show each stage's message, let user review/edit in the TUI, send when user confirms (`Enter`), collect responses, after all 5 stages complete enter interactive mode
   - Interactive mode uses `cli.NewModel()` with `PluginCompleter` and plugin command executor (wraps `TextMuxConn.CallRPC()`)
6. **Register in `cmd/ze/bgp/main.go`** — add `"plugin"` case in switch that calls `cmdPlugin(args[1:])`
7. **Write functional tests**
8. **Run all tests** → `make ze-verify`
9. **Critical Review**

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error after move | Phase 1 step 3 (missed import update) |
| Existing test fails after move | Phase 1 step 3 (package name mismatch) |
| Edit command works in command-only mode | Phase 2 step 3 (nil guard missing) |
| SSH session crashes | Phase 3 step 4 (model initialization wrong) |
| Ctrl+Arrow not recognized | Phase 4 step 3 (wrong key constant) |
| Plugin negotiation hangs | Phase 5 step 5 (timeout/framing issue) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### Package Naming
The new package is `cli` at `internal/component/cli/`. The `cmd/ze/cli/` package is also named `cli` but at a different import path. Where both are needed, use import alias `componentcli` for the internal package.

### Nil Editor Pattern
The unified model supports "command-only" mode by allowing `editor` field to be nil. This is cleaner than a separate mode flag — but requires nil guards beyond just `dispatchCommand()`.

**Non-command paths that access `m.editor` (MUST guard):**

| Location | Access | Guard strategy |
|----------|--------|----------------|
| `model_render.go` `View()` header | `m.editor.Dirty()` | Skip "[modified]" indicator if editor nil. Called every render frame — highest priority guard |
| `model.go` `handleEnter()` mode switch | `SwitchMode(ModeEdit)` at lines 588-598 | Block transition: if editor nil, return "edit mode not available" error, stay in ModeCommand |
| `model.go` `Update()` WindowSizeMsg | `m.editor.WorkingContent()` | Skip config display if editor nil |
| `model.go` `handleEnter()` exit/quit | `m.editor.Dirty()` | Treat as not dirty if editor nil (allow immediate exit) |
| `model.go` `Dirty()` accessor | `m.editor.Dirty()` | Return false if editor nil |
| `model.go` `updateCompletions()` ModeCommand | `m.completer.Complete()` at line 801 | Guard with `if m.completer != nil` before calling YANG completer methods |
| `model.go` `updateCompletions()` ModeEdit | `m.completer.Complete()` at line 809 | Unreachable if mode switch blocked (row 2), but guard defensively |
| `model_render.go` `configViewAtPath()` | `m.editor.ContentAtPath()` | Never called when editor nil (only from config commands) |
| `model_commands.go` `showConfigContent()` | `m.editor.WorkingContent()` | Return empty if editor nil |
| `model_commands.go` `runValidation()` | `m.editor.WorkingContent()` | Skip validation if editor nil |
| `model_commands.go` `scheduleValidation()` | Returns timer that fires `runValidation()` | Return nil cmd if editor nil (avoid unnecessary timers) |
| `model_load.go` `handleConfirmCountdown()` | `m.editor.Rollback()` via `rollbackConfirmed()` | Defensive: return early if editor nil (should be unreachable) |

**Strategy:** Add `func (m *Model) hasEditor() bool` helper. Two critical checkpoints prevent most nil paths: (1) `View()` header guard — prevents render crash, (2) `SwitchMode(ModeEdit)` block — prevents entering edit mode without editor. Remaining guards are defensive for paths reachable only in edit mode. Guard `Update()`, `handleEnter()`, `Dirty()`, `showConfigContent()`, `runValidation()`, `scheduleValidation()` with early nil checks. Command dispatch guards are in addition to these.

**Rendering in command-only mode:** `View()` header shows "Ze CLI [command]" instead of "Ze Editor [command]" when editor is nil. Help overlay shows only operational commands (excludes set, delete, commit, etc.).

### Plugin CLI Manual Mode
Manual mode shows each stage as a text block in the viewport. User can type modifications in the input line. "Send" (Enter on empty line after stage text) transmits the stage. Engine-side timeout is disabled for manual mode by not sending the `ready` message until user completes all stages — the engine won't start its timeout until it receives the first byte on the plugin socket.

### Output Model — Accumulating vs Replacing

**Decision: Command mode uses accumulating output (like `ze cli` today). Edit mode uses replacing output (like `ze config edit` today).**

When editor is nil (command-only), the model accumulates command output in a scroll-back buffer (`outputBuf`). Each command echoes the prompt + input, then appends the result. The viewport scrolls to bottom after each result. This matches terminal-like CLI behavior.

When editor is present (edit mode), the model replaces viewport content on each command (`setViewportText`). Config commands show the config tree; operational commands replace the viewport content. This matches the VyOS-style editor where the viewport shows the current config state, not history.

**Implementation:** Add `outputBuf string` field to the model. In `handleCommandResult()`, if `m.editor == nil`, append to `outputBuf` and `GotoBottom()`. If `m.editor != nil`, replace viewport content (existing behavior). The mode-switch state already saves/restores viewport content per mode, so switching between edit and command within `ze config edit` preserves both patterns.

### Pipe Processing

Pipe operators (`| table`, `| json`, `| match`, `| count`) are currently handled in both `cmd/ze/cli/main.go:708` (via `command.ProcessPipesDefaultTable`) and `cmd/ze/config/cmd_edit.go:50` (same function). Both split the command from the format function before sending to the executor.

**Decision:** Move pipe processing into the unified model's command execution path. When the model dispatches an operational command (command mode or `run` prefix in edit mode), it calls `command.ProcessPipesDefaultTable(input)` to split command from format, sends the command via executor, and applies the format function to the result. This deduplicates the logic and ensures all entry points (CLI, SSH, plugin CLI) get pipe support.

The executor signature remains `func(string) (string, error)` — it receives the pre-pipe command and returns raw JSON. The model applies pipe formatting after.

### SSH Config Access
SSH sessions need access to the config file for full edit mode. The SSH server's `teaHandler` receives the config path from the server config. If no config file is configured for SSH editing, SSH falls back to command-only mode (like `ze cli`).

## RFC Documentation

N/A — no protocol changes, TUI refactoring only.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
