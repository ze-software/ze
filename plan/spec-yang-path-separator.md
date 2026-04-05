# Spec: yang-path-separator

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/reader.go` - handler path construction
4. `internal/component/config/schema.go` - schema lookup
5. `internal/component/config/yang/validator.go` - validation paths

## Task

Change the internal YANG/config path separator from `.` (dot) to `/` (slash).
Currently, config handler paths use `bgp.peer.timer` while YANG extensions
(`ze:required`, `ze:suggest`) and web/CLI context paths already use `/`.
Unifying on `/` eliminates the inconsistency and aligns with the YANG extension
convention already established in the codebase.

Introduce three helper functions in `internal/component/config/` so callers
express intent ("build a config path") not mechanism ("join with slash"):

| Function | Signature | Purpose |
|----------|-----------|---------|
| `JoinPath` | `JoinPath(parts ...string) string` | Join path segments (replaces `strings.Join(x, ".")`) |
| `AppendPath` | `AppendPath(prefix, name string) string` | Append one segment, handles empty prefix (replaces `prefix + "." + name` with guard) |
| `SplitPath` | `SplitPath(path string) []string` | Split path into segments (replaces `strings.Split(path, ".")`) |

The separator is defined once. Consumers outside `internal/component/config/` (hub, plugin/server,
web, cli/testing, test/runner, bgp/config) call these helpers instead of knowing the separator.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - config path handling
  -> Decision: paths are dot-separated in handler/schema layer
  -> Constraint: env vars (`ze.bgp.X.Y`) use dots as env var convention, NOT YANG paths -- must not change
- [ ] `docs/architecture/config/syntax.md` - config syntax and parsing

**Key insights:**
- Two separator conventions coexist: `.` for handler/schema/validator paths, `/` for YANG extensions and web/CLI
- YANG extensions already use `/` (e.g., `ze:required "connection/remote/ip"`)
- Web config handler and CLI context splitting already use `/`
- Environment variables (`ze.bgp.section.key`) use `.` as env var convention -- this is separate and must not change

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/reader.go` - constructs handler paths with `.` (lines 312, 347), splits with `.` (line 444), joins with `.` (line 446)
- [ ] `internal/component/config/schema.go` - `Lookup()` splits path by `.` (line 242)
- [ ] `internal/component/config/yang/validator.go` - splits by `.` (line 118), constructs child paths with `.` (lines 485, 495, 530, 544)
- [ ] `internal/component/config/yang_schema.go` - constructs child paths with `.` (lines 445, 509, 612, 631)
- [ ] `internal/component/config/diff.go` - joins diff path with `.` (line 90)
- [ ] `internal/component/config/tree.go` - `collectPaths()` builds tree paths with `.` (line 165)
- [ ] `internal/component/config/parser_freeform.go` - warning message uses `name.key` format (line 266)
- [ ] `internal/component/bgp/config/resolve.go` - `deepMergeAt()` builds cumulative key paths with `.` (line 418)
- [ ] `internal/component/hub/schema.go` - splits by `.` (line 103), joins in error message (line 109)
- [ ] `internal/component/plugin/server/schema.go` - splits by `.` (line 327), joins (line 329)
- [ ] `internal/component/plugin/server/hub.go` - constructs handler paths with `.` (lines 213, 215)
- [ ] `internal/component/plugin/server/reload.go` - joins with `.` (lines 361, 371)
- [ ] `internal/component/plugin/server/startup_autoload.go` - `navigateNestedMap()` splits by `.` (line 227), `collectContainerMapPaths()` joins with `.` (line 251)
- [ ] `internal/component/cli/testing/expect.go` - joins context path with `.` for test expectations (line 77)
- [ ] `internal/component/web/cli.go` - `schema.Lookup()` path joined with `.` (line 262)
- [ ] `internal/test/runner/json.go` - JSON diff paths built with `.` (line 225)

**Already using `/`:**
- `internal/component/config/yang_schema.go` line 540 - `ze:required`/`ze:suggest` paths split by `/`
- `cmd/ze/config/cmd_completion.go` line 70 - CLI context splits by `/`
- `internal/component/web/cli.go` lines 146, 445, 523, 662 - web CLI splits by `/`
- `internal/component/web/handler_config.go` line 556 - field paths split by `/`
- `internal/component/web/fragment.go` lines 805, 825, 925 - fragment paths split by `/`

**Environment variables (DO NOT CHANGE):**
- `internal/component/config/environment.go` line 493 - `"ze.bgp." + section + "." + option`
- `internal/component/config/env/env.go` line 30 - `"ze.bgp." + section + "." + key`
- `internal/core/slogutil/slogutil.go` lines 140, 148 - `ze.log.` env var hierarchical lookup
- These use `.` as the env var naming convention, not YANG paths

