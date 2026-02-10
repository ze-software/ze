# Spec: file-splitting

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/editor/model.go` - primary split target
4. `internal/config/bgp.go` - secondary split target

## Task

Split oversized files in `internal/config/` to reduce context loading cost. The `internal/config/` tree contains 32,544 lines across ~50 files. Several files exceed 2000 lines, making targeted reads expensive. This is a pure mechanical refactor — no behavior changes.

**Motivation:** The ongoing `spec-editor-tree-canonical` work requires reading `model.go` (2340 lines) and `bgp.go` (3093 lines) repeatedly. Splitting by concern lets us read only the ~500-line slice we need.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - not directly relevant but understand package boundaries

### Source Files (READ before splitting)
- [ ] `internal/config/editor/model.go` - 2340 lines, primary split target
- [ ] `internal/config/editor/model_test.go` - 2213 lines, test split target
- [ ] `internal/config/bgp.go` - 3093 lines, secondary split target
- [ ] `internal/config/bgp_test.go` - 3538 lines, test split target

**Key insights:**
- Go compiles all files in a package together — splitting has zero semantic effect
- Each new file needs its own import block; `goimports` (via auto-linter hook) cleans unused imports
- File-local types must move with the functions that use them

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/editor/model.go` - 2340-line Bubble Tea model with rendering, commands, load/merge, and pipe logic
- [ ] `internal/config/editor/model_test.go` - 2213 lines, 57 test functions covering all model behavior
- [ ] `internal/config/bgp.go` - 3093-line BGP config with types, peer parsing, route extraction, NLRI parsers, utilities
- [ ] `internal/config/bgp_test.go` - 3538 lines, 88 test functions covering all BGP config behavior

**Behavior to preserve:** ALL existing behavior. This is purely moving code between files in the same package.

**Behavior to change:** None — pure file reorganization.

## Data Flow (MANDATORY)

Not applicable — no data flow changes. Files are reorganized within existing packages.

### Entry Point
- No new entry points. All existing entry points (NewModel, TreeToConfig, etc.) remain in their respective packages unchanged.

### Transformation Path
- No transformations change. Code moves between files in the same Go package, which is a single compilation unit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | No boundaries crossed — same package | [x] |

### Integration Points
- All existing integration points preserved — same package, same exported API.

### Architectural Verification
- [x] No bypassed layers — same package, same compilation unit
- [x] No unintended coupling — splitting reduces coupling by organizing by concern
- [x] No duplicated functionality — pure move
- [x] Zero-copy preserved — no encoding changes

## Split Plan

### 1. `editor/model.go` (2340 lines → 4 files)

| New File | Lines (approx) | Content |
|----------|------:|---------|
| **model.go** | ~750 | Model struct, styles, NewModel, Init, Update, key handlers, completions, public accessors |
| **model_render.go** | ~490 | setViewportData, setViewportText, highlightValidationIssues, View, overlayDropdown, placeOverlay, overlayLine, truncateAtWidth, skipWidth, renderDropdownBox, contextLabel, renderHelpOverlay, buildPrompt, renderInputWithGhost |
| **model_commands.go** | ~470 | executeCommand, dispatchCommand, cmdTop, cmdUp, cmdEdit, showConfigContent, cmdShow, cmdHistory, cmdRollback, cmdSet, cmdDelete, runValidation, scheduleValidation, cmdCommit, cmdDiscard, cmdErrors, tokenizeCommand, handleQuoteChar, joinTokensWithQuotes |
| **model_load.go** | ~630 | cmdCommitConfirm, cmdConfirm, cmdAbort, cmdLoad, cmdLoadMerge, resolveConfigPath, parseLoadArgs, cmdLoadNew, applyLoadAbsolute, applyLoadRelative, replaceAtContext, mergeAtContext, cmdShowPipe, applyPipeFilter, mergeConfigs, extractConfigKey, findPipeIndex, dispatchWithPipe, parsePipeFilters, readFile, isAbsPath, getDir, joinPath, parseIntArg |

**Line boundaries:**
- `model.go` keeps: lines 1-568 (through completions) + lines 1535-1639 (public accessors)
- `model_render.go` gets: lines 570-1057
- `model_commands.go` gets: lines 1061-1533 (includes tokenize helpers at 1343-1424)
- `model_load.go` gets: lines 1641-2340

### 2. `editor/model_test.go` (2213 lines → 4 files)

