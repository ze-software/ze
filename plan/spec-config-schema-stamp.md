# Spec: config-schema-stamp

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/serialize_set.go` - serializer functions
4. `internal/component/cli/editor_commit.go` - persistence path
5. `internal/component/config/setparser.go` - comment handling in parser

## Task

Add config schema stamping infrastructure so that every committed config file carries
a `# ze-schema: N` header line. This is prep work for future config versioning
(auto-migration on upgrade, downgrade-safe rollback walk). The stamp is emitted on
every commit but not yet consumed for any migration or downgrade logic.

**In scope:** stamp constant, stamp formatter, stamp scanner, stamp emission at the
commit persistence site, tests.

**Out of scope (future spec):** downgrade rollback walk, pre-migration backup at
startup, `cmd_migrate` stamping, any behavioral change based on version mismatch.

### Design History

- Learned 041/065: Ze previously had `ConfigVersion` type and `MigrateV2ToV3()` numbered
  migrations. Removed in favor of detect-based transformation registry. This spec does
  NOT reintroduce numbered migrations. The stamp is a monotonic integer for future
  downgrade recovery only.
- Learned 008: "No version field in config files." This spec uses a comment line, not a
  YANG tree value. The stamp is invisible to the parser (discarded as a comment) and
  re-emitted by the serializer from a binary constant.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config format specification
  -> Constraint: three formats (hierarchical, set, set-meta); comments start with `# ` (hash-space)
  -> Decision: stamp uses `# ze-schema: N` which is a valid comment in all formats

### Source Files
- [ ] `internal/component/config/serialize_set.go:41-81` - DetectFormat, comment handling
  -> Constraint: `# ` prefix is a comment, skipped by DetectFormat
- [ ] `internal/component/config/setparser.go:72` - set parser skips `#` lines
  -> Constraint: stamp line discarded during parse (intentional)
- [ ] `internal/component/config/tokenizer.go:174-193` - hierarchical parser skips `#` comments
  -> Constraint: stamp also discarded in hierarchical parse
- [ ] `internal/component/cli/editor_commit.go:140-165` - commit persistence path
  -> Decision: stamp prepended here, after serialize, before WriteFile
  -> Constraint: `SerializeSetWithMeta` has 23 call sites across 8 files; stamp must NOT be in the serializer

**Key insights:**
- Both parsers discard `# ` comments. Stamp survives round-trip because the serializer
  re-emits it from a constant, not from the tree.
- Only one persistence site for committed config: `editor_commit.go:162`.
- Display-only callers (show config, fmt, annotated) must NOT emit the stamp.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/serialize_set.go` - SerializeSet, SerializeSetWithMeta
- [ ] `internal/component/config/serialize.go:161` - Serialize (hierarchical)
- [ ] `internal/component/cli/editor_commit.go:140-165` - commit path
- [ ] `internal/component/config/storage/storage.go` - WriteVersion, rollback copies

**Behavior to preserve:**
- Serialize functions return tree content without any header (23 callers for display depend on this)
- `DetectFormat` skips `# ` comments (stamp must not affect format detection)
- Set parser skips `#` lines (stamp must not appear in parsed tree)
- Hierarchical tokenizer skips `#` comments (same)
- Rollback copies store raw file content as-is (stamp included if present)

**Behavior to change:**
- Committed config file gains a `# ze-schema: 1` first line on every commit

## Data Flow (MANDATORY)

### Entry Point
- Config commit via CLI editor (`CommitSession`)

### Transformation Path
1. Editor builds committed tree + meta (`editor_commit.go:153-160`)
2. `SerializeSetWithMeta` produces config text (no stamp) (`editor_commit.go:161`)
3. `FormatSchemaStamp(SchemaVersion)` produces stamp line
4. Stamp prepended to serialized output
5. `guard.WriteFile` writes stamped content to `config.conf` (`editor_commit.go:162`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config package -> cli package | `FormatSchemaStamp` called from editor_commit | [ ] |

### Integration Points
- `editor_commit.go:161-162` - stamp prepended between serialize and WriteFile

### Architectural Verification
- [ ] No bypassed layers (stamp added at persistence point, not in serializer)
- [ ] No unintended coupling (version.go has no dependencies beyond stdlib)
- [ ] No duplicated functionality (new capability, nothing to duplicate)
- [ ] Zero-copy preserved where applicable (string concat, not hot path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI config commit | -> | `FormatSchemaStamp` prepended to output | `TestCommitStampsSchemaVersion` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ScanSchemaVersion` given `# ze-schema: 3\nset bgp ...` | Returns 3 |
| AC-2 | `ScanSchemaVersion` given `set bgp ...` (no stamp) | Returns 0 |
| AC-3 | `ScanSchemaVersion` given empty input | Returns 0 |
| AC-4 | `ScanSchemaVersion` given `# ze-schema: abc\n...` (non-numeric) | Returns 0 |
| AC-5 | `FormatSchemaStamp(1)` | Returns `"# ze-schema: 1\n"` |
| AC-6 | Config commit via editor | Written file starts with `# ze-schema: 1\n` |
| AC-7 | `show config` display | No stamp line in output |
| AC-8 | `ScanSchemaVersion` given `# ze-schema: 1\n` (stamp only, no config) | Returns 1 |
| AC-9 | Rollback copy of stamped config | Stamp present in rollback file (inherited from source) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestScanSchemaVersion` | `internal/component/config/version_test.go` | AC-1: parses stamp from first line | |
| `TestScanSchemaVersionMissing` | `internal/component/config/version_test.go` | AC-2: returns 0 when no stamp | |
| `TestScanSchemaVersionEmpty` | `internal/component/config/version_test.go` | AC-3: returns 0 for empty input | |
| `TestScanSchemaVersionInvalid` | `internal/component/config/version_test.go` | AC-4: returns 0 for non-numeric | |
| `TestFormatSchemaStamp` | `internal/component/config/version_test.go` | AC-5: correct format | |
| `TestScanSchemaVersionStampOnly` | `internal/component/config/version_test.go` | AC-8: works with stamp-only input | |
| `TestSchemaStampRoundTrip` | `internal/component/config/version_test.go` | stamp -> scan -> same value | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| version int | 0-MaxInt | N/A (monotonic) | N/A (0 = absent) | N/A (future versions) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestCommitStampsSchemaVersion` | `internal/component/cli/editor_commit_test.go` | AC-6: commit writes stamped config | |

### Future (deferred to config-schema-downgrade spec)
- Downgrade walk tests
- Startup migration backup tests

## Files to Modify
- `internal/component/cli/editor_commit.go` - prepend stamp at line 161

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` - document stamp line |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` - schema stamp section |

## Files to Create
- `internal/component/config/version.go` - SchemaVersion constant, ScanSchemaVersion, FormatSchemaStamp
- `internal/component/config/version_test.go` - unit tests

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - create version.go with stub, add stamp call site in editor_commit.go
   - Tests: `TestCommitStampsSchemaVersion`
   - Files: `version.go`, `editor_commit.go`
   - Verify: wiring test fails because `FormatSchemaStamp` returns empty stub

2. **Phase: Scanner + Formatter** - implement `ScanSchemaVersion` and `FormatSchemaStamp`
   - Tests: `TestScanSchemaVersion`, `TestScanSchemaVersionMissing`, `TestScanSchemaVersionEmpty`, `TestScanSchemaVersionInvalid`, `TestFormatSchemaStamp`, `TestScanSchemaVersionStampOnly`, `TestSchemaStampRoundTrip`
   - Files: `version.go`, `version_test.go`
   - Verify: all unit tests pass

3. **Phase: Commit integration** - wire stamp into editor_commit.go
   - Tests: `TestCommitStampsSchemaVersion`
   - Files: `editor_commit.go`
   - Verify: wiring test passes, `show config` unaffected

4. **Functional tests** - verify stamp present in committed file, absent from display
5. **Full verification** - `make ze-verify`

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-9 all have tests |
| Correctness | Stamp format matches `# ze-schema: N\n` exactly |
| Naming | `SchemaVersion` not `ConfigVersion` (avoids collision with removed type from learned 041) |
| Data flow | Stamp added at persistence site only, not in serializers |
| Rule: no-layering | No YANG leaf added (comment-only approach) |
| Rule: design-history | No numbered migration groups reintroduced |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `version.go` exists | `ls internal/component/config/version.go` |
| `version_test.go` exists | `ls internal/component/config/version_test.go` |
| `SchemaVersion` constant exported | `grep -n 'SchemaVersion' internal/component/config/version.go` |
| `ScanSchemaVersion` exported | `grep -n 'func ScanSchemaVersion' internal/component/config/version.go` |
| `FormatSchemaStamp` exported | `grep -n 'func FormatSchemaStamp' internal/component/config/version.go` |
| Stamp in editor_commit.go | `grep -n 'SchemaStamp\|FormatSchema' internal/component/cli/editor_commit.go` |
| No stamp in serialize functions | `grep -c 'SchemaStamp\|FormatSchema' internal/component/config/serialize*.go` returns 0 |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | `ScanSchemaVersion` must handle malicious first lines (very long lines, binary data, negative numbers) without panic |
| No user-editable version | Stamp is a comment, not a YANG leaf; users can delete it but result is version 0 (safe default) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

N/A - no RFC work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/740-config-schema-stamp.md`
- [ ] Summary included in commit
