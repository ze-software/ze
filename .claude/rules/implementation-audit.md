# Implementation Audit

**BLOCKING:** Before marking any spec done, complete line-by-line audit comparing spec to implementation.
Rationale: `.claude/rationale/implementation-audit.md`

## When

Before: writing summary to `plan/learned/`, claiming "done", asking to commit.

## Process

1. Extract all requirements from spec: task items, AC-N assertions, TDD tests, files listed
2. Verify each with status: ✅ Done (file:line), ⚠️ Partial, ❌ Skipped, 🔄 Changed
3. Fill audit table in spec (template in `plan/TEMPLATE.md`)

## Approval Required

- ⚠️ Partial: document what's missing, ASK user
- ❌ Skipped: explain why, ASK user
- 🔄 Changed: document deviation (no approval needed if improvement)

## Cannot Mark Done Until

```
[ ] Every Task requirement has a status
[ ] Every AC-N has status + "Demonstrated By" evidence
[ ] Every TDD test has a status
[ ] Every file in plan has a status
[ ] All Partial/Skipped have user approval
[ ] Integration points verified (YANG, CLI, docs)
[ ] Wiring Test table complete — every row has a test name, none deferred
[ ] Audit Summary totals accurate
```

## Evidence Standards

| Claim | Acceptable Evidence | NOT Acceptable |
|-------|-------------------|----------------|
| Feature works | Test name + output | "make ze-verify passes" |
| Feature is wired in | Wiring test that exercises entry→feature path | Unit test with mock/fake entry point |
| AC-N done (wiring) | Functional test name exercising full path | Unit test in isolation |
| AC-N done (logic) | Unit test name + file:line, assertion matches AC text | "should work" |
| AC-N done (behavior) | Test asserts the AC's expected behavior directly | Test asserts mechanism (e.g., "no error" as proxy for "rejected") |

## AC Evidence Verification (BLOCKING)

For each AC-N, quote the expected behavior from the AC table, then name the test and its assertion. The assertion must verify the BEHAVIOR, not just the mechanism.

**Mechanical check:** Read the AC text. Read the test assertion. If the test would still pass with a no-op implementation, the evidence is invalid.

| Pattern | Invalid evidence | Valid evidence |
|---------|-----------------|----------------|
| AC says "routes not installed" | Test checks no error returned | Test checks route is absent from delivery callback |
| AC says "session torn down" | Test checks NOTIFICATION struct created | Test checks connection closed |
| AC says "config rejected" | Test checks error is non-nil | Test checks error message contains expected text |

## Mechanically Enforced

Hook `pre-commit-spec-audit.sh` (exit 2) blocks `git commit` when:

| Check | What it verifies |
|-------|-----------------|
| Wiring Test .ci files | Every `.ci` path in Wiring Test table exists on disk |
| Functional Tests .ci files | Every `.ci` path in Functional Tests table exists on disk |
| Files to Create | Every file path in "Files to Create" section exists on disk |
| TDD test files | Every `_test.go` file referenced in TDD Plan exists on disk |
| Audit tables filled | All 4 audit tables have data rows (not just headers) |
| AC evidence | Every AC row has non-empty "Demonstrated By" column |
| Audit Summary | Has actual totals (not template placeholders) |
| Pre-Commit Verification | Section exists with filled Files Exist + AC Verified + Wiring Verified tables |
| Learned summary | Exists and does not contain "not wired", "library only", etc. |

Bypass: clear `.claude/selected-spec` for unrelated commits.

## Pre-Commit Verification (BLOCKING)

**Do NOT trust the audit.** After filling the audit, independently re-verify every item.
This is a separate section in the spec (see `TEMPLATE.md`). It requires FRESH evidence:

| Table | What to verify | How |
|-------|---------------|-----|
| Files Exist | Every file from "Files to Create" | `ls -la <path>` — paste output |
| AC Verified | Every AC-N | grep, test output, or ls — NOT a copy from audit |
| Wiring Verified | Every wiring test row | Read the .ci file, confirm it tests the claimed path |

**NOT acceptable:** "Already checked in audit", "should work", empty cells.

## Red Flags

- AC-N with no test or evidence
- Can't find where feature was implemented
- TDD test from plan doesn't exist
- File from "Files to Create" wasn't created
- New RPCs without functional tests
- New CLI commands without usage text
- Learned summary admits incompleteness ("not wired", "infrastructure only")
- Commit message says "library and interface only"