**Behavior to preserve:**
- Environment variable paths remain dot-separated (`ze.bgp.session.asn`)
- YANG extension paths remain slash-separated (already correct)
- Web/CLI context paths remain slash-separated (already correct)
- Config parsing and serialization unaffected (separator is internal, not in config text syntax)

**Behavior to change:**
- Handler paths: `bgp.peer.timer` becomes `bgp/peer/timer`
- Schema lookup paths: `bgp.peer` becomes `bgp/peer`
- Validator paths: `bgp.peer.session.asn.local` becomes `bgp/peer/session/asn/local`
- Diff paths: same separator change
- Editor test expectations: `context:path=bgp.peer` becomes `context:path=bgp/peer`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file parsed by reader.go, which constructs handler paths
- Schema queries use paths for node lookup
- Validator walks tree and constructs paths for error reporting

### Transformation Path
1. reader.go constructs handler paths during config parsing (`basePrefix + sep + blockName`)
2. Schema lookup splits path to walk the YANG tree
3. Validator constructs child paths as it recurses
4. CLI/web uses paths for context display and navigation
5. Diff uses paths for change reporting

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser -> schema | Path string passed to Lookup() | [ ] |
| Schema -> validator | Path string passed through validation tree | [ ] |
| Plugin server -> hub | Handler path in config section messages | [ ] |
| CLI -> test expectations | ContextPath() joined for comparison | [ ] |

### Integration Points
- `reader.go:312,347` - path construction entry point
- `schema.go:242` - path consumption (split)
- `yang/validator.go:118,485,495,530,544` - path construction and consumption
- `yang_schema.go:445,509,612,631` - child path construction
- `tree.go:165` - tree path collection
- `parser_freeform.go:266` - warning message path format
- `bgp/config/resolve.go:418` - cumulative key path construction
- `hub/schema.go:103` - path consumption
- `plugin/server/schema.go:327,329` - path consumption and reconstruction
- `plugin/server/hub.go:213,215` - path construction
- `plugin/server/reload.go:361,371` - path construction and prefix matching
- `plugin/server/startup_autoload.go:227,251` - path navigation and collection
- `web/cli.go:262` - schema lookup path join
- `cli/testing/expect.go:77` - path display
- `test/runner/json.go:225` - JSON diff path construction

### Architectural Verification
- [ ] No bypassed layers (mechanical find-and-replace, no logic change)
- [ ] No unintended coupling (separator is a string constant, not protocol)
- [ ] No duplicated functionality (unifying two conventions into one)
- [ ] Zero-copy preserved where applicable (N/A - string operations only)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config parse | -> | handler path construction in reader.go | Existing parse tests in `test/parse/` |
| Editor context | -> | context path display | Editor .et tests with updated expectations |
| Schema lookup | -> | path split in schema.go | Existing config unit tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Handler paths in reader.go | Use `/` separator, not `.` |
| AC-2 | Schema Lookup() path split | Splits on `/` |
| AC-3 | Validator child path construction | Uses `/` separator |
| AC-4 | Hub schema query path | Splits on `/` |
| AC-5 | Plugin server schema lookup | Splits on `/` |
| AC-6 | Plugin server hub handler path | Uses `/` separator |
| AC-7 | Plugin server reload path | Uses `/` separator |
| AC-8 | Diff path construction | Uses `/` separator |
| AC-9 | CLI test expect context path | Joins with `/` |
| AC-10 | Editor .et test expectations | Use `/` (e.g., `context:path=bgp/peer`) |
| AC-11 | Environment variable paths | UNCHANGED -- still use `.` |
| AC-12 | YANG extension paths (ze:required) | UNCHANGED -- already use `/` |
| AC-13 | Tree path collection | Uses `/` separator |
| AC-14 | Parser freeform warning path | Uses `/` separator |
| AC-15 | BGP config resolve cumulative key path | Uses `/` separator |
| AC-16 | Plugin server startup autoload paths | Uses `/` separator |
| AC-17 | Web CLI schema lookup path join | Uses `/` separator |
| AC-18 | Test runner JSON diff paths | Uses `/` separator |
| AC-19 | Slogutil env var paths | UNCHANGED -- still use `.` |
| AC-20 | `make ze-verify` passes | All tests pass after change |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing config tests | `internal/component/config/*_test.go` | No regression after separator change | |
| Existing validator tests | `internal/component/config/yang/*_test.go` | Validator paths use `/` | |
| Existing hub tests | `internal/component/hub/*_test.go` | Query paths use `/` | |
| Existing plugin server tests | `internal/component/plugin/server/*_test.go` | Handler paths use `/` | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Config parse tests | `test/parse/*.ci` | Config still parses correctly | |
| Editor context tests | `test/editor/**/*.et` | Context paths display with `/` | |

