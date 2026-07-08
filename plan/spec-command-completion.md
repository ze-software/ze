# Spec: command-completion

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-13 |

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
| Plugin → Engine | CommandDecl via declare-registration RPC | [ ] |
| Engine → CLI completion | command.Node tree | [ ] |

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
| A-1 | command.Node tree supports runtime insertion after BuildCommandTree | Code structure | Need a different injection point | Read BuildCommandTree | unvalidated |
| A-2 | Plugin registration completes before CLI completion is first needed | Startup order | Need lazy injection or rebuild | Trace startup sequence | unvalidated |
| A-3 | CommandDecl.Args provides enough info for completion hints | rpc/types.go | Need richer arg metadata | Read existing completion code | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Plugin re-registration (hot reload) could leave stale completion entries | Completion shows commands from unloaded plugins | Clear and rebuild on re-registration |
| R-2 | Race between plugin startup and early CLI tab-press | Missing completion for just-registered plugin | Acceptable -- completion improves as plugins start |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registers CommandDecl | → | Command appears in completion tree | TestCommandRegistryInjectsIntoCompletionTree |
| Plugin registers CommandDecl with Hidden: true | → | Command absent from completion tree | TestHiddenCommandExcludedFromCompletion |
| User presses tab after "show bgp ir" | → | Completes to "show bgp irr" | TestPluginCommandCompletion |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers a CommandDecl | Command appears in tab-completion |
| AC-2 | Plugin registers CommandDecl with Hidden: true | Command does NOT appear in tab-completion but works when typed |
| AC-3 | Plugin unregisters (process exits) | Command disappears from completion tree |
| AC-4 | All 51 currently-missing commands | All appear in tab-completion (except any marked Hidden) |
| AC-5 | Existing YANG-backed commands | Continue to work exactly as before |

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
| `TestCommandRegistryInjectsIntoCompletionTree` | `internal/component/plugin/server/command_registry_test.go` | AC-1: registered commands appear in tree | |
| `TestHiddenCommandExcludedFromCompletion` | `internal/component/plugin/server/command_registry_test.go` | AC-2: Hidden suppresses completion | |
| `TestUnregisteredCommandRemovedFromCompletion` | `internal/component/plugin/server/command_registry_test.go` | AC-3: cleanup on unregister | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-command-completion` | `test/plugin/plugin-command-completion.ci` | Tab-completion returns plugin commands | |

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

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
