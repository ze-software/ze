# Lint Gate

**When:** Run before claiming implementation work is complete
**Severity:** blocking

## Directives

Run before claiming implementation work is complete.

## The Problem

The per-edit hook (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) uses
`--new-from-rev=HEAD`, which only catches issues on lines changed since the last
commit. Cross-file effects slip through: unused functions after refactoring,
import issues from renaming, type mismatches across package boundaries.
`make ze-verify` catches these but takes minutes (see `ai/rules/testing.md`
for current timings).

## The Rule

Before claiming any Go implementation work is done, run:

```
make ze-lint-changed
```

This lints all packages with uncommitted Go changes. Takes 3-10 seconds.

Fix every issue it reports. Do not claim done with lint failures outstanding.

## When to run

| Moment | Action |
|--------|--------|
| After finishing all edits for a task | Run `make ze-lint-changed` |
| After fixing lint issues | Re-run to confirm clean |
| Before `/ze-commit` or `/ze-commit-check` | Already covered if you ran it above |

## What it catches that per-edit hooks miss

- Functions/variables made unused by refactoring another file
- Import cycles introduced by cross-package changes
- Type mismatches from interface changes
- Constants/vars that became unreferenced
- Package-level issues that only manifest with full package analysis
