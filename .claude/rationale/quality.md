# Quality Rationale

Why: `.claude/rules/quality.md`

## Lint Fix Examples

- `hugeParam`: pass large structs by pointer
- `rangeValCopy`: use index or pointer in range
- `shadow`: rename the variable
- `emptyStringTest`: use `s == ""`
- `appendCombine`: combine appends

Every lint check exists for a reason. "Style" issues affect readability. Performance warnings are real.

## When Facing Many Issues

1. Create todo list
2. Fix systematically, file by file
3. No shortcuts regardless of volume
