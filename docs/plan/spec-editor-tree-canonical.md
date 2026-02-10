# Spec: editor-tree-canonical

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - config file format
4. `internal/config/editor/model.go` - editor model struct + Update/Init (681 lines)
4b. `internal/config/editor/model_commands.go` - command handlers: cmdEdit, cmdSet, cmdDelete, cmdShow (486 lines)
5. `internal/config/editor/editor.go` - Editor struct, file I/O
6. `internal/config/parser.go` - Tree struct, all tree methods
7. `internal/config/serialize.go` - Tree → config text serializer

## Task

Make `config.Tree` the canonical in-memory representation for the configuration editor. Currently the editor treats raw text (`workingContent string`) as the source of truth and uses ad-hoc brace-counting to navigate, filter, and modify config. This spec replaces that with tree-based operations.

**This is Part 1 of a series:**
- **Part 1 (this spec):** Foundation — tree as source of truth, tree-based navigation, tree-based set/delete
- **Part 2 (future):** Subtree serialization — `show` displays via `Serialize(subtree)` instead of text filtering
- **Part 3 (future):** Load/merge via tree operations — `load` parses into tree and merges, replacing text surgery

Part 1 is self-contained: it eliminates the text-surgery functions and brace-counting navigation, replacing them with tree walks. Display temporarily still uses full-tree `Serialize()` (the viewport shows the full serialized config, not a filtered subsection). Parts 2-3 can follow independently.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config file format, section types, how JUNOS-style syntax works
- [ ] `docs/architecture/core-design.md` - overall system architecture (editor is a peripheral tool)

### Source Files (MUST read before implementation)
- [ ] `internal/config/parser.go` - Tree struct definition, all methods (Get, Set, GetContainer, GetList, RemoveContainer, RemoveListEntry, etc.)
- [ ] `internal/config/serialize.go` - Serialize() function, round-trip tested
- [ ] `internal/config/serialize_test.go` - round-trip tests proving Serialize works on real configs
- [ ] `internal/config/schema.go` - Schema struct, ContainerNode, ListNode, navigation via Lookup()
- [ ] `internal/config/editor/editor.go` - Editor struct (originalContent, workingContent, tree, dirty, save/discard)
- [ ] `internal/config/editor/model.go` - Model struct, Update, Init, key handling (681 lines)
- [ ] `internal/config/editor/model_commands.go` - command handlers: cmdEdit, cmdTop, cmdUp, cmdSet, cmdDelete, cmdShow, dispatchCommand (486 lines)
- [ ] `internal/config/editor/model_load.go` - cmdLoad, cmdCommitConfirm, merge/pipe functions (710 lines)
- [ ] `internal/config/editor/model_render.go` - View, highlight, render, overlay functions (498 lines)
- [ ] `internal/config/editor/completer.go` - Completer, already uses YANG schema
- [ ] `internal/config/editor/validator.go` - ConfigValidator, already parses tree for validation

**Key insights:**
- `config.Tree` already has all needed mutation methods: `Set`, `GetContainer`, `GetOrCreateContainer`, `RemoveContainer`, `AddListEntry`, `RemoveListEntry`, `ClearList`, `MergeContainer`
- Missing: `Delete(name)` for leaf values — needs to be added to Tree
- `Serialize(tree, schema)` already round-trips through 80+ real config files (TestRoundtripConfigFiles)
- The editor already holds a `tree *config.Tree` field but only uses it for completions
- `Schema.Lookup(path)` can navigate to sub-schema nodes for future subtree serialization

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/config/editor/editor.go` - Editor holds both `workingContent string` (raw text, source of truth) and `tree *config.Tree` (parsed once at startup, used only for completions). All mutations go through `SetWorkingContent()` which modifies the raw text. The tree is never updated after startup.
- [ ] `internal/config/editor/model_commands.go` - Navigation uses text-based functions:
- [ ] `internal/config/parser.go` - Tree struct definition with Get/Set/GetContainer/GetList/RemoveContainer/RemoveListEntry methods
- [ ] `internal/config/serialize.go` - Serialize() round-trips through 80+ config files
- [ ] `internal/config/schema.go` - Schema with ContainerNode/ListNode, Lookup() for path navigation
- [ ] `internal/config/editor/completer.go` - Already tree-based, uses YANG schema
- [ ] `internal/config/editor/validator.go` - Already parses tree for validation

**Current behavior details:**
- Text-based navigation functions in model_commands.go and model_load.go:
  - `findFullContextPath()` — walks raw text counting `{`/`}` to locate blocks (70 lines)
  - `filterContentByContextPath()` — walks raw text counting braces to extract subsection (90 lines)
  - `findParentOfKeyword()` — walks raw text counting braces (50 lines)
  - `setValueInConfig()` — text surgery to insert/replace key-value pairs (80 lines)
  - `replaceAtContext()` / `mergeAtContext()` / `mergeConfigs()` — text surgery for load commands
- Command handlers in model_commands.go:
  - `cmdSet()` builds full path from contextPath + args, calls `setValueInConfig()` on raw text
  - `cmdDelete()` is a stub — marks dirty but does not actually delete anything
  - `cmdEdit()` calls `findFullContextPath()` to locate block, then `filterContentByContextPath()` to extract content for viewport
  - `cmdShow()` calls `filterContentByContext()` to extract current section from raw text
  - `cmdTop()` / `cmdUp()` set contextPath and call `filterContentByContextPath()` for display
  - `cmdCommit()` validates via `validator.Validate(workingContent)` which re-parses the text into a tree
  - `runValidation()` calls `validator.Validate(editor.WorkingContent())` — re-parses text each time

**Behavior to preserve:**
- All editor commands: set, delete, edit, show, compare, commit, discard, top, up, history, rollback, exit, help, load, errors
- Context path display in prompt: `ze[bgp peer 1.1.1.1]#`
- Tab completion (already tree-based via Completer)
- YANG-based validation (already tree-based via ConfigValidator)
- Backup/rollback mechanism (file-based, independent of in-memory representation)
- Pipe filters on show command (grep, head, tail)
- Template editing with wildcard (`edit peer *`)
- Paste mode for `load terminal` commands
- Commit confirm timer