| New File | Tests |
|----------|-------|
| **model_test.go** | Lifecycle + accessor tests: TestModelValidationOnLoad, TestModelCommitBlockedOnErrors, TestModelCommitSucceedsWhenValid, TestModelRevalidatesOnDiscard, TestModelValidationDebounce, TestModelStatusMessage*, TestModelStatusBarErrorIndicator, TestModelKeyrunesTriggersValidation |
| **model_render_test.go** | TestHighlightValidationIssues*, TestModelContextHighlighting, TestModelStatusBarNoErrorsWhenValid |
| **model_commands_test.go** | TestModelCmdTop, TestModelCmdEdit*, TestModelCmdUp*, TestModelErrorsCommand*, TestModelPipeShow*, TestModelPipeChain, TestSetCommand*, TestTokenizeCommand*, TestJoinTokensWithQuotes, TestEditQuotedListKey, TestSetInQuotedListEntry |
| **model_load_test.go** | TestModelLoadFile, TestModelLoadMerge, TestModelLoadNotFound, TestModelLoadRelativePath, TestLoadFile*, TestLoadOld*, TestLoadTerminal*, TestParseLoadArgs*, TestModelCommitConfirm*, TestModelConfirm*, TestModelAbort* |

### 3. `config/bgp.go` (3093 lines → 4 files)

| New File | Lines (approx) | Content |
|----------|------:|---------|
| **bgp.go** | ~700 | All type definitions (FamilyMode through PluginConfig) + TreeToConfig + validateProcessCapabilities + applyTreeSettings |
| **bgp_peer.go** | ~735 | parsePeerConfig, ribOutParseResult type, parseRIBOutConfig, applyRIBOutParseResult, parseProcessBindings, parseOldProcessBindings, parseNewProcessBinding, parseNLRIEntries, parseReceiveConfig, parseSendConfig, mergeProcessBindings, parseNexthopFamilies |
| **bgp_routes.go** | ~1340 | UpdateBlockRoutes type, parseAnnounceAFIRoutes, extractRoutesFromTree, extractRoutesFromUpdateBlock, all NLRI parsers (FlowSpec/VPLS/MVPN/MUP parse + extract + inline), applyAttributesFromTree, parseRouteConfig, parseLabelsArray, parseInlineKeyValues, parseKeyValuesFromTokens, tokenizeInline, parseAddPathFamily |
| **bgp_util.go** | ~320 | PeerReactor type, PeerGlob type, ipToUint32, ToReactorPeers, parseDurationValue, isValidGroupName, IPGlobMatch, cidrMatch, ipv6GlobMatch, normalizeIPv6Parts, ipv4GlobMatch, ExtractEnvironment |

### 4. `config/bgp_test.go` (3538 lines → 4 files)

| New File | Tests |
|----------|-------|
| **bgp_test.go** | TestBGPSchema*, TestFamilyMode*, TestFamilyConfig*, TestHoldTime*, TestLocalAddress*, TestASN4*, TestExtendedMessage*, TestPerFamilyAddPath*, TestRIBOutConfig* |
| **bgp_peer_test.go** | TestPeerKeyword*, TestSingleInheritance, TestInheritRejected*, TestMatchRejected*, TestGroupNameValidation, TestTemplate*, TestPeerProcess*, TestAPIBinding*, TestMergeProcess*, TestReceiveAll*, TestSendAll*, TestEmpty*, TestMultiple*, TestConfigValidation*, TestOldSyntaxRejected, TestPerNeighborRIBOut |
| **bgp_routes_test.go** | TestParseUpdateBlock_*, TestParseNLRIEntries*, TestAPIConfigNLRI*, TestAPIConfigAttribute* |
| **bgp_util_test.go** | TestIPGlobMatch, TestIPv6GlobMatch, TestCIDRPatternMatch, TestTemplateCIDR*, TestPeerGlob*, TestPluginConfigTimeout* |

## 🧪 TDD Test Plan

### Unit Tests

No new tests — this is a refactor. All existing tests must continue to pass unchanged.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing editor tests | `internal/config/editor/*_test.go` | Editor behavior unchanged | |
| All existing config tests | `internal/config/*_test.go` | Config behavior unchanged | |

### Functional Tests

No new functional tests. All existing must pass.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing | `make functional` | Full system behavior unchanged | |

## Files to Modify
- `internal/config/editor/model.go` - split into 4 files
- `internal/config/editor/model_test.go` - split into 4 test files
- `internal/config/bgp.go` - split into 4 files
- `internal/config/bgp_test.go` - split into 4 test files

## Files to Create
- `internal/config/editor/model_render.go`
- `internal/config/editor/model_commands.go`
- `internal/config/editor/model_load.go`
- `internal/config/editor/model_render_test.go`
- `internal/config/editor/model_commands_test.go`
- `internal/config/editor/model_load_test.go`
- `internal/config/bgp_peer.go`
- `internal/config/bgp_routes.go`
- `internal/config/bgp_util.go`
- `internal/config/bgp_peer_test.go`
- `internal/config/bgp_routes_test.go`
- `internal/config/bgp_util_test.go`

## Implementation Steps

1. **Split `model.go`** — create 3 new source files, move functions
   → **Review:** Do all files compile? `go build ./internal/config/editor/...`

