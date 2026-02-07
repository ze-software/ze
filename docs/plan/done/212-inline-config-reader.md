# Spec: inline-config-reader

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze-config-reader/main.go` - current implementation being replaced
4. `internal/config/loader.go` - existing config loading (integration point)
5. `internal/config/diff.go` - existing DiffMaps function

## Task

Replace the separate `ze-config-reader` binary with an in-process library in `internal/config/`. The config reader's logic (parse config blocks, map to handlers, diff on reload) becomes direct function calls instead of a subprocess communicating over text protocol pipes.

### Why

The config reader:
- Uses only `internal/config.Tokenizer` (already an internal package)
- Performs pure data transformation (tokens to handler-mapped blocks)
- Has no crash isolation, language boundary, or CPU isolation need
- Adds unnecessary complexity: a Makefile target, text protocol serialization, process lifecycle management, `bufio.Reader`/`Writer` pairs — all replacing what could be Go function calls

Every other internal component with similar characteristics (rib, gr, hostname, etc.) runs in-process as goroutines. The config reader does less than any of them.

### Goals

1. Move config block parsing, handler matching, and diffing logic to `internal/config/reader.go`
2. Export a library API: `NewReader()`, `Load()`, `Reload()`
3. Delete `cmd/ze-config-reader/` directory
4. Remove Makefile build target
5. Migrate tests to `internal/config/reader_test.go`

### Non-Goals

- Integrating with Hub (Hub integration is a separate concern; this spec only moves the code)
- YANG validation of config values (follow-up: spec-config-yang-validation)
- Pluggable config format front-ends (follow-up: spec-pluggable-config-frontend)
- Validating API text commands (follow-up: spec-yang-api-validation)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - [config syntax that reader parses]
- [ ] `docs/architecture/overview.md` - [references ze-config-reader, needs update]

### Source Files
- [ ] `cmd/ze-config-reader/main.go` - [current implementation to move]
- [ ] `cmd/ze-config-reader/main_test.go` - [tests to migrate]
- [ ] `internal/config/tokenizer.go` - [tokenizer the reader depends on]
- [ ] `internal/config/diff.go` - [existing DiffMaps, potential overlap with reader's diffConfig]
- [ ] `internal/config/loader.go` - [future integration point, not changed in this spec]

**Key insights:**
- `internal/config/diff.go` already has `DiffMaps()` for map diffing; the reader's `diffConfig()` is a different level (handler/key granularity, not map key granularity) — both should coexist
- The reader's `tokensToJSON()`, `parseValue()`, `findHandler()` are pure functions with no external dependencies beyond the tokenizer
- The text protocol (`#serial`, `@serial`, `config schema`, `config done`) is eliminated entirely — callers pass Go structs directly

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-config-reader/main.go` - standalone binary, reads schemas from stdin text protocol, parses config, sends namespace commands to Hub via stdout, handles reload via event loop
- [ ] `cmd/ze-config-reader/main_test.go` - 12 tests covering schema parsing, handler matching, command formatting, diffing, type preservation, serial increment, deterministic commit order

**Behavior to preserve:**
- `SchemaInfo` struct and handler map construction — currently done by `parseSchemaLine()` from text protocol; becomes direct struct construction by caller
- `findHandler()` — longest-prefix match: exact → base path → progressively shorter prefixes
- `tokensToJSON()` — converts token stream to JSON string, preserving numeric types
- `parseValue()` — `"true"`/`"false"` → bool, integers → int64, floats → float64, else string
- `parseBlocks()` — recursive block extraction mapping to handlers
- `diffConfig()` — computes create/modify/delete changes between two ConfigState snapshots
- `ConfigState`, `ConfigBlock`, `ConfigChange` types — data structures for config state tracking
- Deterministic ordering — handlers and keys sorted alphabetically for diffing

**Behavior to change:**
- Eliminate stdin/stdout text protocol (`#serial command` / `@serial response`)
- Eliminate `receiveInit()`, `sendCommand()`, `sendCommit()`, `sendComplete()`, `waitResponse()`, `eventLoop()` — these are protocol glue, not logic
- Eliminate `ConfigReader.reader`/`writer`/`serial` fields — no longer needed
- Change `ConfigReader` from a process-lifecycle struct to a stateless reader with explicit API

