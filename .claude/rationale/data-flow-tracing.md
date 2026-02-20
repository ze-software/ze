# Data Flow Tracing Rationale

Why: `.claude/rules/data-flow-tracing.md`

## Why Tracing Matters

Ze has distinct layers: Engine ↔ Plugin (JSON pipes), Wire ↔ Parsed ↔ RIB, Negotiated caps affect encoding. Changes correct in isolation may violate boundaries or create impossible data paths.

## Common Boundaries

| Boundary | Allowed | Not Allowed |
|----------|---------|-------------|
| Engine → Plugin | JSON events | Direct function calls |
| Plugin → Engine | Text commands | Direct memory access |
| Wire → Storage | Via iterators/pools | Direct struct copy |
| RIB → Wire | Via PackContext | Ignoring negotiated caps |
| Config → Runtime | Via loader | Global state mutation |

## Bad Spec Example

```markdown
## Files to Modify
- internal/rib/rib.go - add new field
```
Problem: no data flow — how does data get to RIB? What format?

## Good Spec Example

```markdown
## Data Flow
1. Entry: UPDATE via TCP (wire bytes)
2. Parsing: attribute iterator extracts type
3. Storage: pool deduplicates with ref-counting
4. RIB: entry stores ref to pooled attribute
5. Output: JSON event to plugin in attr object (kebab-case)
```
