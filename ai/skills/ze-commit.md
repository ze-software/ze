# Scoped Commit

Prepare a user-run commit script immediately. No verification, no health
checks, no late completeness review. Does NOT run git commit or git add
directly.

See also: `/ze-commit-check` (commit with verification only when needed)

## Fast Path

1. **Commit scope:** Use the user's request and current session context. If the
   exact files are known, do not rediscover them. If not, inspect only a concise
   changed-file list needed to avoid staging unrelated work. Ask one narrow
   question only when files cannot be safely classified.
2. **Draft commit message:** Base the subject and body on the scoped changes.
   Do not run `git log` just to imitate style unless the user explicitly asks.
3. **Lesson check:** If the commit changes agent workflow, rules, tooling,
   verification, discovery paths, or a reusable gotcha, write a
   `plan/learned/NNN-<name>.md` summary, bump `plan/learned/.counter`, and
   include both files. If no reusable lesson is useful, pass
   `--lesson-not-needed "<reason>"` to the helper.
4. **Generate commit script:** Use `scripts/dev/commit_helper.py create` so the
   session ID, message file, executable script, ignored-path checks,
   `git commit -F`, and lesson gate are handled consistently:

```bash
scripts/dev/commit_helper.py create \
  --replace \
  --subject "type: subject line" \
  --body "Body explaining why." \
  --file file1.go \
  --file file2.go \
  --file file3_test.go
```

5. **Present to user:** Show only the included files, commit subject/body
   summary, generated message file path, generated script path, and that
   verification was skipped because `/ze-commit` is intentionally unchecked.
   The user runs the script themselves.

## Rules

- **NEVER run `git add` or `git commit` directly.** Write the commit script only.
- Use `scripts/dev/commit_helper.py create` unless the commit shape cannot be expressed by the helper.
- Do not run `make ze-verify`, `make ze-verify-changed`, lint, health checks, completeness audits, recent-commit style reviews, or remaining-work scans for `/ze-commit`.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- Protect unrelated work: include only explicit paths. Ask one narrow question if scope cannot be determined from context and the concise file list.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.