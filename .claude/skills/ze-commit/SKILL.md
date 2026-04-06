# Scoped Commit

Prepare a commit script with explicit scope verification. Does NOT run git commit or git add directly.

See also: `/ze-verify` (must pass before committing)

## Steps

1. **Verify tests passed (if Go code changed):** Check if any `.go` files are in the commit scope. If yes: run `make ze-verify-changed` (scoped to modified packages only, timeout 180s); if it fails, stop and report all failures. If no `.go` files changed (docs-only, `.claude/` config, specs, etc.): skip verification entirely.
2. **Show scope:** Run `git status` and `git diff --stat` to identify all changed files.
3. **Identify task scope:** Determine which files belong to the current task. If unclear, ask the user.
4. **Exclude unrelated changes:** If files outside the task scope are modified, explicitly list them and confirm with the user: "These files are outside the current task scope: [list]. Exclude from commit?"
5. **Health check (conditional):** Only if `.claude/` files are in the commit scope, run the .claude health check (see below). Skip entirely for pure code/docs commits.
6. **Check recent commits:** Run `git log --oneline -5` to match commit message style.
7. **Draft commit message:** Based on the actual changes (not the spec), write a concise commit message.
8. **Generate commit script:** Write to `tmp/commit-SESSION.sh` where SESSION is the 8-char session ID:

```bash
#!/bin/bash
set -e
git add file1.go file2.go file3_test.go
git commit -m "$(cat <<'EOF'
type: subject line

Body explaining why.
EOF
)"
```

9. **Remaining work table (BLOCKING -- must appear before the commit script):**
   Before showing the commit script, present a table of what is NOT included in this commit.
   This lets the user decide whether to continue working before committing.

   ```
   ## Remaining After This Commit

   | # | Item | Status | Where |
   |---|------|--------|-------|
   | 1 | AC-3: warn-only mode | deferred | plan/deferrals.md |
   | 2 | setparser edge case for empty lists | todo | internal/component/config/setparser.go:142 |
   | 3 | functional test for reload with new option | todo | test/reload/ |

   Nothing remaining. [or table above]
   ```

   Sources to check:
   - **Spec ACs:** if work was driven by a spec, list any AC-N not covered by this commit
   - **Deferrals:** open items in `plan/deferrals.md` related to this work
   - **TODOs in code:** any TODO/FIXME added or pre-existing in the changed files
   - **Uncommitted files:** files modified but excluded from this commit scope
   - **Known gaps:** anything mentioned during the session as "not yet done"

   If nothing remains, say "Nothing remaining." Do not skip this table.

10. **Present to user:** Show the remaining work table, then the staged files, commit message, and health check results. The user runs `bash tmp/commit-SESSION.sh` themselves.

## Health Check

**Only runs when `.claude/` files are in the commit scope.** Skip entirely for pure code/docs commits.

Checks that the .claude system is consistent with the codebase. Reports findings as a table at the end of the commit preparation.

### 5a. Stale file references

Scan `.claude/rules/*.md`, `.claude/skills/*/SKILL.md`, and `.claude/rationale/*.md` for file path references (backtick-quoted paths like `internal/foo/bar.go` or `docs/guide/thing.md`). For each path found:
- Does the file exist? If not: **STALE REF** -- the rule/skill references a deleted file.

Only check paths that look like project files (contain `/` and end in `.go`, `.md`, `.yang`, `.sh`, or a directory pattern). Skip URLs, rule references like `rules/foo.md`, and relative `.claude/` paths.

### 5b. Skill cross-references

Scan all `.claude/skills/*/SKILL.md` for `/ze-` references. For each:
- Does the target skill directory exist? If not: **BROKEN SKILL REF**.

### 5c. INDEX.md link check

For each entry in `.claude/INDEX.md` that points to a `docs/` file:
- Does the target file exist? If not: **BROKEN INDEX LINK**.

### 5d. Memory staleness (quick)

For each file in the memory directory (`~/.claude/projects/.../memory/` or `.claude/memory/`):
- If the memory references a specific file path, function, or type: does it still exist?
- Only check memories that name concrete code artifacts. Skip preference/feedback memories.

### 5e. Hook script existence

For each hook in `.claude/settings.json` that references a script via `$CLAUDE_PROJECT_DIR/.claude/hooks/`:
- Does the script file exist? If not: **MISSING HOOK SCRIPT**.

### Report format

```
## Health Check

| # | Type | Location | Reference | Status |
|---|------|----------|-----------|--------|
| 1 | stale ref | rules/foo.md:12 | `internal/old/deleted.go` | file not found |
| 2 | broken link | INDEX.md:45 | `docs/gone.md` | file not found |

N issues found. [or "Clean -- no issues."]
```

**On findings:**
- Stale refs in rules/skills: fix them now (update or remove the reference), include fixes in the commit.
- Broken INDEX.md links: fix them now.
- Missing hook scripts: report to user -- do not fix (may require investigation).
- Stale memories: report to user -- do not auto-delete.

## Rules

- **NEVER run `git add` or `git commit` directly.** Write the commit script only.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- If `make ze-verify` hasn't passed this session, run it before preparing the commit.
- If in doubt about scope, ask. The cost of asking is low; the cost of a bad commit is high.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.
- Health check fixes go into the same commit if they touch files already in scope. Otherwise, note them as a separate follow-up.
