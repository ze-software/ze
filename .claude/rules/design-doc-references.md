# Design Document References

**BLOCKING:** All `.go` source files (non-test, non-generated) MUST have `// Design:` comment.
Rationale: `.claude/rationale/design-doc-references.md`

## Format

```
// Design: docs/architecture/core-design.md — topic annotation
```

Place in package doc block (primary file) or as first comment (other files).
Topic annotations preferred over section numbers (survive restructuring).

## When to Add

| Situation | Action |
|-----------|--------|
| New file | Add before writing code |
| Split file | Inherit from original |
| Touching file without refs | Add for parts you understand |
| No design doc | `// Design: (none — predates documentation)` |

## Exempt

`*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`
