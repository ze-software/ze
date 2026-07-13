# Spec: command-completion

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/cli-patterns.md` - Command Completion section
4. `internal/component/plugin/server/command_registry.go` - plugin command registry
5. `internal/component/config/yang/command.go` - BuildCommandTree

## Task

Make every plugin-registered command appear in CLI tab-completion automatically,
without requiring a per-plugin YANG command file. Currently 51 commands across
12 plugins work but lack tab-completion because the completion tree is built
only from YANG, not from the plugin command registry.

**Design decision (user-approved):** Approach A -- inject CommandRegistry entries
into the completion tree after plugin registration. No per-plugin YANG files needed.

**Opt-out:** Add `Hidden bool` to `CommandDecl`. Hidden commands work when typed
in full but don't appear in completion or help. Exception, not default.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall registration pattern
  → Decision: registration-based discovery is the pattern; completion must follow it

### RFC Summaries (MUST for protocol work)
N/A -- not protocol work.

**Key insights:**
- Dispatcher already has two-phase lookup: builtins (YANG-backed) then plugin registry
- Plugin registry already stores command name, description, args, completable flag
- BuildCommandTree produces a command.Node tree; plugin commands need to be injected into this same tree
- The completion tree is built at startup from YANG; plugin commands register later (after plugin startup)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command_registry.go` - stores RegisteredCommand with Name, Description, Args, Completable, Hidden (to add)
- [ ] `internal/component/config/yang/command.go` - BuildCommandTree, WireMethodToPath, PathToDescription, PathToArgDefs
- [ ] `internal/component/plugin/server/command.go` - Dispatcher.Dispatch: phase 1 (builtins) then phase 3 (plugin registry via dispatchPlugin)
- [ ] `pkg/plugin/rpc/types.go` - CommandDecl struct: Name, Description, Args, Completable, DeprecatedNames
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - ExecuteCommandHandler receives (serial, command, args, peer)

**Behavior to preserve:**
- All 51 commands continue to work exactly as today
- YANG-backed commands continue to get completion from YANG
- Dispatch order: builtins first, plugin registry second
- Plugin commands validate verb-first grammar at registration

**Behavior to change:**
- Plugin-registered commands appear in tab-completion tree
- New `Hidden` field on CommandDecl suppresses completion (opt-out)

## Data Flow (MANDATORY)

### Entry Point
- Plugin startup: plugin calls `declare-registration` RPC with CommandDecl list
- CLI: user presses tab after typing partial command

### Transformation Path
1. Plugin sends CommandDecl list during registration
2. CommandRegistry.Register() validates and stores commands
3. **NEW:** After registration, inject stored commands into the completion tree
4. CLI reads completion tree for tab candidates

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | CommandDecl via declare-registration RPC | [x] existing path; `Hidden` already carried on `CommandDecl`/`CommandDef` |
| Engine → CLI completion | `command.Node` tree via `VisibleCommandEntries` → `MergeCommandPaths` | [x] SSH `session_factory.go` + web `web_completer.go` |

