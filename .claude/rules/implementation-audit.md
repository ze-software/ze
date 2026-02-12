# Implementation Audit

**BLOCKING:** Before marking any spec as done, complete a line-by-line audit comparing the spec to the implementation.

## Why This Exists

Tests passing ≠ spec complete. You can:
- Write tests for 70% of features and claim "done"
- Skip difficult features without noticing
- Forget items after context compaction

The audit forces explicit verification of EVERY spec item.

## When to Audit

Perform implementation audit:
- Before moving spec to `docs/plan/done/`
- Before claiming "done" or "complete"
- Before asking to commit final changes

## Audit Process

### Step 1: Extract All Requirements

Go through the spec and list EVERY item that should be implemented:

| Source Section | Items to Check |
|----------------|----------------|
| Task | Each feature/requirement mentioned |
| 🧪 TDD Test Plan → Unit Tests | Each test row |
| 🧪 TDD Test Plan → Functional Tests | Each test row |
| Files to Modify | Each file listed |
| Files to Create | Each file listed |
| Implementation Steps | Each step completed |

### Step 2: Verify Each Item

For EACH item, determine:

| Status | Meaning | Action Required |
|--------|---------|-----------------|
| ✅ Done | Fully implemented | Record location (file:line) |
| ⚠️ Partial | Partially implemented | Document what's missing, get user approval |
| ❌ Skipped | Not implemented | Get explicit user approval with reason |
| 🔄 Changed | Different from spec | Document deviation and reason |

### Step 3: Fill Audit Table

Complete the Implementation Audit table in the spec:

```markdown
## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [Feature 1] | ✅ Done | `file.go:123` | |
| [Feature 2] | ⚠️ Partial | `file.go:200` | Missing edge case X |
| [Feature 3] | ❌ Skipped | - | User approved: not needed for MVP |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFoo | ✅ Done | `foo_test.go:50` | |
| TestBar | ✅ Done | `bar_test.go:30` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/x/y.go` | ✅ Modified | |
| `test/foo.ci` | ✅ Created | |

### Audit Summary
- **Total items:** N
- **Done:** N
- **Partial:** N (all approved by user)
- **Skipped:** N (all approved by user)
- **Changed:** N (all documented)
```

## Approval Requirements

### Partial Implementation

If any item is ⚠️ Partial:
1. Document exactly what's missing
2. ASK user: "Feature X is partially implemented (missing Y). Approve moving forward?"
3. Record approval in Notes column

### Skipped Items

If any item is ❌ Skipped:
1. Explain why it was skipped
2. ASK user: "Feature X was not implemented because [reason]. Approve?"
3. Record approval in Notes column

### Changed Items

If any item is 🔄 Changed:
1. Document what changed and why
2. Update spec's "Deviations from Plan" section
3. No approval needed if change improves the design

## Red Flags

Stop and investigate if:

- You can't find where a feature was implemented
- A test from the TDD plan doesn't exist
- A file from "Files to Create" wasn't created
- You're unsure whether something was implemented
- New RPCs were added but YANG schema wasn't updated
- New RPCs/APIs were added with only unit tests — functional tests are MANDATORY
- New CLI commands/flags were added but usage text and docs weren't updated
- Integration Checklist items from spec marked "needed" but not in Files from Plan

## Cannot Mark Done Until

```
[ ] Every Task requirement has a status
[ ] Every TDD test has a status
[ ] Every file in plan has a status
[ ] All Partial items have user approval
[ ] All Skipped items have user approval
[ ] Integration points verified (YANG, CLI, docs — per Integration Checklist)
[ ] Audit Summary totals are accurate
```

## Example: Good vs Bad Audit

### Bad Audit (Incomplete)

```markdown
## Implementation Audit
Everything was implemented.
```

**Problem:** No verification, no locations, no evidence.

### Good Audit (Complete)

```markdown
## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Parse YANG schema | ✅ Done | `yang/parse.go:45` | |
| Route commands to plugins | ✅ Done | `router/dispatch.go:120` | |
| Support nested containers | ⚠️ Partial | `yang/container.go:80` | Only 2 levels deep; user approved |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseModule | ✅ Done | `yang/parse_test.go:30` | |
| TestRouteCommand | ✅ Done | `router/dispatch_test.go:55` | |
| test-yang-basic.ci | ✅ Done | `test/yang/basic.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/yang/parse.go` | ✅ Created | |
| `internal/yang/parse_test.go` | ✅ Created | |
| `test/yang/basic.ci` | ✅ Created | |

### Audit Summary
- **Total items:** 8
- **Done:** 7
- **Partial:** 1 (user approved: nesting depth limit acceptable)
- **Skipped:** 0
- **Changed:** 0
```

## Integration

This rule is enforced by:
- Completion Checklist step 3 in `planning.md`
- Spec template includes Implementation Audit section
