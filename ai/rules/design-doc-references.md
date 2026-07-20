# Design Document References

**When:** All `.go` source files (non-test, non-generated) MUST have `// Design:` comment
**Severity:** blocking

## Directives

All `.go` source files (non-test, non-generated) MUST have `// Design:` comment.
Rationale: `ai/rationale/design-doc-references.md`

## Format

```
// Design: docs/architecture/core-design.md — topic annotation
```

Topic annotations preferred over section numbers (survive restructuring).

## Line Ordering

The `// Design:` line must be the first comment in every file. Only compiler
directives (`//go:build`) may precede it:

```
//go:build linux

// Design: docs/architecture/core-design.md — topic annotation
// Related: sibling.go — description
package foo
```

`// Package` doc comments go after the header block, not before it.

## When to Add

| Situation | Action |
|-----------|--------|
| New file | Add before writing code |
| Split file | Inherit from original |
| Touching file without refs | Add for parts you understand |
| No design doc | `// Design: (none — predates documentation)` |

## Exempt

`*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`, files starting with `// Code generated`.