### Integration Points
- `command.Node` tree (internal/component/command/node.go) - must accept runtime additions
- `CommandRegistry` (internal/component/plugin/server/command_registry.go) - source of plugin commands
- CLI completion handler - reads the command tree

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | command.Node tree supports runtime insertion after BuildCommandTree | Code structure | Need a different injection point | Read BuildCommandTree | CONFIRMED — `command.MergeCommandPaths` (`node.go:110`) inserts into `Node.Children` after build; the completer offers name-only nodes (`completer.go:262-273` never reads `WireMethod`) |
| A-2 | Plugin registration completes before CLI completion is first needed | Startup order | Need lazy injection or rebuild | Trace startup sequence | WRONG (as feared) — `buildServices` (main.go:729) runs before `WaitForStartupComplete` (main.go:918). Resolved by rebuilding at use time: SSH per-session (`session_factory.go`), web live per-request overlay (`web_completer.go`), so registration/unregistration is always reflected |
| A-3 | CommandDecl.Args provides enough info for completion hints | rpc/types.go | Need richer arg metadata | Read existing completion code | N/A — completion is name-only by design (see Known Limitations); arg-value hints remain YANG/ValueHints only |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Plugin re-registration (hot reload) could leave stale completion entries | Completion shows commands from unloaded plugins | Clear and rebuild on re-registration |
| R-2 | Race between plugin startup and early CLI tab-press | Missing completion for just-registered plugin | Acceptable -- completion improves as plugins start |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registers CommandDecl | → | Command appears in completion tree | `TestCommandRegistryInjectsIntoCompletionTree` ✅ |
| Plugin registers CommandDecl with Hidden: true | → | Command absent from completion tree | `TestHiddenCommandExcludedFromInjectedTree` (+ existing `TestHiddenCommandExcludedFromCompletion` for the registry path) ✅ |
| User completes "show myplugin " over the web `/cli/complete` endpoint | → | Completes to the plugin subcommand | `TestCLICompleteOperationalIncludesPluginCommand` (end-to-end httptest) ✅ |
| Plugin unregisters (process exits) | → | Command gone from a rebuilt tree | `TestUnregisteredCommandRemovedFromCompletion` ✅ |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | Plugin registers a CommandDecl | Command appears in tab-completion | ✅ `TestCommandRegistryInjectsIntoCompletionTree` (SSH/tree) + `TestCLICompleteOperationalIncludesPluginCommand` (web e2e) + `TestPluginAwareCommandCompleterIsLive` |
| AC-2 | Plugin registers CommandDecl with Hidden: true | Command does NOT appear in tab-completion but works when typed | ✅ `TestHiddenCommandExcludedFromInjectedTree` (excluded from tree) + `Lookup` still finds it (dispatch) |
| AC-3 | Plugin unregisters (process exits) | Command disappears from completion tree | ✅ SSH: per-session rebuild reads current registry (`TestUnregisteredCommandRemovedFromCompletion`); web: live per-request overlay (`TestPluginAwareCommandCompleterIsLive` asserts unregister removes it) |
| AC-4 | All currently-missing commands | All appear in tab-completion (except any marked Hidden) | ✅ by construction — injection iterates `VisibleCommandEntries()` (every non-Hidden registered command), not a per-command allowlist |
| AC-5 | Existing YANG-backed commands | Continue to work exactly as before | ✅ non-destructive merge (`TestMergeCommandPathsNonDestructive`, `TestPluginAwareCommandCompleterYANGWins`); YANG tree never mutated on the web path |

## Files to Modify
- `pkg/plugin/rpc/types.go` - add Hidden field to CommandDecl
- `pkg/plugin/sdk/sdk_types.go` - re-export Hidden if needed
- `internal/component/plugin/server/command_registry.go` - store Hidden, expose method to list visible commands
- `internal/component/config/yang/command.go` - accept runtime command additions to Node tree (or new injection point)
- `internal/component/plugin/server/command.go` - inject registered commands into completion tree after plugin startup

## Files to Create
- TestCommandRegistryInjectsIntoCompletionTree test files

## Known Limitations
- Completion for plugin commands will be name-only (no YANG-level argument typing/validation unless CommandDecl.Args is rich enough)
- Plugin commands registered after initial CLI connection may not appear until next tab-press refresh

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMergeCommandPaths{InsertsNewCommand,CreatesIntermediateNodes,NonDestructive,SkipsEmptyAndNilRoot}` | `internal/component/command/node_merge_test.go` | injection primitive: insert, intermediate nodes, non-destructive, nil-safe | ✅ green |
| `TestCommandRegistryInjectsIntoCompletionTree` | `internal/component/plugin/server/command_registry_test.go` | AC-1: registered commands appear in tree | ✅ green |
| `TestHiddenCommandExcludedFromInjectedTree` | `internal/component/plugin/server/command_registry_test.go` | AC-2: Hidden suppresses tree injection | ✅ green |
| `TestUnregisteredCommandRemovedFromCompletion` | `internal/component/plugin/server/command_registry_test.go` | AC-3: cleanup on unregister | ✅ green |
| `TestPluginAwareCommandCompleterIsLive` | `cmd/ze/hub/web_completer_test.go` | web AC-1 + AC-3: live register AND unregister per request | ✅ green |
| `TestPluginAwareCommandCompleterYANGWins` | `cmd/ze/hub/web_completer_test.go` | AC-5: YANG description not shadowed by plugin, deduped | ✅ green |
| `TestMergePluginCommandsNilSafe` | `cmd/ze/hub/session_factory_test.go` | SSH merge is nil-safe during startup race (R-2) | ✅ green |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestCLICompleteOperationalIncludesPluginCommand` | `internal/component/web/cli_test.go` | A plugin command, merged into the tree, is returned by the real `/cli/complete` handler as JSON (real merge + real completer + production HTTP handler + JSON contract) | ✅ green |

> **Why a Go httptest, not a `.ci`:** the planned `test/plugin/plugin-command-completion.ci` is impractical — the `.ci` harness's built-in `http=` verb sends no auth (`internal/test/runner/runner_validate.go:578-606`), and `/cli/complete` requires an authenticated context, so a bare `http=get` cannot reach it. A `.ci` would need a full `cmd=foreground` Python driver that boots a daemon, configures a plugin, performs a web login for a session cookie, then GETs the endpoint. The Go httptest exercises the identical production handler + JSON contract the browser consumes, at a fraction of the cost.