### Future (if deferring any tests)
- None -- all existing tests must pass with the new separator

## Files to Modify

### Go source -- replace inline separator with helpers

| File | Lines | Change |
|------|-------|--------|
| `internal/component/config/reader.go` | 312, 347, 444, 446 | `AppendPath` (312,347), `SplitPath` (444), `JoinPath` (446) |
| `internal/component/config/schema.go` | 242 | `SplitPath` |
| `internal/component/config/diff.go` | 90 | `AppendPath` |
| `internal/component/config/tree.go` | 165 | `AppendPath` |
| `internal/component/config/parser_freeform.go` | 266 | `AppendPath` |
| `internal/component/config/yang/validator.go` | 118, 485, 495, 530, 544 | `SplitPath` (118), `AppendPath` (485,495,530,544) |
| `internal/component/config/yang_schema.go` | 445, 509, 612, 631 | `AppendPath` |
| `internal/component/bgp/config/resolve.go` | 418 | `AppendPath` |
| `internal/component/hub/schema.go` | 103, 109 | `SplitPath` (103), `JoinPath` (109) |
| `internal/component/plugin/server/schema.go` | 327, 329 | `SplitPath` (327), `JoinPath` (329) |
| `internal/component/plugin/server/hub.go` | 213, 215 | `AppendPath` |
| `internal/component/plugin/server/reload.go` | 361, 371 | `AppendPath` (361), prefix `+ "/"` (371) |
| `internal/component/plugin/server/startup_autoload.go` | 227, 251 | `SplitPath` (227), `AppendPath` (251) |
| `internal/component/web/cli.go` | 262 | `JoinPath` |
| `internal/component/cli/testing/expect.go` | 77 | `JoinPath` |
| `internal/test/runner/json.go` | 225 | `AppendPath` |

### DO NOT MODIFY

| File | Lines | Why |
|------|-------|-----|
| `internal/component/config/environment.go` | 493 | Env var convention, not YANG path |
| `internal/component/config/env/env.go` | 30 | Env var convention |
| `internal/component/config/env/env_test.go` | 66, 148, 215 | Env var test |
| `internal/core/slogutil/slogutil.go` | 140, 148 | Env var convention (`ze.log.`) |
| `internal/component/config/yang_schema.go` | 540 | Already uses `/` for ze:required/ze:suggest |
| `cmd/ze/config/cmd_completion.go` | 70 | Already uses `/` |
| `internal/component/web/cli.go` | 146, 445, 523, 662 | Already uses `/` (but line 262 needs change -- in Files to Modify) |
| `internal/component/web/handler_config.go` | 556 | Already uses `/` |
| `internal/component/web/fragment.go` | 805, 825, 925 | Already uses `/` |

### Editor test expectations (30 .et files)

All `expect=context:path=X.Y.Z` lines must change to `expect=context:path=X/Y/Z`.

### Go test files

Any test asserting dot-separated handler/schema paths must be updated.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] N/A | - |
| CLI commands/flags | [ ] N/A | - |
| Editor autocomplete | [x] test expectations | `test/editor/**/*.et` |
| Functional test for new RPC/API | [ ] N/A | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | - |
| 2 | Config syntax changed? | [ ] No (internal only) | - |
| 3 | CLI command added/changed? | [ ] No | - |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] No | - |
| 6 | Has a user guide page? | [ ] No | - |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented? | [ ] No | - |
| 10 | Test infrastructure changed? | [x] Yes | `docs/functional-tests.md` -- context path format in .et tests uses `/` |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [x] Yes | `docs/architecture/config/yang-config-design.md` -- path separator is `/` |

## Files to Create
- `internal/component/config/path.go` -- `JoinPath`, `AppendPath`, `SplitPath` helpers with separator constant
- `internal/component/config/path_test.go` -- unit tests for the helpers

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- verify line numbers still match |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Path helpers** -- create `JoinPath`, `AppendPath`, `SplitPath` in config package
   - New file: `internal/component/config/path.go` with the three helpers and the separator constant
   - Unit tests: `internal/component/config/path_test.go` -- empty prefix, single segment, multi-segment, round-trip
   - Verify: `go test -race ./internal/component/config/...`

2. **Phase: Core config paths** -- replace inline separator ops with helpers
   - Files: `reader.go` (JoinPath:446, AppendPath:312,347, SplitPath:444), `schema.go` (SplitPath:242), `diff.go` (AppendPath:90), `yang_schema.go` (AppendPath:445,509,612,631), `tree.go` (AppendPath:165), `parser_freeform.go` (AppendPath:266)
   - Verify: `go test -race ./internal/component/config/...`