**Behavior to change (user explicitly requested):**
- Raw text (`workingContent`) is no longer the source of truth — tree is
- All mutations (set, delete, load) operate on the tree directly
- Navigation (edit, up, top) resolves against the tree, not raw text
- Display uses `Serialize(tree, schema)` for full config view
- Validation operates on the tree directly (no re-parse needed)

## Data Flow (MANDATORY)

### Entry Point
- User types a command (set, delete, edit, show, load, etc.) in the TUI
- Command is tokenized and dispatched by `dispatchCommand()`

### Transformation Path (current → proposed)

**Current (text-first):**
1. Command parsed → full path built from contextPath + args
2. Raw text modified via text surgery (`setValueInConfig`)
3. Modified text stored as `workingContent`
4. Display: raw text filtered by brace-counting → viewport
5. Validation: raw text re-parsed into tree → validate tree

**Proposed (tree-first):**
1. Command parsed → full path built from contextPath + args
2. Tree navigated to target node via `GetContainer`/`GetList`
3. Tree mutated directly (`Set`, `RemoveContainer`, etc.)
4. Display: `Serialize(tree, schema)` → viewport (full config initially; subtree in Part 2)
5. Validation: validate tree directly (already parsed, no re-parse)
6. Save: `Serialize(tree, schema)` → write to disk

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| User input → Tree | Command dispatch → tree walk → mutation | [ ] |
| Tree → Display | `Serialize(tree, schema)` → viewport content | [ ] |
| Tree → Disk | `Serialize(tree, schema)` → `os.WriteFile` | [ ] |
| Disk → Tree | `os.ReadFile` → `parser.Parse()` → tree | [ ] |

### Integration Points
- `config.Parser.Parse()` — creates tree from config text (used on load from disk)
- `config.Serialize()` — creates config text from tree (used for display and save)
- `config.Tree` methods — all mutations go through existing API
- `Completer.SetTree()` — already accepts tree, needs to be called after mutations
- `ConfigValidator.ValidateWithYANG()` — already accepts tree directly

