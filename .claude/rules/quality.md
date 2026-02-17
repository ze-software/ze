# Quality Standards

## Core Principle

**Do the work properly. No shortcuts.**

## Linting

### Never Disable Linters to Hide Problems

When facing lint issues:
1. **FIX the issue** - this is the only acceptable action
2. **DO NOT** disable linters, checks, or rules to make issues disappear
3. **DO NOT** add exclusion patterns to avoid fixing code

Disabling a linter is hiding problems, not fixing them.

### Acceptable Exclusions

Only these exclusions are permitted:
- `fieldalignment` (govet) - confirm with users if it should be silenced
- Test file exclusions for `dupl`, `goconst`, `prealloc`, `gosec` - tests have different requirements

Everything else: fix the actual code.

### When Facing Many Issues

If there are many lint issues:
1. Create a todo list to track them
2. Fix them systematically, file by file
3. Do not take shortcuts regardless of volume

## Rationale

- Each lint check exists for a reason
- "Style" issues affect readability and maintainability
- Performance warnings are real performance issues
- The linter config reflects project standards - respect it

## What This Means

- If `hugeParam` warns about passing large structs by value: pass by pointer
- If `rangeValCopy` warns about copying in range: use index or pointer
- If `shadow` warns about variable shadowing: rename the variable
- If `emptyStringTest` suggests `s == ""`: use that form
- If `appendCombine` suggests combining appends: combine them

No exceptions. Do the work.

## Self-Critical Review

After each implementation step and before claiming "done":

| Check | Question |
|-------|----------|
| Correctness | Does it actually work? Edge cases handled? |
| Simplicity | Is this the simplest solution? Over-engineered? |
| Consistency | Does it follow existing patterns in the codebase? |
| Completeness | Any TODOs, FIXMEs, or unfinished work? |
| Quality | Debug statements removed? Error messages clear? |
| Tests | Tests cover the change? Any flaky tests introduced? |

When issues are found: **FIX immediately**, document in spec if significant, add a test if the bug could have been caught.

Common issues: unused variables/imports, error paths without cleanup, race conditions, missing validation, inconsistent naming.

## Critical Reviews

When asked for a critical review, validate your understanding of the existing architecture BEFORE agreeing with or proposing changes. Read the actual code/specs first — never assume from memory. Check git history for recent changes to avoid proposing work that's already done.

## Proof of Completion

- Paste command output as proof when claiming something works
- `make ze-verify` output is required before claiming done
- "Should work" is not evidence — run it, paste it

## Anti-Rationalization

See `rules/anti-rationalization.md` for pre-addressed excuses covering TDD, test failures, completion claims, and review responses. If you catch yourself thinking any phrase from those tables, **STOP** — the answer is already "no."
