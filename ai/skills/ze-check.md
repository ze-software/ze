---
name: ze-check
description: Check
---

# Check

Run completeness and health checks on uncommitted changes without preparing a commit.

See also: `/ze-commit` (commit without verification), `/ze-commit-check` (commit with verification)

## Steps

1. **Verify (if Go code changed):** Check if any `.go` files are modified. If yes:
   run `./le verify worktree` (foreground, largest timeout your harness allows). If it fails, report all failures from
   `tmp/ze-verify-failures.log`. If no `.go` files changed: skip.
2. **Show scope:** Run `git status` and `git diff --stat` to identify all changed files.

3. **Completeness check:** For each changed file, check whether companion artifacts
   are missing. Present findings as a table.

   | Change type | Expected companion |
   |-------------|-------------------|
   | New/changed CLI command | `docs/guide/command-reference.md` updated |
   | New/changed config option | `docs/guide/configuration.md` or YANG doc updated |
   | New/changed wire format | `docs/architecture/wire/` updated |
   | New/changed web endpoint | docs or inline help updated |
   | New/changed plugin behavior | `docs/guide/plugins.md` updated |
   | New feature | `docs/features.md` entry |
   | New/changed API | `docs/architecture/api/commands.md` updated |
   | New/changed skill | canonical `ai/skills/` source exists |
   | New/changed `.claude/` rule | `ai/INSTRUCTIONS.md` pointer if needed |
   | New exported Go symbol | at least one non-test caller |
   | New user-facing behavior | `.ci` or `.et` functional test exists |

4. **Health check:** Run all sub-checks below.

5. **Report:** Present a single consolidated table.

   ```
   ## Check Results

   | # | Type | Location | Detail | Status |
   |---|------|----------|--------|--------|
   | 1 | missing docs | docs/features.md | new feature X not listed | MISSING |
   | 2 | stale ref | .claude/rules/foo.md:12 | `internal/old/deleted.go` | STALE | <!-- doc-links: ignore (example output) -->
   | 3 | wiring | internal/component/host/info.go | GetHostInfo has no non-test caller | UNWIRED |

   N issues found. [or "Clean -- no issues."]
   ```

## Health Sub-Checks

### 4a. Stale file references

Scan `.claude/rules/*.md`, `.claude/skills/*/SKILL.md`, and `ai/rationale/*.md` for
backtick-quoted file paths (containing `/`, ending in `.go`, `.md`, `.yang`, `.sh`).
For each: does the file exist? If not: **STALE REF**.

Skip URLs, rule references like `rules/foo.md`, and relative `.claude/` paths. <!-- doc-links: ignore (example pattern) -->

### 4b. Skill cross-references

Scan all `.claude/skills/*/SKILL.md` for `/ze-` references. For each:
does the target skill directory exist? If not: **BROKEN SKILL REF**.

### 4c. INDEX.md link check

For each entry in `ai/INDEX.md` that points to a `docs/` file:
does the target file exist? If not: **BROKEN INDEX LINK**.

### 4d. Memory staleness

For each memory file that references a specific file path, function, or type:
does it still exist? Skip preference/feedback memories.

### 4e. Native hook registration

Run `./le hook-check unit`, then compare the configured hook kinds with the
actions listed by `./le hook-check`. A configured kind with no native action is
**MISSING HOOK ACTION**.

### 4f. Wiring check

For every new exported symbol in the diff:
```
grep -rn 'Symbol' internal/ cmd/ --include="*.go" | grep -v "_test.go" | grep -v "plan/"
```
If the only hits are definition and test files: **UNWIRED**.

### 4g. Canonical source check

For every modified file in the diff, check whether it is a generated file
(per `ai/rules/repo-maintenance.md`). If a generated file was edited directly:
**WRONG SOURCE** -- the canonical file should have been edited instead.

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT prepare a commit script.
- After presenting the report, ask the user what to address.