### Interop Tests (MANDATORY for protocol features)
N/A -- not protocol work.

### Future (if deferring any tests)
- None planned

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- check what exists |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |

### Implementation Phases

1. **Phase: Wiring** -- add Hidden to CommandDecl, store in registry
   - Tests: TestHiddenCommandExcludedFromCompletion
   - Files: pkg/plugin/rpc/types.go, command_registry.go
   - Verify: Hidden field accepted and stored
2. **Phase: Tree injection** -- inject plugin commands into completion tree
   - Tests: TestCommandRegistryInjectsIntoCompletionTree
   - Files: command_registry.go, command.go or command.go integration point
   - Verify: registered commands appear in Node tree
3. **Phase: Cleanup** -- remove commands from tree on plugin exit
   - Tests: TestUnregisteredCommandRemovedFromCompletion
   - Files: command_registry.go
   - Verify: commands removed on process exit
4. **Functional tests** -- end-to-end tab-completion verification
5. **Full verification** -- `make ze-verify`

## Implementation Summary

The registry already carried `Hidden` (on `CommandDef` and `CommandDecl`) and a
`Complete()` method — so the "add Hidden" work was already shipped. The real gap
was that the **interactive** completion tree was built from YANG only and never
consulted the plugin registry. (Shell completion `ze completion words` runs in a
standalone CLI process with no daemon and cannot see live plugin commands; the
daemon's `system command complete` RPC already completed them via
`Registry().Complete()`.)

| Piece | Location |
|-------|----------|
| Hidden-filtered entry source | `command_registry.go` `VisibleCommandEntries()` |
| Non-destructive tree injection primitive | `command/node.go` `MergeCommandPaths` + `CommandEntry` |
| SSH: eager per-session merge | `session_factory.go` `mergePluginCommands` (via `params.APIServer().Dispatcher().Registry()`) |
| Web: live per-request overlay | `web_completer.go` `pluginAwareCommandCompleter`, sourced from `main.go` → `ServiceDeps.WebCommands` → `startWebServer` |
| Architecture doc | `docs/architecture/api/commands.md` § Plugin Command Completion |

**Documentation Update Checklist:** architecture doc updated (`commands.md`
§ Plugin Command Completion, with `<!-- source: -->` reverse-index anchors). No
config-surface / YANG / env-var change (behavior is internal completion wiring),
so `config-reference.md` and guide docs need no change. `Hidden` opt-out already
documented under `docs/plugin-development/commands.md`.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (adversarial review over the complete diff)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Web httptest uses `cli.NewCommandCompleter` directly, not the production wrapper (structural: the wrapper lives in `cmd/ze/hub` (ze_web) and can't be imported from `internal/component/web` by import direction) | `internal/component/web/cli_test.go` | acknowledged — the wrapper is covered by `TestPluginAwareCommandCompleter*` in `cmd/ze/hub` |
| 2 | NOTE | Original `sync.Once` web design snapshotted the registry at first request, so unregister was not reflected for web until restart (AC-3 gap) | `cmd/ze/hub/web_completer.go` | **fixed** — replaced with a live per-request overlay (`pluginAwareCommandCompleter`); AC-3 now met for web (`TestPluginAwareCommandCompleterIsLive`) |
| 3 | NOTE | `adminChildren` admin-nav is built from the YANG-only tree at setup, so plugin commands don't appear in admin nav | `cmd/ze/hub/service_web.go:493-502` | acknowledged — pre-existing, out of this spec's tab-completion scope, not regressed |

Review confirmed: `MergeCommandPaths` non-destructive (no clobber/panic/mis-insert); `VisibleCommandEntries` correctly excludes Hidden; no data races (SSH tree is genuinely per-session — `BuildCommandTree` allocates fresh nodes each call; web YANG tree is never mutated); wiring fully reachable (no dead code); builtin dispatch precedence preserved; tests are genuine end-to-end, not tautological.

### Fixes applied
- Replaced the web `sync.Once` snapshot completer with `pluginAwareCommandCompleter`, which overlays plugin commands live on every `/cli/complete` request (immutable YANG tree + throwaway per-request overlay). This closes NOTE 2 (AC-3 for web) and matches the spec's R-1 mitigation ("clear and rebuild on re-registration").

### Final status
- [x] Adversarial review shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (NOTE 2 fixed; NOTE 1 and NOTE 3 acknowledged as structural / pre-existing)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-command-completion.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-command-completion.md`
