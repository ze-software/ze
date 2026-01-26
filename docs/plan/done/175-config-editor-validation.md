# Spec: config-editor-validation (Phases 1-2)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/editor/` - existing editor implementation
4. `internal/hub/schema.go` - ConfigStore (live/edit)
5. `docs/plan/done/004-edit-command.md` - original editor spec

## Task

Enhance the config editor with:
1. Real-time validation as user types
2. Full validation at commit time
3. Persistent edit state between sessions (until commit)
4. Hub integration for edit file detection
5. Move entry point to `ze config edit <file>`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config syntax rules
- [ ] `docs/architecture/hub-architecture.md` - hub design

### Source Files
- [ ] `internal/config/editor/editor.go` - current editor
- [ ] `internal/config/editor/model.go` - Bubble Tea model
- [ ] `internal/hub/schema.go` - ConfigStore
- [ ] `cmd/ze/bgp/config_edit.go` - current entry point

**Key insights:**
- Editor already has full VyOS-like command set
- Backup system uses `<name>-YYYY-MM-DD-<N>.conf` format
- ConfigStore has live/edit separation but not connected to editor
- Schema-driven completion already parses input for suggestions

## Design

### Edit File Persistence

Edit sessions are persisted to allow leaving and returning:

| File | Purpose |
|------|---------|
| `<name>.conf` | Original/committed config |
| `<name>.conf.edit` | Uncommitted edit session |
| `<name>-YYYY-MM-DD-<N>.conf` | Backups (on commit) |

**Lifecycle:**
1. `ze config edit foo.conf` - loads `foo.conf.edit` if exists, else `foo.conf`
2. Any change creates/updates `foo.conf.edit`
3. `commit` - validates, creates backup, writes to `foo.conf`, deletes `foo.conf.edit`
4. `discard` - deletes `foo.conf.edit`, reloads from `foo.conf`
5. Exit without commit - `foo.conf.edit` preserved for next session

### Validation Levels

| Level | When | What |
|-------|------|------|
| Syntax | As you type | Tokenization, bracket matching, semicolons |
| Schema | As you type | Known keywords, value types, required fields |
| Semantic | On commit | Cross-field rules (peer-as ≠ local-as, unique addresses) |

### Real-Time Validation

Validation runs on each keystroke with debouncing (100ms):

**Syntax errors:**
- Missing semicolon
- Unclosed braces
- Invalid tokens

**Schema errors:**
- Unknown keyword at path
- Wrong value type (string vs number vs IP)
- Missing required field in block

**Display:**
- Error indicator in status bar: `⚠️ 2 errors`
- Error details on `Ctrl+E` or `errors` command
- Inline highlighting (red underline) for error location

### Commit-Time Validation

Before saving, full validation runs:

**Semantic rules:**
- `peer-as` ≠ `local-as` (unless route reflector)
- `local-address` exists on system (optional, warn only)
- `router-id` is valid IPv4
- No duplicate peer addresses
- Referenced groups/templates exist

**Commit behavior:**
1. Run all validation levels
2. If errors: show error list, abort commit
3. If warnings only: show warnings, prompt "Commit anyway? [y/N]"
4. If clean: create backup, save, delete `.edit` file

### Entry Point Changes

**New:** `ze config edit <file>` - top-level config editor
**Keep:** `ze bgp config edit <file>` - alias for backwards compatibility
**Separate:** `ze cli` - API commands to running daemon (not config editing)

### Hub Integration

On startup, editor checks for existing edit file:

```
$ ze config edit /etc/ze/config.conf

Found uncommitted changes from 2025-01-25 14:30.
  [c] Continue editing
  [d] Discard and start fresh
  [v] View changes first
Choice:
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEditFilePersistence` | `editor_test.go` | Edit file created on change, deleted on commit | ✅ |
| `TestEditFileResume` | `editor_test.go` | Loads .edit file if exists | ✅ |
| `TestEditFileDeletedOnCommit` | `editor_test.go` | Edit file deleted on commit | ✅ |
| `TestEditFileDeletedOnDiscard` | `editor_test.go` | Edit file deleted on discard | ✅ |
| `TestValidateSyntaxMissingSemicolon` | `validator_test.go` | Detects missing semicolons | ✅ |
| `TestValidateSyntaxUnclosedBrace` | `validator_test.go` | Detects unclosed braces | ✅ |
| `TestValidateSemanticPeerAsLocalAs` | `validator_test.go` | Detects peer-as = local-as | ✅ |
| `TestValidateSemanticDuplicatePeer` | `validator_test.go` | Detects duplicate peers | ✅ |
| `TestValidateSemanticRouterID` | `validator_test.go` | Validates router-id format | ✅ |
| `TestValidateSemanticHoldTime` | `validator_test.go` | RFC 4271 hold-time rules | ✅ |
| `TestModelCommitBlockedOnErrors` | `model_test.go` | Commit blocked when errors exist | ✅ |
| `TestModelErrorsCommand` | `model_test.go` | Errors command shows issues | ✅ |
| `TestSchemaValidation` | `validator_test.go` | Detects unknown keywords, wrong types | (Phase 3) |
| `TestCommitWithWarnings` | `editor_test.go` | Commit prompts on warnings | (Phase 2) |
| `TestValidationDebounce` | `model_test.go` | Validation doesn't run on every keystroke | (Phase 2) |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN | 1-4294967295 | 4294967295 | 0 | 4294967296 |
| Hold time | 0 or 3-65535 | 65535 | 1, 2 | 65536 |
| Port | 1-65535 | 65535 | 0 | 65536 |
| Router-ID | valid IPv4 | 255.255.255.255 | N/A | 256.0.0.0 |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `edit-resume` | `test/config/edit-resume.ci` | Start edit, exit, resume, commit | |
| `edit-validation-realtime` | `test/config/edit-validation.ci` | Type invalid, see error, fix, error clears | |
| `edit-commit-blocked` | `test/config/edit-commit.ci` | Try commit with errors, blocked | |

## Files to Modify

- `cmd/ze/main.go` - add `config` subcommand routing
- `cmd/ze/bgp/config_edit.go` - move to `cmd/ze/config/edit.go`
- `internal/config/editor/editor.go` - add edit file persistence
- `internal/config/editor/model.go` - add validation display, debouncing

## Files to Create

- `cmd/ze/config/main.go` - config subcommand entry
- `cmd/ze/config/edit.go` - edit command (moved from bgp)
- `internal/config/editor/validator.go` - validation engine
- `internal/config/editor/validator_test.go` - validation tests
- `internal/config/editor/semantic.go` - semantic rules
- `internal/config/editor/semantic_test.go` - semantic tests

## Implementation Steps

1. **Create validator infrastructure**
   - Create `validator.go` with `Validate(content string) []Error`
   - Implement syntax validation (tokenizer errors)
   - Write tests, verify FAIL
   → **Review:** Are all syntax error cases covered?

2. **Add schema validation**
   - Extend validator to check against schema
   - Unknown keywords, wrong value types
   - Write tests, verify FAIL
   → **Review:** Does it catch all schema violations?

3. **Add semantic validation**
   - Create `semantic.go` with cross-field rules
   - peer-as ≠ local-as, unique addresses, valid router-id
   - Write tests, verify FAIL
   → **Review:** Are semantic rules complete?

4. **Implement edit file persistence**
   - Modify `Editor` to use `.edit` file
   - Create on first change, delete on commit/discard
   - Write tests, verify FAIL
   → **Review:** Edge cases (permissions, disk full)?

5. **Add real-time validation to model**
   - Debounced validation on content change
   - Error display in status bar
   - Error list command
   - Write tests, verify FAIL
   → **Review:** Is debouncing working correctly?

6. **Add commit-time validation**
   - Block commit if errors
   - Prompt on warnings
   - Write tests, verify FAIL
   → **Review:** UX for error display clear?

7. **Move entry point**
   - Create `cmd/ze/config/` package
   - Move edit command, update routing
   - Keep `ze bgp config edit` as alias
   → **Review:** Both entry points work?

8. **Add edit file detection on startup**
   - Check for `.edit` file
   - Prompt user for action
   → **Review:** All prompt options work?

9. **Verify all**
   - `make lint && make test && make functional`
   → **Review:** Zero issues?

## Validation Rules

### Syntax Rules

| Rule | Example Error |
|------|---------------|
| Missing semicolon | `peer-as 65001` (no `;`) |
| Unclosed brace | `peer 1.1.1.1 {` (no `}`) |
| Invalid token | `peer-as "not a number";` |
| Empty block | `peer 1.1.1.1 { }` (warning) |

### Schema Rules

| Rule | Example Error |
|------|---------------|
| Unknown keyword | `unknown-field value;` |
| Wrong type | `hold-time "ninety";` (expected number) |
| Missing required | `peer 1.1.1.1 { }` (needs peer-as) |
| Invalid enum | `origin xyz;` (must be igp/egp/incomplete) |

### Semantic Rules