### Architectural Verification
- [ ] No bypassed layers (all mutations go through Tree API)
- [ ] No unintended coupling (Editor depends on config.Tree, not raw text)
- [ ] No duplicated functionality (replaces text surgery with tree methods)
- [ ] Display uses established Serialize() infrastructure

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTreeDeleteValue` | `internal/config/parser_test.go` | New `Delete(name)` method removes leaf value | |
| `TestTreeDeleteValueOrder` | `internal/config/parser_test.go` | `Delete` also removes key from `valuesOrder` | |
| `TestTreeDeleteNonexistent` | `internal/config/parser_test.go` | `Delete` on missing key is a no-op | |
| `TestEditorTreeNavigation` | `internal/config/editor/editor_test.go` | `WalkPath()` navigates tree via GetContainer/GetList | |
| `TestEditorTreeNavigationListKey` | `internal/config/editor/editor_test.go` | `WalkPath(["bgp","peer","1.1.1.1"])` finds peer entry | |
| `TestEditorTreeNavigationMissing` | `internal/config/editor/editor_test.go` | `WalkPath` returns nil for nonexistent path | |
| `TestEditorTreeSet` | `internal/config/editor/editor_test.go` | Setting a value mutates tree and marks dirty | |
| `TestEditorTreeSetNewKey` | `internal/config/editor/editor_test.go` | Setting a new key creates it in the tree | |
| `TestEditorTreeDelete` | `internal/config/editor/editor_test.go` | Deleting a value removes it from tree | |
| `TestEditorTreeDeleteContainer` | `internal/config/editor/editor_test.go` | Deleting a container removes the block | |
| `TestEditorTreeDeleteListEntry` | `internal/config/editor/editor_test.go` | Deleting a list entry (e.g., peer) removes it | |
| `TestEditorSerializeRoundtrip` | `internal/config/editor/editor_test.go` | After set, `Serialize(tree)` contains the new value | |
| `TestEditorContentAfterSet` | `internal/config/editor/editor_test.go` | `WorkingContent()` returns serialized tree, not stale text | |
| `TestEditorSaveSerialized` | `internal/config/editor/editor_test.go` | `Save()` writes `Serialize(tree)` to disk | |
| `TestModelCmdEditTreeBased` | `internal/config/editor/model_test.go` | `edit bgp` navigates via tree, sets contextPath | |
| `TestModelCmdEditPeerTreeBased` | `internal/config/editor/model_test.go` | `edit peer 1.1.1.1` navigates via tree through list | |
| `TestModelCmdEditNotFound` | `internal/config/editor/model_test.go` | `edit nonexistent` returns error from tree lookup | |
| `TestModelCmdSetTreeBased` | `internal/config/editor/model_test.go` | `set hold-time 90` mutates tree at context path | |
| `TestModelCmdDeleteTreeBased` | `internal/config/editor/model_test.go` | `delete hold-time` removes value from tree | |
| `TestModelCmdUpTreeBased` | `internal/config/editor/model_test.go` | `up` pops context, display updates | |
| `TestModelCmdTopTreeBased` | `internal/config/editor/model_test.go` | `top` clears context, shows full serialized config | |
| `TestModelCompleterUpdatedAfterSet` | `internal/config/editor/model_test.go` | Completer.SetTree() called after tree mutation | |
| `TestModelValidationAfterSet` | `internal/config/editor/model_test.go` | Validation runs on tree directly after mutation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric fields introduced | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-editor-tree-set` | `test/editor/tree-set.ci` | User sets a value, shows config, value appears in serialized output | |
| `test-editor-tree-delete` | `test/editor/tree-delete.ci` | User deletes a value, shows config, value is gone | |
| `test-editor-tree-navigate` | `test/editor/tree-navigate.ci` | User edits into a block, sets value, goes up, shows full config | |

### Future (if deferring any tests)
- Subtree serialization tests (Part 2)
- Load/merge via tree tests (Part 3)

## Files to Modify

- `internal/config/parser.go` - Add `Delete(name)` method to Tree (removes leaf value + valuesOrder entry)
- `internal/config/editor/editor.go` - Replace `workingContent string` with tree-canonical model:
  - `WorkingContent()` returns `Serialize(tree, schema)` instead of stored string
  - `SetWorkingContent()` parses text into tree (for backward compat during transition)
  - Add `WalkPath(path []string) *config.Tree` for tree-based navigation
  - Add `SetValue(path []string, key, value string)` for tree-based mutation
  - Add `DeleteValue(path []string, key string)` for tree-based deletion
  - Add `DeleteContainer(path []string, name string)` for block deletion
  - Add `DeleteListEntry(path []string, listName, key string)` for list entry deletion
  - Store `schema *config.Schema` for serialization
  - `Save()` uses `Serialize(tree, schema)` to produce file content
  - `Discard()` re-parses original content into tree
- `internal/config/editor/model_commands.go` - Replace text-based commands with tree-based:
  - `cmdEdit()` — navigate tree via `WalkPath`, not `findFullContextPath`
  - `cmdUp()` — pop contextPath, no brace-counting needed
  - `cmdTop()` — clear contextPath, serialize full tree for display
  - `cmdSet()` — call `editor.SetValue()`, not `setValueInConfig()`
  - `cmdDelete()` — call `editor.DeleteValue()` (currently a stub)
  - `cmdShow()` — serialize full tree for display (subtree filtering deferred to Part 2)
  - `cmdCommit()` — validate tree directly, serialize for save
  - `cmdDiscard()` — call `editor.Discard()`, re-serialize for display
  - `runValidation()` — validate tree directly instead of re-parsing text
  - `showConfigContent()` — serialize tree for display
  - Delete from model_commands.go: `findFullContextPath()`, `findParentOfKeyword()`, `setValueInConfig()`, `setRootLevelValue()`, `extractKeyFromLine()`, `formatBlockPattern()`, `formatKeyValue()`
  - Delete from model_render.go: `filterContentByContextPath()`