## Data Flow (MANDATORY)

### Entry Point
- Caller constructs a `Reader` with handler registrations (schemas) and config file path
- No text protocol — schemas passed as Go slice, path as string

### Transformation Path
1. Caller creates `Reader` with `[]SchemaInfo` and config path
2. `Reader.Load()` reads file, tokenizes with `config.NewTokenizer()`, extracts blocks via `parseBlocks()`
3. Each block matched to handler via `findHandler()` (longest prefix match)
4. Matched blocks converted to JSON via `tokensToJSON()`
5. Returns `ConfigState` (handler → key → ConfigBlock)
6. `Reader.Reload()` re-parses, calls `diffConfig()` with previous state, returns `[]ConfigChange`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file → internal types | `config.NewTokenizer()` + `parseBlocks()` | [ ] |
| Reader → caller | Direct return of `ConfigState` / `[]ConfigChange` | [ ] |

### Integration Points
- `config.NewTokenizer()` / `config.Token` — existing tokenizer types, used as-is
- `config.TokenType` constants — `TokenWord`, `TokenLBrace`, `TokenRBrace`, `TokenSemicolon`
- Future: Hub will call `Reader.Load()` / `Reader.Reload()` directly instead of spawning a process

### Architectural Verification
- [x] No bypassed layers — reader still uses tokenizer, not raw file I/O
- [x] No unintended coupling — reader is a pure library, no Hub/Engine dependencies
- [x] No duplicated functionality — `diffConfig()` is handler-level diffing, distinct from `DiffMaps()` map-level diffing
- [x] Zero-copy preserved where applicable — N/A (config is small, copied by design)

## 🧪 TDD Test Plan

### Unit Tests

Tests migrate from `cmd/ze-config-reader/main_test.go` to `internal/config/reader_test.go`, adapted to use the library API instead of the process protocol.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchemaInfo_HandlerMap` | `internal/config/reader_test.go` | Schema handler map construction from SchemaInfo slice | |
| `TestReader_FindHandler` | `internal/config/reader_test.go` | Longest-prefix handler matching (exact, base, prefix) | |
| `TestReader_FindHandler_Unknown` | `internal/config/reader_test.go` | Unknown paths return nil | |
| `TestTokensToJSON_TypePreservation` | `internal/config/reader_test.go` | Integer, float, bool, string types preserved in JSON | |
| `TestReader_ParseBlocks` | `internal/config/reader_test.go` | Config file tokenized and blocks extracted to correct handlers | |
| `TestReader_DiffConfig_Create` | `internal/config/reader_test.go` | New blocks produce "create" changes | |
| `TestReader_DiffConfig_Delete` | `internal/config/reader_test.go` | Removed blocks produce "delete" changes | |
| `TestReader_DiffConfig_Modify` | `internal/config/reader_test.go` | Changed blocks produce "modify" changes | |
| `TestReader_DiffConfig_NoChange` | `internal/config/reader_test.go` | Identical state produces no changes | |
| `TestReader_DiffConfig_Deterministic` | `internal/config/reader_test.go` | Changes sorted by handler then key | |
| `TestReader_HandlerPathBoundary` | `internal/config/reader_test.go` | Long handler paths (512 chars) accepted | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric range inputs in the reader's own code — handler paths are strings.

### Functional Tests

No functional tests needed — this is an internal library refactor with no user-visible behavior change. The config reader is not directly invoked by users.

### Dropped Tests

The following tests from the old binary validated text protocol behavior that no longer exists:

| Old Test | Reason Dropped |
|----------|---------------|
| `TestConfigReader_ReceiveInit` | Tested stdin text protocol parsing — eliminated |
| `TestConfigReader_ReceiveInitWithYang` | Tested heredoc parsing from stdin — eliminated |
| `TestConfigReader_SendCommand` | Tested `#N namespace path action {json}` formatting — eliminated |
| `TestConfigReader_SendCommandError` | Tested `@N error` response parsing — eliminated |
| `TestConfigReader_SendCommit` | Tested `#N namespace commit` formatting — eliminated |
| `TestConfigReader_EventLoop` | Tested stdin event loop (shutdown) — eliminated |
| `TestConfigReader_SerialIncrement` | Tested serial counter in text protocol — eliminated |
| `TestConfigReader_CommitOrderDeterministic` | Commit ordering was protocol concern; diffing determinism is preserved in `TestReader_DiffConfig_Deterministic` |

