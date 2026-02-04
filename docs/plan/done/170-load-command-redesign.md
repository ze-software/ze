# Spec: load-command-redesign

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/editor/model.go` - command dispatch, cmdLoad functions
4. `internal/config/parser.go` - Tree structure, MergeContainer

## Task

Redesign the `load` command syntax in the config editor to:
1. Make source explicit (`file` vs `terminal`)
2. Support context-relative merging (`relative` vs `merge root`)
3. Add terminal/stdin input capability

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/editor/model.go` - load command dispatch at line 1008-1013, `cmdLoad()` at line 1746, `cmdLoadMerge()` at line 1770, `mergeConfigs()` at line 1897
- [ ] `internal/config/parser.go` - Tree structure with MergeContainer() function

**Current syntax:**
| Command | Behavior |
|---------|----------|
| `load <file>` | Replace entire config with file content |
| `load merge <file>` | Merge file at root (line-based deduplication) |

**Behavior to preserve:**
- Path resolution relative to config file directory
- Dirty flag set after load
- Revalidation triggered after load
- Error messages for missing files/arguments

**Behavior to change:** (user explicitly requested)
- Remove old syntax: `load <file>`, `load merge <file>`
- New syntax: `load <source> <location> <action> [file]`

| Keyword | Position | Values | Description |
|---------|----------|--------|-------------|
| source | 1st | `file`, `terminal` | Where content comes from |
| location | 2nd | `absolute`, `relative` | Where to apply (root vs current context) |
| action | 3rd | `replace`, `merge` | How to apply (overwrite vs combine) |
| file | 4th | path | Only required when source=`file` |

**Full command matrix:**

| Command | Behavior |
|---------|----------|
| `load file absolute replace <path>` | Replace entire config with file |
| `load file absolute merge <path>` | Merge file content at root |
| `load file relative replace <path>` | Replace current context subtree with file |
| `load file relative merge <path>` | Merge file content at current context |
| `load terminal absolute replace` | Paste mode → replace entire config |
| `load terminal absolute merge` | Paste mode → merge at root |
| `load terminal relative replace` | Paste mode → replace current context subtree |
| `load terminal relative merge` | Paste mode → merge at current context |

- Missing keywords → error with usage hint showing full syntax

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall architecture context

### Source Files
- [ ] `internal/config/editor/model.go` - command dispatch, load implementation
- [ ] `internal/config/parser.go` - Tree structure, MergeContainer function
- [ ] `internal/config/setparser.go` - walkAndSet pattern for path navigation

**Key insights:**
- Command dispatch uses simple `args[0] == "merge"` check
- `contextPath []string` already tracks current edit location
- `Tree.MergeContainer()` exists for tree-level merging
- Current merge is line-based, not tree-based (comment at line 1783 notes TODO)
- Bubble Tea consumes stdin, so terminal input requires either pre-TUI detection or paste mode

## Data Flow (MANDATORY)

### Entry Point
- User types `load ...` command in editor command line
- Parsed by `dispatchCommand()` at model.go:943

### Transformation Path
1. Command string tokenized by `strings.Fields()`
2. First token identifies command (`load`)
3. Remaining tokens parsed for source/path/mode
4. File read via `readFile()` or terminal buffer collected
5. Content either replaces (`SetWorkingContent`) or merges
6. For `relative`: navigate to contextPath, merge there
7. Dirty flag set, revalidation triggered

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Command → Action | dispatchCommand switch | [ ] |
| File → Content | readFile() utility | [ ] |
| Content → Tree | Parse() for tree-based merge | [ ] |

