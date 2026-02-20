# Implementation Audit Rationale

Why: `.claude/rules/implementation-audit.md`

## Why Tests Passing ≠ Spec Complete

You can: write tests for 70% of features and claim "done", skip difficult features without noticing, forget items after compaction. The audit forces explicit verification of EVERY spec item.

## Good Audit Example

```markdown
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Parse YANG schema | ✅ Done | `yang/parse.go:45` | |
| Route commands to plugins | ✅ Done | `router/dispatch.go:120` | |
| Support nested containers | ⚠️ Partial | `yang/container.go:80` | Only 2 levels deep; user approved |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestParseModule` in `yang/parse_test.go:30` | |
```

## Bad Audit (Don't Do This)

```markdown
## Implementation Audit
Everything was implemented.
```