- `internal/config/editor/completer.go` - No changes (already tree-based)
- `internal/config/editor/validator.go` - `Validate()` accepts `*config.Tree` directly instead of string (or add `ValidateTree()` method)

## Files to Create

- None — all changes are modifications to existing files

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Add `Delete(name)` to Tree** - Add method to remove a leaf value and its insertion-order entry
   → **Review:** Does it handle missing keys? Does it update valuesOrder?

2. **Write unit tests for Tree.Delete** - Test delete existing, delete missing, order preservation
   → **Review:** Boundary cases covered?

3. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason?

4. **Implement Tree.Delete** - Minimal code
   → **Review:** Simple, no side effects?

5. **Run tests** - Verify PASS (paste output)
   → **Review:** All pass?

6. **Write unit tests for Editor tree operations** - WalkPath, SetValue, DeleteValue
   → **Review:** Edge cases: empty path, missing container, list navigation

7. **Run tests** - Verify FAIL
   → **Review:** Right failure reasons?

8. **Modify Editor struct** - Add schema field, WalkPath, SetValue, DeleteValue methods. Change WorkingContent() to return Serialize(). Change Save() to use Serialize(). Change Discard() to re-parse original.
   → **Review:** Is tree always kept in sync? Is schema available?

9. **Run tests** - Verify PASS
   → **Review:** All editor tests pass including existing ones?

10. **Write unit tests for Model tree-based commands** - cmdEdit, cmdSet, cmdDelete, cmdUp, cmdTop using tree navigation
    → **Review:** Do tests cover the navigation logic?

11. **Run tests** - Verify FAIL
    → **Review:** Right failure reasons?

12. **Modify Model command handlers** - Replace text-based implementations with tree-based ones
    → **Review:** All text surgery functions removed? No orphaned code?

13. **Delete dead code** - Remove findFullContextPath, findParentOfKeyword, filterContentByContextPath, setValueInConfig, setRootLevelValue, extractKeyFromLine, formatBlockPattern, formatKeyValue and their tests
    → **Review:** Nothing else depends on deleted functions?

14. **Run full suite** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests pass?

15. **Final self-review** - Re-read all changes, check for unused code, debug statements, edge cases

## Scope Limitations (Part 1 only)

These are explicitly deferred to later specs:

| Deferred | Why | Which Part |
|----------|-----|------------|
| Subtree serialization for `show` in context | Requires schema path navigation for sub-schema | Part 2 |
| `show` pipe filters on subtree | Depends on subtree serialization | Part 2 |
| `load file/terminal` via tree merge | Requires tree-level merge (MergeContainer exists but load commands need rework) | Part 3 |
| `compare` via tree diff | Currently string diff; tree diff is a separate feature | Part 3 |
| Template editing (`edit peer *`) | Complex interaction with tree navigation — needs separate design | Part 2 or 3 |
| Line-number-based error highlighting | With tree-canonical, line numbers come from serialized output — need to map back | Part 2 |
| `replaceAtContext()` / `mergeAtContext()` / `mergeConfigs()` | Text surgery for relative load — needs tree-based equivalent | Part 3 |

For Part 1, these commands use simplified behavior:
- `show` in context: shows full serialized config (not filtered to context subsection)
- `load`: continues to use existing text-based approach temporarily (parses text into tree after load)
- `compare`: compares serialized current tree vs original content string
- Template editing: deferred, returns "not yet supported in tree mode" if attempted

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Design Insights
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Tree.Delete(name) method | | | |
| Editor.WalkPath() for tree navigation | | | |
| Editor.SetValue() for tree mutation | | | |
| Editor.DeleteValue() for tree deletion | | | |
| WorkingContent() returns Serialize() | | | |
| Save() uses Serialize() | | | |
| cmdEdit uses tree navigation | | | |
| cmdSet uses tree mutation | | | |
| cmdDelete uses tree deletion (no longer stub) | | | |
| cmdUp/cmdTop use tree, no brace-counting | | | |
| Validation on tree directly | | | |
| Completer updated after mutations | | | |
| Dead text-surgery code removed | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestTreeDeleteValue | | | |
| TestTreeDeleteValueOrder | | | |
| TestTreeDeleteNonexistent | | | |
| TestEditorTreeNavigation | | | |
| TestEditorTreeSet | | | |
| TestEditorTreeDelete | | | |
| TestEditorSerializeRoundtrip | | | |
| TestModelCmdEditTreeBased | | | |
| TestModelCmdSetTreeBased | | | |
| TestModelCmdDeleteTreeBased | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/config/parser.go` | | |
| `internal/config/editor/editor.go` | | |
| `internal/config/editor/model.go` | | |
| `internal/config/editor/validator.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed (all items have status + location)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