| Rule | Severity | Example |
|------|----------|---------|
| peer-as = local-as | Error | Both 65001 (unless RR client) |
| Duplicate peer address | Error | Two peers with 192.0.2.1 |
| Invalid router-id | Error | `router-id 999.999.999.999;` |
| Local address not on system | Warning | `local-address 10.0.0.1;` (not found) |
| Hold time 1 or 2 | Error | RFC requires 0 or ≥3 |

## UI Design

### Status Bar

```
zebgp[peer 192.0.2.1]# set hold-time 90
                                        ⚠️ 2 errors │ Modified │ Ctrl+? help
```

### Error List (`errors` or `Ctrl+E`)

```
Errors (2):
  Line 5: peer-as equals local-as (both 65001)
  Line 12: unknown keyword 'hld-time' (did you mean 'hold-time'?)

Warnings (1):
  Line 8: local-address 10.99.0.1 not found on system
```

### Commit Blocked

```
zebgp# commit
Cannot commit: 2 errors found.
Run 'errors' to see details.
```

### Commit with Warnings

```
zebgp# commit
Warnings (1):
  Line 8: local-address 10.99.0.1 not found on system

Commit anyway? [y/N]:
```

## Checklist

### 🏗️ Design
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during development)
- [x] Implementation complete (Phase 1)
- [x] Tests PASS
- [x] Boundary tests cover all numeric inputs (hold-time 0,1,2,3,65535)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] Architecture docs updated with learnings (N/A - editor feature, no core architecture changes)

## Out of Scope (Separate Specs)

### `ze cli` - API Command Interface

`ze cli` is for sending API commands to running plugins, NOT config editing:

```bash
ze cli                           # Interactive mode, connects to running hub
ze cli peer list                 # One-shot command
ze cli rib show ipv4/unicast     # Query RIB plugin
```

This requires:
- Unix socket connection to hub
- Command routing to plugins
- Response formatting

**Separate spec needed:** `spec-cli-api-interface.md`

### Live Config Apply

Applying config changes to running daemon (beyond file save):
- Connect to hub
- Send config verify/apply
- Handle partial failures

**Separate spec needed:** `spec-config-live-apply.md`

## Implementation Summary

### What Was Implemented

**Phase 1 - Core Validation (Completed):**

1. **ConfigValidator** (`validator.go`)
   - Syntax validation: missing semicolons, unclosed braces, extra close braces
   - Semantic validation: router-id format, hold-time RFC compliance, peer-as = local-as check, duplicate peer detection
   - Returns line numbers and clear error messages

2. **Edit File Persistence** (`editor.go`)
   - `HasPendingEdit()` - detects existing .edit file
   - `LoadPendingEdit()` - loads content from .edit file
   - `SaveEditState()` - writes working content to .edit file
   - Edit file deleted on commit or discard

3. **Model Validation Integration** (`model.go`)
   - Validation runs on load and after discard
   - `cmdCommit()` - blocks commit if validation errors exist
   - `cmdErrors()` - displays error list with line numbers
   - `runValidation()` - re-validates current content

4. **Tests**
   - `validator_test.go` - syntax and semantic validation tests
   - `editor_test.go` - edit file persistence tests
   - `model_test.go` - commit blocking and errors command tests

**Phase 2 - Real-Time Validation (Completed):**

1. **Debounced Validation** (`model.go`)
   - `scheduleValidation()` - schedules validation after 100ms debounce
   - `validationTickMsg` - handles debounce tick, ignores stale ticks
   - Triggers on each keystroke via `KeyRunes` handler

2. **Status Bar Error Indicator** (`model.go`)
   - Shows `⚠️ N error(s)` in header when errors exist
   - Shows `⚡ N warning(s)` for warnings only

3. **Inline Error/Warning Highlighting** (`model.go`)
   - `highlightValidationIssues()` - applies styling to error/warning lines
   - `errorLineStyle` - red text on dark red background (errors)
   - `warningLineStyle` - yellow text on dark yellow background (warnings)
   - Errors take precedence over warnings on same line
   - Called by `setViewportData()` before displaying config

4. **Explicit Data Bundling** (`model.go`)
   - `viewportData` struct - bundles content + lineMapping together
   - `setViewportData(viewportData)` - displays bundled data with highlighting
   - `setViewportText(string)` - convenience for non-config content
   - `showConfigContent()` - handles context filtering and display
   - `filterContentByContext()` - returns viewportData with mapping
   - Eliminates implicit coupling between cmdShow and setViewport

5. **commandResult Pattern** (`model.go`)
   - Commands return `commandResult` struct instead of mutating model
   - `commandResult` carries: output, configView, statusMessage, newContext, clearContext, isTemplate, showHelp, revalidate
   - Update handler applies changes from result - fixes Bubble Tea closure capture issue
   - `statusMessage` field for temporary notifications (cleared on next command)