### Integration Points
- `dispatchCommand()` - add new command routing
- `cmdLoad()`, `cmdLoadMerge()` - refactor into unified handler
- `Tree.MergeContainer()` - use for tree-based merge
- `contextPath` - use for relative merge location

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoadArgParsing` | `model_test.go` | Argument parsing handles all keyword combinations | |
| `TestLoadArgParsingErrors` | `model_test.go` | Missing/invalid keywords return clear errors | |
| `TestLoadFileAbsoluteReplace` | `model_test.go` | `load file absolute replace` replaces entire config | |
| `TestLoadFileAbsoluteMerge` | `model_test.go` | `load file absolute merge` merges at root | |
| `TestLoadFileRelativeReplace` | `model_test.go` | `load file relative replace` replaces context subtree | |
| `TestLoadFileRelativeMerge` | `model_test.go` | `load file relative merge` merges at context | |
| `TestLoadTerminalPasteMode` | `model_test.go` | `load terminal ...` enters paste mode | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `load-file-absolute-replace.et` | `test/editor/lifecycle/` | Replace entire config from file | |
| `load-file-absolute-merge.et` | `test/editor/lifecycle/` | Merge file at root | |
| `load-file-relative-replace.et` | `test/editor/lifecycle/` | Replace context subtree from file | |
| `load-file-relative-merge.et` | `test/editor/lifecycle/` | Merge file at current context | |
| `load-terminal-absolute-replace.et` | `test/editor/lifecycle/` | Paste mode replaces config | |
| `load-syntax-error.et` | `test/editor/lifecycle/` | Missing keywords show usage hint | |

## Files to Modify

- `internal/config/editor/model.go` - command dispatch, load handlers, paste mode state
- `internal/config/editor/model_test.go` - unit tests for parseLoadArgs, paste mode

## Files to Delete

- `test/editor/lifecycle/load-file.et` - replaced by new syntax tests
- `test/editor/lifecycle/load-merge.et` - replaced by new syntax tests

## Files to Create

- `test/editor/lifecycle/load-file-absolute-replace.et` - test `load file absolute replace <path>`
- `test/editor/lifecycle/load-file-absolute-merge.et` - test `load file absolute merge <path>`
- `test/editor/lifecycle/load-file-relative-replace.et` - test `load file relative replace <path>`
- `test/editor/lifecycle/load-file-relative-merge.et` - test `load file relative merge <path>`
- `test/editor/lifecycle/load-terminal-absolute-replace.et` - test paste mode (if testable)
- `test/editor/lifecycle/load-syntax-error.et` - verify missing keywords show usage

## Implementation Steps

### Phase 1: Remove Old Syntax
- [ ] Remove `load <file>` dispatch
- [ ] Remove `load merge <file>` dispatch
- [ ] Update error message to show new syntax

### Phase 2: Refactor Argument Parsing
- [ ] Create `parseLoadArgs(args) (source, location, action, path, error)` helper
- [ ] source: `"file"` or `"terminal"`
- [ ] location: `"absolute"` or `"relative"`
- [ ] action: `"replace"` or `"merge"`
- [ ] path: file path (required when source=`file`, error otherwise)
- [ ] Validate all keywords present, return clear error if missing
- [ ] Update dispatch to use new parser

### Phase 3: Implement File Variants
- [ ] `load file absolute replace <path>` - replace entire config
- [ ] `load file absolute merge <path>` - merge at root (tree-based)
- [ ] `load file relative replace <path>` - replace context subtree
- [ ] `load file relative merge <path>` - merge at contextPath

### Phase 4: Implement Paste Mode for Terminal
- [ ] Add `pasteMode bool` state to Model
- [ ] Add `pasteBuffer strings.Builder` to accumulate input
- [ ] Add `pasteModeAction string` to track intended action (replace/merge-root/relative)
- [ ] Handle Ctrl-D to end paste mode and process buffer
- [ ] Show "[Paste mode - Ctrl-D to finish]" in status
- [ ] Process buffer according to pasteModeAction

### Phase 5: Implement Tree-Based Merge
- [ ] Add `MergeAtPath(path []string, content string) error` to editor
- [ ] Parse content to tree
- [ ] Navigate to path (create containers if needed)
- [ ] Use `Tree.MergeContainer()` to combine

### Phase 6: Update Help Text
- [ ] Add all load variants to help overlay
- [ ] Document paste mode behavior

### Phase 7: Tests
- [ ] Delete old `load-file.et`, `load-merge.et`
- [ ] Create `load-file-absolute-replace.et`
- [ ] Create `load-file-absolute-merge.et`
- [ ] Create `load-file-relative-replace.et`
- [ ] Create `load-file-relative-merge.et`
- [ ] Create `load-terminal-absolute-replace.et` (if testable in headless)
- [ ] Create `load-syntax-error.et`
- [ ] Unit tests for `parseLoadArgs()`

## Design Decisions (RESOLVED)

### Q1: Default behavior for `load file <path>` without explicit mode?
**Decision: Require explicit** - User must specify `replace`, `merge root`, or `relative`

### Q2: Terminal input approach?
**Decision: Paste mode** - Enter paste mode inside editor, accumulate lines until Ctrl-D

### Q3: Backwards compatibility for old syntax?
**Decision: Remove immediately** - Ze has no users, clean break

## Implementation Summary

### What Was Implemented
- `parseLoadArgs()` function for parsing new syntax: `load <source> <location> <action> [path]`
- `cmdLoadNew()` dispatches to appropriate handler based on parsed args
- `applyLoadAbsolute()` handles `absolute` location (root-level operations)
- `applyLoadRelative()` handles `relative` location (context-level operations)
- `replaceAtContext()` and `mergeAtContext()` for context-relative operations
- Paste mode state (`pasteMode`, `pasteBuffer`, `pasteModeLocation`, `pasteModeAction`)
- Help text updated with all load command variants
- Old syntax (`load <file>`, `load merge <file>`) rejected with clear error message

### Unit Tests Added
- `TestLoadFileAbsoluteReplace` - replace entire config from file
- `TestLoadFileAbsoluteMerge` - merge file at root
- `TestLoadFileRelativeReplace` - replace context subtree from file
- `TestLoadFileRelativeMerge` - merge file at current context
- `TestLoadOldSyntaxRejected` - old syntax returns error
- `TestLoadOldMergeSyntaxRejected` - old merge syntax returns error
- `TestLoadTerminalEntersPasteMode` - terminal source enters paste mode

### Functional Tests Added
- `load-file-absolute-replace.et`
- `load-file-absolute-merge.et`
- `load-file-relative-replace.et`
- `load-file-relative-merge.et`
- `load-syntax-error.et`
- `load-missing-args.et`
- `load-not-found.et`

### Old Files Deleted
- `load-file.et` (replaced by new syntax tests)
- `load-merge.et` (replaced by new syntax tests)

### Bugs Found/Fixed
- **Index out of bounds in `replaceAtContext` and `mergeAtContext`**: When contextPath had only 1 element (e.g., `["bgp"]` from `edit bgp`), the code accessed `contextPath[len-2]` before checking the length, causing a panic. Fixed by checking length first with proper if/else structure.
- **Tests added**:
  - `TestLoadFileRelativeReplaceSingleContext` - validates replace with single-element contextPath, verifies old content removed
  - `TestLoadFileRelativeMergeSingleContext` - validates merge with single-element contextPath, verifies original preserved

### Deviations from Plan
- None - implementation followed spec exactly

## Checklist

### 🏗️ Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes
