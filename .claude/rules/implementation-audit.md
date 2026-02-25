# Implementation Audit

**BLOCKING:** Before marking any spec done, complete line-by-line audit comparing spec to implementation.
Rationale: `.claude/rationale/implementation-audit.md`

## When

Before: moving spec to `docs/plan/done/`, claiming "done", asking to commit.

## Process

1. Extract all requirements from spec: task items, AC-N assertions, TDD tests, files listed
2. Verify each with status: ✅ Done (file:line), ⚠️ Partial, ❌ Skipped, 🔄 Changed
3. Fill audit table in spec (template in `docs/plan/TEMPLATE.md`)

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
| AC-N done (logic) | Unit test name + file:line | "should work" |

## Red Flags

- AC-N with no test or evidence
- Can't find where feature was implemented
- TDD test from plan doesn't exist
- File from "Files to Create" wasn't created
- New RPCs without functional tests
- New CLI commands without usage text
