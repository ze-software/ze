---
name: ze-fix-alloc
description: Use when working in the Ze repo and the user asks for ze-fix-alloc or wants a specific encoding allocation removed. Read the target and its callers, convert the path to the repo's buffer-writing pattern, and verify with lint and unit tests.
---

# Ze Fix Alloc

Use this to remove one concrete allocation in an encoding path.

## Workflow

1. Require a target such as `file.go:line` or `file.go:functionName`.
2. Read the containing function and every caller before editing.
3. Decide which pattern fits:
   - add `WriteTo` to a type that only has `Pack`
   - switch a caller from `Pack` to `WriteTo`
   - convert a helper from returning `[]byte` to writing into a buffer
   - use a session buffer in hot-path code
4. Make the smallest change that removes the allocation without changing behavior.
5. Run `make ze-lint` and `make ze-unit-test`.

## Rules

- Preserve the buffer-first style used by the Ze repo.
- Do not delete compatibility helpers unless all callers are migrated.