6. **Hierarchical Context Paths** (`model.go`)
   - `findFullContextPath()` - finds full hierarchical path to target block (e.g., `["bgp", "peer", "1.1.1.1"]`)
   - `filterContentByContextPath()` - extracts content for hierarchical path with exact matching
   - `cmdEdit()` uses hierarchical paths matching actual config tree structure
   - `cmdUp()` navigates up the hierarchy properly
   - Exact block matching prevents false positives (e.g., `peer` won't match `peer-as`)

7. **Template Mode** (`model.go`)
   - `findParentOfKeyword()` - finds parent block where keyword blocks exist
   - `edit peer *` enters template mode for editing defaults applying to all peers
   - Template context: parent + keyword + "*" (e.g., `["bgp", "peer", "*"]`)
   - `isTemplate` flag tracks template mode state

8. **Tests** (`model_test.go`)
   - `TestHighlightValidationIssues` - verifies error line styling
   - `TestHighlightValidationIssuesEmpty` - handles empty error list
   - `TestHighlightValidationIssuesOutOfRange` - ignores invalid line numbers
   - `TestHighlightValidationIssuesWithMapping` - verifies line mapping works
   - `TestHighlightValidationIssuesWarnings` - verifies warning styling
   - `TestHighlightValidationIssuesErrorPrecedence` - errors override warnings
   - `TestModelContextHighlighting` - integration test for filtered view
   - `TestModelValidationDebounce` - verifies tick matching logic
   - `TestModelStatusBarErrorIndicator` - verifies error display
   - `TestModelKeyrunesTriggersValidation` - verifies keystroke triggers
   - `TestModelCmdTop`, `TestModelCmdUp`, `TestModelCmdUpAtRoot` - navigation tests
   - `TestModelCmdUpFromTemplate` - navigation from template context
   - `TestModelCmdEditHierarchical` - hierarchical path matching
   - `TestModelCmdEditNotFound` - error on block not found
   - `TestModelCmdEditFromContext` - relative edit from context
   - `TestModelCmdEditExactMatch` - exact vs prefix matching
   - `TestModelCmdEditWildcardTemplate` - template mode entry
   - `TestModelStatusMessageDisplay` - temporary notification display
   - `TestModelStatusMessageClearsOnCommand` - notification clears on next command
   - `TestModelStatusMessageClearsOnError` - notification clears on error

### Deferred to Future Phases

**Phase 3 - Schema Validation:**
- Unknown keyword detection
- Value type checking (string vs number vs IP)
- Required field validation

**Phase 4 - Entry Point Changes:**
- Move to `ze config edit <file>`
- Keep `ze bgp config edit` as alias

**Phase 5 - Hub Integration:**
- Edit file detection prompt on startup
- ConfigStore connection

### Design Insights

- Validation types split clearly: ConfigValidator (not Validator) to avoid collision with yang.Validator
- Semantic validation uses regex for simplicity - could be replaced with full parser if needed
- Edit file is adjacent to config (foo.conf.edit) for easy discovery
- Bubble Tea model must use immutable pattern - commands return results, Update applies changes
- Hierarchical context paths enable proper config tree navigation (matching actual brace structure)
- Template mode (`edit peer *`) requires separate path finding - parent of keyword, not the keyword itself

### Bugs Found/Fixed

- Type name collision: `Validator` already exists in `internal/yang/validator.go`, renamed to `ConfigValidator`
- Closure capture bug: tea.Cmd closures captured model by value, losing state changes. Fixed with `commandResult` pattern
- `cmdCommit()` used stale validation state - fixed by validating inline
- Prefix matching too loose (`peer` matched `peer-as`) - fixed with exact block matching
- Brace handling order: `} foo {` processed incorrectly - fixed by processing `}` before `{`
- Wildcard template broken by block-not-found error - added `findParentOfKeyword()` for special handling

### Deviations from Plan

- Implemented semantic validation in validator.go, not separate semantic.go (simpler structure)
- Warning prompts deferred - current implementation blocks on errors, doesn't distinguish warnings
- Added `commandResult` pattern not in original plan - required to fix Bubble Tea closure capture issue
- Hierarchical context paths not in original plan - emerged from need for proper tree navigation

## Status

**Phases 1 and 2 complete.** Remaining phases (3-5) can be separate specs if needed:
- Phase 3: Schema validation (unknown keywords, type checking)
- Phase 4: Entry point changes (`ze config edit`)
- Phase 5: Hub integration (edit file detection prompt)