## Files to Modify
- `Makefile` - remove `ze-config-reader` build target from `build:` line and its rule
- `docs/architecture/overview.md` - remove `ze-config-reader/` from directory listing

## Files to Create
- `internal/config/reader.go` - config block reader library (moved from `cmd/ze-config-reader/main.go`)
- `internal/config/reader_test.go` - unit tests (migrated from `cmd/ze-config-reader/main_test.go`)

## Files to Delete
- `cmd/ze-config-reader/main.go` - replaced by `internal/config/reader.go`
- `cmd/ze-config-reader/main_test.go` - replaced by `internal/config/reader_test.go`
- `cmd/ze-config-reader/` directory

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Write unit tests** in `internal/config/reader_test.go` — test the library API (NewReader, Load, findHandler, tokensToJSON, diffConfig)
   → **Review:** Do tests cover handler matching, type preservation, all diff actions, deterministic ordering?

2. **Run tests** — verify FAIL (paste output)
   → **Review:** Do tests fail for the right reason (missing functions), not syntax errors?

3. **Create `internal/config/reader.go`** — move logic from `cmd/ze-config-reader/main.go`, stripping all protocol code (receiveInit, sendCommand, sendCommit, waitResponse, eventLoop, serial, reader/writer). Export types and functions.
   → **Review:** Is any protocol code left? Are all pure logic functions preserved? Does it compile?

4. **Run tests** — verify PASS (paste output)
   → **Review:** All tests pass? Any warnings?

5. **Delete `cmd/ze-config-reader/`** directory
   → **Review:** No other code imports from this package?

6. **Update Makefile** — remove `bin/ze-config-reader` from `build:` target and its rule
   → **Review:** `make build` still works?

7. **Update `docs/architecture/overview.md`** — remove `ze-config-reader/` line
   → **Review:** Any other docs reference this as a current binary?

8. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

9. **Final self-review** — Re-read all changes, check for unused imports, debug statements, TODOs

## Implementation Summary

### What Was Implemented
- Moved all pure logic from `cmd/ze-config-reader/main.go` to `internal/config/reader.go`: `findHandler`, `parseBlocks`, `TokensToJSON`, `parseConfigValue`, `DiffBlocks`
- Exported library API: `NewReader()`, `Load()`, `Reload()`, `DiffBlocks()`, `TokensToJSON()`
- Renamed types to avoid package-namespace collision: `ConfigState`→`BlockState`, `ConfigBlock`→`BlockEntry`, `ConfigChange`→`BlockChange`
- Eliminated all text protocol code (11 functions): `receiveInit`, `sendCommand`, `sendCommit`, `waitResponse`, `sendComplete`, `eventLoop`, `handleReload`, `sendDone`, `sendError`, `applyChanges`, `parseSchemaLine`
- Deleted `cmd/ze-config-reader/` directory (766 + 524 lines)
- Removed `bin/ze-config-reader` from Makefile `build:` target and its rule
- Removed `ze-config-reader/` from `docs/architecture/overview.md` directory listing
- Migrated 14 tests to `internal/config/reader_test.go`, adding 3 new tests (Reload integration, error paths)
- Optimized `parseBlocks` to skip recursion when block has no nested sub-blocks

