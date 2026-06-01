# Scoped Commit

Prepare a commit script. No verification, no health checks. Does NOT run git commit or git add directly.

See also: `/ze-commit-check` (commit with verification and health checks)

## Steps

1. **Show scope:** Run `git status` and `git diff --stat` to identify all changed files.
2. **Identify task scope:** Determine which files belong to the current task. If unclear, ask the user.
3. **Exclude unrelated changes:** If files outside the task scope are modified, explicitly list them and confirm with the user: "These files are outside the current task scope: [list]. Exclude from commit?"
4. **Completeness check:** For each changed file in scope, check whether companion
   artifacts are missing. Present a table of findings. Missing items are warnings
   (the user decides whether to address them now or proceed).

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
   | New exported Go symbol | at least one non-test caller |

   ```
   ## Completeness Check

   | # | Expected | Status |
   |---|----------|--------|
   | 1 | docs/features.md entry | missing |
   | 2 | functional test | included |

   All present. [or table above]
   ```

5. **Check recent commits:** Run `git log --oneline -5` to match commit message style.
6. **Draft commit message:** Based on the actual changes (not the spec), write a concise subject and body.
7. **Lesson check:** If the commit changes agent workflow, rules, tooling, verification, discovery paths, or a reusable gotcha, write a `plan/learned/NNN-<name>.md` summary, bump `plan/learned/.counter`, and include both files. If no reusable lesson is useful, pass `--lesson-not-needed "<reason>"` to the helper.
8. **Generate commit script:** Use `scripts/dev/commit_helper.py create` so the session ID, message file, executable script, ignored-path checks, `git commit -F`, and lesson gate are handled consistently:

```bash
scripts/dev/commit_helper.py create \
  --replace \
  --subject "type: subject line" \
  --body "Body explaining why." \
  --file file1.go \
  --file file2.go \
  --file file3_test.go
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

   Nothing remaining. [or table above]
   ```

   Sources to check:
   - **Spec ACs:** if work was driven by a spec, list any AC-N not covered by this commit
   - **Deferrals:** open items in `plan/deferrals.md` related to this work
   - **TODOs in code:** any TODO/FIXME added or pre-existing in the changed files
   - **Uncommitted files:** files modified but excluded from this commit scope
   - **Known gaps:** anything mentioned during the session as "not yet done"

   If nothing remains, say "Nothing remaining." Do not skip this table.

10. **Present to user:** Show the completeness check, remaining work table, then the staged files, commit message file, and generated script path. The user runs the script themselves.

## Rules

- **NEVER run `git add` or `git commit` directly.** Write the commit script only.
- Use `scripts/dev/commit_helper.py create` unless the commit shape cannot be expressed by the helper.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- If in doubt about scope, ask. The cost of asking is low; the cost of a bad commit is high.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.