2. **Split `model_test.go`** — create 3 new test files, move test functions
   → **Review:** `go test ./internal/config/editor/...` all pass?

3. **Split `bgp.go`** — create 3 new source files, move functions + file-local types
   → **Review:** `go build ./internal/config/...`

4. **Split `bgp_test.go`** — create 3 new test files, move test functions
   → **Review:** `go test ./internal/config/...`

5. **Full verification** — `make lint && make test && make functional`
   → **Review:** Zero lint issues, all tests pass, no regressions

## Risks

- **Import blocks**: Each new file needs imports for only what it uses. The auto-linter hook runs `goimports` on every write, which handles this automatically.
- **File-local types**: `ribOutParseResult`, `UpdateBlockRoutes`, `PeerGlob`, `PeerReactor` must move with their consumers.
- **Test helpers**: Shared test setup (config strings, helper functions) stays in the base `_test.go` file. Other test files in the same package can reference them.
- **Init ordering**: Go doesn't guarantee `init()` order across files. This package has no `init()` functions, so not a concern.

## Checklist

### 🏗️ Design
- [x] No premature abstraction — pure mechanical split
- [x] No speculative features — reduces existing complexity
- [x] Single responsibility — each new file has one concern
- [x] Explicit behavior — no behavior changes
- [x] Minimal coupling — splitting reduces what must be loaded together

### 🧪 TDD
- [x] Tests written (no new tests — existing tests validate refactor correctness)
- [x] Tests FAIL (N/A — refactor, not new feature; existing tests must keep passing)
- [x] Tests PASS (all existing tests pass after split)
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (224 tests: 42 encode + 45 plugin + 22 parse + 22 decode + 93 editor)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes
- [x] `make test-all` passes (includes ExaBGP: 37 tests)

## Implementation Summary

### What Was Implemented
- Split `editor/model.go` (2340 → 681 + 498 + 486 + 710)
- Split `editor/model_test.go` (2213 → 389 + 262 + 733 + 861)
- Split `config/bgp.go` (3093 → 495 + 986 + 1310 + 330)
- Split `config/bgp_test.go` (3538 → 949 + 1637 + 566 + 408)

### Bugs Found/Fixed
- None — pure mechanical refactor

### Design Insights
- Shared test helpers (e.g., `schemaWithGR`, `parseConfig`) must stay in the base test file since Go test files in the same package share namespace
- The auto-linter hook's goimports handles unused import cleanup automatically
- `gofmt -w` needed after bash-created files (sed/cat don't preserve Go formatting)

### Deviations from Plan
- Spec planned `validateProcessCapabilities` and `applyTreeSettings` in `bgp.go` but they were moved to `bgp_peer.go` (they are peer-related logic, not type definitions)
- `parseAddPathFamily` placed in `bgp_util.go` instead of `bgp_routes.go` (it's a utility parser, not route extraction)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Split editor/model.go into 4 files | ✅ Done | `internal/config/editor/model*.go` | |
| Split editor/model_test.go into 4 files | ✅ Done | `internal/config/editor/model*_test.go` | |
| Split config/bgp.go into 4 files | ✅ Done | `internal/config/bgp*.go` | |
| Split config/bgp_test.go into 4 files | ✅ Done | `internal/config/bgp*_test.go` | |
| No behavior changes | ✅ Done | All tests pass | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All editor tests pass | ✅ Done | `go test ./internal/config/editor/...` | |
| All config tests pass | ✅ Done | `go test ./internal/config/...` | |
| All functional tests pass | ✅ Done | `make functional` (224 tests) | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `editor/model.go` | ✅ Modified | 2340 → 681 lines |
| `editor/model_test.go` | ✅ Modified | 2213 → 389 lines |
| `config/bgp.go` | ✅ Modified | 3093 → 495 lines |
| `config/bgp_test.go` | ✅ Modified | 3538 → 949 lines |
| `editor/model_render.go` | ✅ Created | 498 lines |
| `editor/model_commands.go` | ✅ Created | 486 lines |
| `editor/model_load.go` | ✅ Created | 710 lines |
| `editor/model_render_test.go` | ✅ Created | 262 lines |
| `editor/model_commands_test.go` | ✅ Created | 733 lines |
| `editor/model_load_test.go` | ✅ Created | 861 lines |
| `config/bgp_peer.go` | ✅ Created | 986 lines |
| `config/bgp_routes.go` | ✅ Created | 1310 lines |
| `config/bgp_util.go` | ✅ Created | 330 lines |
| `config/bgp_peer_test.go` | ✅ Created | 1637 lines |
| `config/bgp_routes_test.go` | ✅ Created | 566 lines |
| `config/bgp_util_test.go` | ✅ Created | 408 lines |

### Audit Summary
- **Total items:** 21
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (documented in Deviations — improved grouping)