3. **Phase: Validator paths** -- replace inline separator ops with helpers
   - Files: `yang/validator.go` (SplitPath:118, AppendPath:485,495,530,544)
   - Verify: `go test -race ./internal/component/config/yang/...`

4. **Phase: BGP config paths** -- replace inline separator ops with helpers
   - Files: `bgp/config/resolve.go` (AppendPath:418)
   - Verify: `go test -race ./internal/component/bgp/config/...`

5. **Phase: Hub and plugin server paths** -- replace inline separator ops with helpers
   - Files: `hub/schema.go` (SplitPath:103, JoinPath:109), `plugin/server/schema.go` (SplitPath:327, JoinPath:329), `plugin/server/hub.go` (AppendPath:213,215), `plugin/server/reload.go` (AppendPath:361,371), `plugin/server/startup_autoload.go` (SplitPath:227, AppendPath:251)
   - Verify: `go test -race ./internal/component/hub/... ./internal/component/plugin/server/...`

6. **Phase: Web and CLI paths** -- replace inline separator ops with helpers
   - Files: `web/cli.go` (JoinPath:262), `cli/testing/expect.go` (JoinPath:77)
   - Verify: compiles

7. **Phase: Test runner** -- replace inline separator ops with helpers
   - Files: `test/runner/json.go` (AppendPath:225)
   - Verify: compiles

8. **Phase: Editor test expectations** -- update all .et files
   - Apply ONE file first, run `ze-test editor -p <pattern>`, confirm format
   - Then apply to remaining files
   - Verify: `make ze-editor-test`

9. **Phase: Go test file updates** -- update any test assertions with dot paths
   - Grep for remaining `.` paths in test files
   - Verify: `make ze-unit-test`

10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- fill audit, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every dot-path location changed, no stragglers |
| Correctness | Env var paths unchanged, YANG extension paths unchanged |
| Naming | N/A |
| Data flow | Paths flow through reader -> schema -> validator -> display consistently with `/` |
| Rule: no-layering | No old separator code left (mechanical replacement, not layering) |
| Rule: bulk-edit | Change ONE .et file first, test it, then apply to rest |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Helpers exist | `path.go` has `JoinPath`, `AppendPath`, `SplitPath`; `path_test.go` exists |
| No inline dot separators in path code | `grep -rn '+ "\." +\|"\." +\|+ "\."' reader.go schema.go validator.go tree.go parser_freeform.go resolve.go hub.go reload.go startup_autoload.go json.go cli.go expect.go` returns empty |
| No inline dot splits in path code | `grep -rn 'strings\.Split.*"\."' reader.go schema.go validator.go hub/schema.go startup_autoload.go` returns empty |
| No inline dot joins in path code | `grep -rn 'strings\.Join.*"\."' reader.go schema.go hub/schema.go plugin/server/schema.go cli.go expect.go` returns empty |
| All call sites use helpers | grep for `config.JoinPath\|config.AppendPath\|config.SplitPath` in modified files |
| Env vars still use dots | `grep 'ze\.bgp\.' environment.go env.go` still matches |
| Slogutil still uses dots | `grep 'ze\.log\.' slogutil.go` still matches |
| Editor tests use slash | `grep 'context:path=.*\.' test/editor/**/*.et` returns empty |
| All tests pass | `make ze-verify` exit 0 |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Path traversal | `/` in paths does not enable directory traversal (paths are schema-internal, never used as filesystem paths) |
| Input validation | Config values containing `/` are not confused with path separators (paths are constructed internally, not from user input) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test expects old dot path | Update test expectation to slash |
| Config parse fails | Check reader.go path construction |
| Validator error message has wrong path | Check validator.go childPath construction |
| Editor test fails | Check .et file expectation format |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
- Two path conventions coexisted: `.` (internal handler/schema) and `/` (YANG extensions, web, CLI)
- Environment variable naming (`ze.bgp.X.Y`) uses dots as env var convention, unrelated to YANG paths
- Centralizing the separator in `JoinPath`/`AppendPath`/`SplitPath` helpers means consumers express intent, not mechanism -- and the separator is defined once
- `AppendPath` handles the recurring `if prefix == "" { return name }` guard that appears in 12+ call sites
- Callers outside config package (hub, plugin/server, web, cli/testing, test/runner, bgp/config) no longer need to know the separator character
- 30 .et editor test files need updating -- bulk-edit after validating one

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [ ] Existing tests updated for new separator
- [ ] All tests pass
- [ ] Editor .et tests updated and pass

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
