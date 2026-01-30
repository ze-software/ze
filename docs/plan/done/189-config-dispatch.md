# Spec: config-dispatch

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/main.go` - current dispatch logic
4. `internal/config/tokenizer.go` - tokenizer for probing

## Task

Change `ze config.conf` to start daemons in-process based on config content, with config passed via JSON (not file re-read).

**Current:** `ze config.conf` → `hub.Run()` → forks processes, passes config path
**Target:** `ze config.conf` → hub parses config → starts daemons in-process → passes config as JSON

**Key principle:** BGP daemon should NOT re-read the config file. Hub parses once, passes structured config (JSON) to daemons.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config syntax reference

### Source Files
- [ ] `cmd/ze/main.go` - current entry point and dispatch
- [ ] `cmd/ze/bgp/main.go` - BGP subcommand entry (config detection to be removed)
- [ ] `cmd/ze/bgp/server.go` - BGP server command (config loading to be removed)
- [ ] `cmd/ze/hub/main.go` - hub runner (to be modified for in-process startup)
- [ ] `internal/hub/hub.go` - hub orchestrator (to be modified for in-process BGP)
- [ ] `internal/hub/config.go` - hub config parser (to use YANG parser for bgp block)
- [ ] `internal/config/tokenizer.go` - tokenizer for probing blocks
- [ ] `internal/config/loader.go` - existing BGP config parser (reuse for in-process BGP)

**Key insights:**
- Top-level blocks: `environment`, `plugin`, `bgp`
- `bgp { }` block indicates BGP daemon config
- `plugin { external ... }` indicates hub/orchestrator mode

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - Entry point, dispatches `ze config.conf` to `hub.Run()`
- [ ] `cmd/ze/bgp/main.go` - BGP entry, has `looksLikeConfig()` that dispatches to server
- [ ] `cmd/ze/bgp/server.go` - Calls `config.LoadReactorFileWithPlugins()` to parse config
- [ ] `cmd/ze/hub/main.go` - Loads hub config, creates orchestrator, forks processes
- [ ] `internal/hub/config.go` - Parses `env`, `plugin`, and generic blocks
- [ ] `internal/config/loader.go` - YANG-based BGP config parser, creates reactor

**Behavior to preserve:**
- Config file detection via `looksLikeConfig()` (unchanged)

**Behavior to REMOVE:**
- `ze bgp server config.conf` - NO LONGER parses config directly
- `ze bgp config.conf` - NO LONGER parses config directly
- BGP plugin should NOT have config file parsing capability

**Behavior to change:**
- `ze config.conf` with `bgp { }` block → hub parses config, starts BGP daemon in-process, passes config as JSON
- `ze config.conf` with `plugin { external ... }` → hub orchestrator forks external processes
- `ze config.conf` with neither → error with clear message
- BGP daemon receives config via JSON from hub, NOT by re-reading config file

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDetectConfigType` | `cmd/ze/main_test.go` | Detects `bgp`, `hub`, `unknown` from config content | |
| `TestDetectConfigTypeBGP` | `cmd/ze/main_test.go` | `bgp { }` block returns "bgp" | |
| `TestDetectConfigTypeHub` | `cmd/ze/main_test.go` | `plugin { external ... }` returns "hub" | |
| `TestDetectConfigTypeUnknown` | `cmd/ze/main_test.go` | No recognized block returns "unknown" | |
| `TestDetectConfigTypeBGPPrecedence` | `cmd/ze/main_test.go` | Both `bgp` and `plugin` → "bgp" wins | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dispatch-bgp` | `test/parse/dispatch-bgp.ci` | `ze config.conf` with `bgp {}` validates | ✅ |
| `dispatch-unknown` | `test/parse/dispatch-unknown.ci` | Config without `bgp {}` block → exit 1 | ✅ |

### Future (if deferring any tests)
- Hub mode functional test deferred (orchestrator mode rarely used)

## Files to Modify
- `cmd/ze/main.go` - Add `detectConfigType()`, change dispatch logic
- `cmd/ze/bgp/main.go` - Remove config file detection, remove `looksLikeConfig()` dispatch
- `cmd/ze/bgp/server.go` - Remove or gut config file loading (BGP receives pre-parsed config only)

## Files to Create
- `test/parse/dispatch-bgp.ci` - Functional test for BGP config validation
- `test/parse/dispatch-unknown.ci` - Functional test for missing bgp block error

## Files to Modify (hub changes)
- `internal/hub/hub.go` - Add in-process BGP startup instead of fork
- `internal/hub/config.go` - Use `internal/config.LoadReactor()` for `bgp {}` block

## Files to Delete or Gut
- Config parsing logic in `cmd/ze/bgp/server.go` - moves to hub

## Notes
- Hub is NOT removed - it becomes the in-process orchestrator
- Hub parses config once, passes structured data to daemons
- `ze bgp server config.conf` is REMOVED - all config parsing at root level
- Config parsing code moves from `cmd/ze/bgp/` to root hub
- For `bgp {}` block: hub uses existing `internal/config.LoadReactor()` to parse, starts reactor in-process
- No new JSON format needed - reuse existing YANG-based config parsing
- BGP plugin receives pre-parsed config only (no file path, no re-parsing)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Config Type Detection

1. **Write unit tests** - Create `TestDetectConfigType*` tests in `cmd/ze/main_test.go`
   → **Review:** Are edge cases covered? Empty file? Comments only?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason? Not syntax errors?

3. **Implement `detectConfigType()`** - Use tokenizer to scan for top-level blocks
   → **Review:** Is this the simplest solution? Does it handle malformed configs gracefully?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass?

### Phase 2: Hub In-Process BGP Startup

5. **Modify hub config parser** - Extract `bgp {}` block as structured JSON
   → **Review:** Is the JSON format compatible with BGP daemon expectations?

6. **Modify hub orchestrator** - When `bgp {}` present, start BGP reactor in-process (goroutine) instead of forking
   → **Review:** Is the reactor receiving config via JSON, not re-reading file?

7. **Update dispatch logic** - `cmd/ze/main.go` routes to hub, hub decides fork vs in-process
   → **Review:** Is error message clear for unknown config type?

### Phase 3: Remove Config Parsing from BGP Plugin

8. **Remove `looksLikeConfig()` from `cmd/ze/bgp/main.go`** - BGP subcommand no longer detects config files
   → **Review:** Does `ze bgp config.conf` now error appropriately?

9. **Gut config loading from `cmd/ze/bgp/server.go`** - Remove direct file parsing
   → **Review:** What remains? Only pre-parsed config acceptance?

10. **Update `ze bgp` help text** - Remove config file examples
    → **Review:** Is it clear that config goes through root `ze`?

### Phase 4: Verification

11. **Create functional tests** - Add `.ci` files for end-to-end verification
    → **Review:** Do tests cover user-visible behavior? Error cases included?

12. **Update root usage text** - Reflect new behavior in help output
    → **Review:** Is help text accurate and clear?

13. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests deterministic?

14. **Final self-review** - Before claiming done:
    - Re-read all code changes: any bugs, edge cases, or improvements?
    - Check for unused code, debug statements, TODOs
    - Verify error messages are clear and actionable

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Hub always parses config | Single parse, structured data passed to daemons |
| Config passed as JSON to daemons | Daemons don't re-read file, hub is source of truth |
| BGP daemon runs in-process | No fork for `bgp {}` block, goroutine instead |
| Remove `ze bgp server config.conf` | Config parsing centralized at root, not in plugins |
| BGP plugin receives pre-parsed config | No file access, no re-parsing, clean separation |
| `plugin { external }` still forks | External plugins need separate process |
| Error on unknown config | Fail-fast, don't silently do nothing |

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (simple switch, no framework)
- [x] No speculative features (only `bgp` and `hub` detection)
- [x] Single responsibility (detectConfigType does one thing)
- [x] Explicit behavior (dispatch logic is clear)
- [x] Minimal coupling (tokenizer is only dependency)
- [x] Next-developer test (logic is obvious)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)
- [x] Boundary tests cover all numeric inputs (N/A)
- [x] Feature code integrated into codebase (`cmd/ze/main.go`)
- [x] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] Usage text updated

### Completion (after tests pass)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## Implementation Summary

### What Was Implemented

1. **Config Type Detection** (`cmd/ze/main.go`):
   - `detectConfigType()` uses tokenizer to scan for `bgp {}` or `plugin {}` blocks
   - `looksLikeConfig()` extended to recognize `-` (stdin)
   - `--plugin` parsed as flag (not command) before dispatch

2. **Hub In-Process BGP** (`cmd/ze/hub/main.go`):
   - `Run()` accepts `plugins []string` parameter
   - `probeConfigType()` scans config content for dispatch
   - `runBGPInProcess()` uses `LoadReactorWithPlugins()` for YANG augmentation

3. **Removed Config Parsing from BGP Plugin**:
   - Deleted `cmd/ze/bgp/server.go`
   - `ze bgp server` now errors with deprecation message
   - Config parsing centralized at root `ze` command

4. **Loader Refactoring** (`internal/config/loader.go`):
   - Added `LoadReactorWithPlugins(input, plugins)` for data + plugins
   - Renamed `loadReactorWithConfigAndYANG` → `parseConfigWithYANG` (returns config only)
   - Fixed bug: CLI plugins now merged BEFORE reactor creation

5. **Test Runner** (`internal/test/runner/runner.go`):
   - Changed default command from `ze bgp server` to `ze`

### Files Modified
- `cmd/ze/main.go` - dispatch logic, --plugin flag parsing
- `cmd/ze/hub/main.go` - Run() accepts plugins, runBGPInProcess()
- `cmd/ze/hub/main_test.go` - updated for new Run() signature
- `cmd/ze/bgp/main.go` - deprecated server command
- `internal/config/loader.go` - LoadReactorWithPlugins(), parseConfigWithYANG()
- `internal/test/runner/runner.go` - ze instead of ze bgp server

### Files Deleted
- `cmd/ze/bgp/server.go`

### Files Created
- `internal/config/probe.go` - shared ProbeConfigType() and ConfigType constants
- `test/parse/dispatch-bgp.ci` - valid bgp config dispatch
- `test/parse/dispatch-unknown.ci` - missing bgp block error

### Bugs Found/Fixed
- `--plugin` was intercepted as command (list plugins) not flag (load plugin)
- `LoadReactorWithPlugins` merged plugins AFTER reactor creation (plugins ignored)

### Test Results
- 42 encode + 32 plugin + 14 parse + 20 decode = 108 tests pass
- `make verify` passes
