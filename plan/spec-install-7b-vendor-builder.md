# Spec: install-7b-vendor-builder

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-install-7a-namespace |
| Phase | - |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/install/appliance/cmd_build.go` - existing build with gok shell-out
4. `gokrazy/tools/cmd/ze-gok/main.go` - existing gok.Context usage pattern
5. `gokrazy/tools/vendor/github.com/gokrazy/tools/gok/gok.go` - gok API

## Task

Replace the external `bin/gok` shell-out in `ze install appliance build` with a
direct Go API call to `gok.Context{}.Execute()`. This eliminates the only external
build-tool dependency, making ze self-contained for image building.

Currently `cmd_build.go:runGokBuild()` calls `runExternalFn(gokBinary, ...)` where
`gokBinary = "bin/gok"`. The `bin/gok` binary is built from `gokrazy/tools/cmd/ze-gok/`
which is a thin wrapper around `gok.Context{}.Execute()` that sets GOMODCACHE.

After this change, `runGokBuild()` calls `gok.Context{}.Execute()` directly with
the same GOMODCACHE setup. No external binary needed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - section 20: Appliance Config Loading Priority
  -> Decision: gokrazy config directory structure (gokrazy/ze/config.json)
  -> Constraint: GOMODCACHE must point to gokrazy/modcache/ for offline builds
- [ ] `docs/guide/appliance.md` - user-facing appliance documentation
  -> Constraint: build workflow documentation must be updated

### Learned Summaries
- [ ] `plan/learned/675-appliance-1-builder.md` - builder decisions
  -> Decision: runExternalFn is a replaceable function variable for testability
  -> Constraint: gok invocation + ext4 inject uses external binaries
- [ ] `plan/learned/578-gokrazy-3-build.md` - gokrazy build config
  -> Decision: explicit GokrazyPackages, GOMODCACHE under gokrazy/modcache/
  -> Constraint: gok hardcodes -mod=mod, GOMODCACHE redirect is required

**Key insights:**
- `gok.Context{Args, Stdin, Stdout, Stderr}.Execute(ctx)` is the full API (gok/gok.go)
- ze-gok already demonstrates the pattern: set GOMODCACHE, call Execute()
- gokrazy/tools brings ~11 new deps to ze's go.mod (cobra, pflag, gokapi, etc.)
- e2fsprogs (mkfs.ext4, debugfs) dependency REMAINS for ZeFS /perm injection
- runExternalFn is a test-injection point; replace its gok usage but keep it for e2fsprogs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/install/appliance/cmd_build.go` - builds image via runExternalFn(gokBinary, ...)
  -> Constraint: runGokBuild() is the only gok call site; injectZeFS() uses runExternalFn for e2fsprogs
  -> Constraint: buildOne() checks `os.Stat(gokBinary)` before build; remove this check
  -> Constraint: runExternalFn returns ([]byte, error); gok.Execute returns error only
- [ ] `gokrazy/tools/cmd/ze-gok/main.go` - wraps gok.Context with GOMODCACHE
  -> Decision: GOMODCACHE = filepath.Join(wd, "gokrazy", "modcache")
  -> Constraint: MkdirAll on modcache before Execute()
- [ ] `gokrazy/tools/vendor/github.com/gokrazy/tools/gok/gok.go` - gok API
  -> Constraint: Context{Stdin, Stdout, Stderr, Args}; Execute(ctx) error
- [ ] `gokrazy/tools/go.mod` - separate module ze-gokrazy-tools
  -> Constraint: gokrazy/tools v0.0.0-20260406155313-5861e2403dc8
  -> Constraint: 15 deps, ~6 overlap with ze's go.mod (x/sys, x/sync, x/mod, x/crypto)

**Behavior to preserve:**
- `buildOne()` flow: LoadConfig -> assemble zefs -> gok build -> inject zefs -> checksum -> manifest
- `injectZeFS()` using e2fsprogs (mkfs.ext4, debugfs, dd) via runExternalFn unchanged
- `findLastPartition()` pure Go GPT reader unchanged
- `buildAll()` parallel build unchanged
- `runExternalFn` test injection for e2fsprogs calls preserved
- GOMODCACHE pointing to gokrazy/modcache/ for offline builds
- All gok CLI args unchanged (--parent_dir, -i, overwrite, --full, --target_storage_bytes)