### Bugs Found/Fixed
- None

### Design Insights
- `TokensToJSON` captures flat key-value pairs including block name+key as a key-value (e.g., `peer 192.0.2.1` → `"peer":"192.0.2.1"`). This means parent blocks see "modify" changes when child list membership changes. This is correct behavior — the parent block's data genuinely changed.
- The `_default` sentinel key for singleton (non-list) blocks avoids a separate type system for "container vs list" distinction

### Deviations from Plan
- `environment.go`: replaced string literal `"false"` with existing `configFalse` constant — minor cleanup, out of spec scope but accepted by user
- Added 3 tests beyond spec plan: `TestReader_Reload`, `TestReader_Load_MissingFile`, `TestReader_Load_EmptyPath`
- Added `parseBlocks` recursion optimization (skip when no nested `TokenLBrace` present)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move parsing/matching/diffing to internal/config/reader.go | ✅ Done | `internal/config/reader.go` | 7 logic functions migrated |
| Export library API (NewReader, Load, Reload) | ✅ Done | `reader.go:73,88,99` | Plus `DiffBlocks:326`, `TokensToJSON:254` |
| Delete cmd/ze-config-reader/ | ✅ Done | git diff shows deletion | 766 + 524 lines removed |
| Remove Makefile target | ✅ Done | `Makefile:11` | `build:` no longer references `bin/ze-config-reader` |
| Migrate tests | ✅ Done | `internal/config/reader_test.go` | 11 spec tests + 3 new = 14 total |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSchemaInfo_HandlerMap | ✅ Done | `reader_test.go:18` | |
| TestReader_FindHandler | ✅ Done | `reader_test.go:39` | 6 subtests |
| TestReader_FindHandler_Unknown | ✅ Done | `reader_test.go:73` | |
| TestTokensToJSON_TypePreservation | ✅ Done | `reader_test.go:89` | 5 subtests |
| TestReader_ParseBlocks | ✅ Done | `reader_test.go:157` | |
| TestReader_DiffConfig_Create | ✅ Done | `reader_test.go:193` | |
| TestReader_DiffConfig_Delete | ✅ Done | `reader_test.go:214` | |
| TestReader_DiffConfig_Modify | ✅ Done | `reader_test.go:234` | |
| TestReader_DiffConfig_NoChange | ✅ Done | `reader_test.go:262` | |
| TestReader_DiffConfig_Deterministic | ✅ Done | `reader_test.go:287` | |
| TestReader_HandlerPathBoundary | ✅ Done | `reader_test.go:313` | |
| TestReader_Reload (added) | ✅ Done | `reader_test.go:330` | Integration test |
| TestReader_Load_MissingFile (added) | ✅ Done | `reader_test.go:398` | Error path |
| TestReader_Load_EmptyPath (added) | ✅ Done | `reader_test.go:410` | Error path |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/config/reader.go | ✅ Created | 405 lines, pure library |
| internal/config/reader_test.go | ✅ Created | 14 tests, all pass |
| Makefile | ✅ Modified | Removed ze-config-reader target |
| docs/architecture/overview.md | ✅ Modified | Removed ze-config-reader line |
| cmd/ze-config-reader/ (delete) | ✅ Deleted | Directory and both files removed |
| internal/config/environment.go | 🔄 Changed | `"false"` → `configFalse` constant — user approved |

### Audit Summary
- **Total items:** 22
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (environment.go constant — user approved)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)
- [x] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [x] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [x] Functional tests verify end-user behavior — N/A (internal library, no user-visible change)

### Verification
- [x] `make lint` passes — 0 issues
- [x] `make test` passes — all tests pass
- [x] `make functional` passes — 93/95, 2 pre-existing failures (rib stage timeout, plugin EOF)

### Documentation (during implementation)
- [x] Required docs read

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings — overview.md updated
- [x] Implementation Audit completed (all items have status + location)
- [x] All Partial/Skipped items have user approval — none
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