**Behavior to change:**
- `runGokBuild()`: replace `runExternalFn(gokBinary, ...)` with `gok.Context{}.Execute(ctx)`
- Remove `gokBinary` var and `os.Stat(gokBinary)` check in `buildOne()`
- Add GOMODCACHE setup before gok.Execute() (same logic as ze-gok/main.go)
- Add `github.com/gokrazy/tools` to ze's main go.mod
- Add `gokBuildFn` test-injection variable (replaces runExternalFn for gok only)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze install appliance build <name>` CLI invocation

### Transformation Path
1. `runBuild(args)` parses flags, resolves appliance name
2. `buildOne(name)` loads config, assembles zefs, calls `runGokBuild()`
3. `runGokBuild()` sets GOMODCACHE, creates `gok.Context` with args, calls `.Execute(ctx)`
4. gok.Execute() internally: cross-compiles Go packages, creates GPT image, writes partitions
5. `injectZeFS()` formats last partition (ext4), injects zefs database (unchanged, still e2fsprogs)
6. `WriteImageChecksum()` + `WriteManifest()` finalize

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> appliance package | `appliance.Run(args)` dispatch | [ ] |
| appliance -> gokrazy builder | `gok.Context{}.Execute(ctx)` (Go API, not exec) | [ ] |
| appliance -> e2fsprogs | `runExternalFn(mkfs, ...)` (unchanged, still external) | [ ] |

### Integration Points
- `gok.Context{}.Execute()` replaces `runExternalFn(gokBinary, ...)` in `runGokBuild()`
- GOMODCACHE env var set before Execute() (same as ze-gok pattern)
- `go.mod` gains gokrazy/tools dependency + transitive deps
- `runExternalFn` retained for e2fsprogs calls only

### Architectural Verification
- [ ] No bypassed layers (gok called through public API, not internal packages)
- [ ] No unintended coupling (gok dependency is import-only, no shared state)
- [ ] No duplicated functionality (replaces shell-out, does not add parallel path)
- [ ] Zero-copy preserved where applicable (image written to disk by gok internally)

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze install appliance build <name>` | -> | `gok.Context{}.Execute()` in `runGokBuild()` | `TestBuildUsesGokAPI` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install appliance build lab` with no `bin/gok` present | Build succeeds (no external binary dependency) |
| AC-2 | `runGokBuild()` called | Uses `gok.Context{}.Execute()`, not `runExternalFn` |
| AC-3 | gok.Execute() called | GOMODCACHE set to `gokrazy/modcache/` before call |
| AC-4 | gok.Execute() fails | Error message includes gok failure details |
| AC-5 | `buildOne()` succeeds | Full flow works: assemble -> gok build -> inject -> checksum -> manifest |
| AC-6 | `go.mod` updated | Contains `github.com/gokrazy/tools` at pinned version |
| AC-7 | `gokBuildFn` replaced in test | Tests can inject mock gok builder |
| AC-8 | `buildAll()` called | All appliances built using vendored gok (not external binary) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildUsesGokAPI` | `cmd/ze/install/appliance/cmd_build_test.go` | runGokBuild uses gokBuildFn (not runExternalFn for gok) | |
| `TestBuildNoGokBinary` | `cmd/ze/install/appliance/cmd_build_test.go` | Build does not check for bin/gok existence | |
| `TestBuildGOKMODCACHE` | `cmd/ze/install/appliance/cmd_build_test.go` | GOMODCACHE set correctly before gok call | |
| `TestBuildGokFailure` | `cmd/ze/install/appliance/cmd_build_test.go` | gok failure propagated with error details | |
| `TestInjectZeFSUnchanged` | `cmd/ze/install/appliance/cmd_build_test.go` | injectZeFS still uses runExternalFn for e2fsprogs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-build-no-gok` | `test/install/appliance-build-no-gok.ci` | `ze install appliance build` succeeds without bin/gok | |

### Future (if deferring any tests)
- Full image build integration test (requires gokrazy modcache populated)
- e2fsprogs replacement with pure Go ext4 writer (separate spec)

## Files to Modify

- `cmd/ze/install/appliance/cmd_build.go` - replace gok shell-out with Go API call
- `go.mod` - add github.com/gokrazy/tools dependency
- `go.sum` - updated by go mod tidy

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A |
| CLI commands/flags | No | CLI unchanged, only internal implementation |
| Go module dependencies | Yes | `go.mod` (github.com/gokrazy/tools + transitive) |
| Doctor check for runtime dependencies | No | bin/gok check removed (was not a doctor check) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A (internal change) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` - remove "run: make bin/gok" prerequisite |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `cmd/ze/install/appliance/cmd_build_test.go` - tests for vendored builder (if not exists)
- `test/install/appliance-build-no-gok.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- add gokBuildFn injection point
   - Tests: `TestBuildUsesGokAPI` (fails: gokBuildFn does not exist yet)
   - Files: `cmd_build.go` (add var gokBuildFn)
   - Verify: test compiles, fails because gokBuildFn is stub

2. **Phase: Vendor gok dependency** -- add to go.mod, import in cmd_build.go
   - Tests: compilation check
   - Files: `go.mod`, `cmd_build.go` (add import)
   - Verify: `go build ./cmd/ze/...` succeeds

3. **Phase: Replace gok shell-out** -- rewrite runGokBuild to use gok.Context
   - Tests: `TestBuildUsesGokAPI`, `TestBuildNoGokBinary`, `TestBuildGOKMODCACHE`
   - Files: `cmd_build.go` (runGokBuild, buildOne)
   - Verify: tests pass, gokBinary var removed, os.Stat check removed

4. **Phase: Error handling** -- gok failure propagation
   - Tests: `TestBuildGokFailure`
   - Files: `cmd_build.go` (error wrapping in runGokBuild)
   - Verify: test passes

5. **Functional tests** -> Create after feature works
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | GOMODCACHE set correctly, gok args identical to previous |
| Naming | gokBuildFn follows runExternalFn naming convention |
| Data flow | gok.Execute() called with same args as previous runExternalFn call |
| Rule: no-layering | gokBinary var fully removed, not just unused |
| Backward compat | injectZeFS unchanged, still uses runExternalFn for e2fsprogs |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| gokBinary var removed | `grep -n gokBinary cmd_build.go` returns nothing |
| `gok.Context` import present | `grep 'gokrazy/tools/gok' cmd_build.go` |
| GOMODCACHE set before Execute | grep in runGokBuild |
| runExternalFn retained for e2fsprogs | grep shows mkfs/debugfs/dd calls unchanged |
| go.mod has gokrazy/tools | `grep gokrazy/tools go.mod` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Dependency audit | gokrazy/tools is maintained by gokrazy project, used by ze already (ze-gok) |
| GOMODCACHE | Must not point outside repo (same constraint as ze-gok) |
| No new network access | gok with GOMODCACHE uses only local files, no network |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error after adding dep | Fix go.mod version, check for conflicts |
| gok.Execute() behaves differently than shell-out | Compare args, check GOMODCACHE, check env |
| Test injection doesn't work | Ensure gokBuildFn is var, not const |
| Circular dependency | gokrazy/tools should not import ze |
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

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Import gok into main module | Keep separate module + binary | Eliminates external dependency; ~11 new deps is acceptable (6 overlap) |
| gokBuildFn injection var | Mock runExternalFn for all calls | Keeps e2fsprogs path testable via runExternalFn; separates concerns |
| GOMODCACHE in runGokBuild | Global env setup at startup | Localized to the one function that needs it; no side effects |

## Known Limitations

- e2fsprogs (mkfs.ext4, debugfs) still needed for ZeFS injection into /perm
- gok.Context is args-based (string CLI args, not structured API); gok CLI changes could break
- GOMODCACHE must be pre-populated (same as current; `make ze-gokrazy-deps`)
- Cross-compilation requires Go toolchain on build machine (same as current)

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Build without external gok binary | functional test | `test/install/appliance-build-no-gok.ci` |
| Same image output | unit test | `TestBuildUsesGokAPI` verifies same args |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

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
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
