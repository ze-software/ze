---
name: ze-commit
description: Scoped Commit
---

# Scoped Commit

Prepare a commit script and run it immediately. No verification, no health
checks, no late completeness review. Does NOT run git commit or git add as
direct tool calls -- everything goes through the script.

See also: `/ze-commit-check` (commit with verification only when needed)

## Fast Path

1. **Commit scope:** Use the user's request and current session context. If the
   exact files are known, do not rediscover them. If not, inspect only a concise
   changed-file list needed to avoid staging unrelated work. Ask one narrow
   question only when files cannot be safely classified.
2. **Draft commit message:** Base the subject and body on the scoped changes.
   Do not run `git log` just to imitate style unless the user explicitly asks.
3. **Generate commit script:** Use `scripts/dev/commit_helper.py create` so the
   session ID, message file, executable script, ignored-path checks, and
   `git commit -F` are handled consistently:

```bash
scripts/dev/commit_helper.py create \
  --replace \
  --subject "type: subject line" \
  --body "Body explaining why." \
  --file file1.go \
  --file file2.go \
  --file file3_test.go
```

4. **Run and report:** Run the generated script yourself, with `bash` and the
   path from the helper's `script=` line (never a path you built from the
   session id), then show the resulting commit SHA(s),
   the included files, commit subject/body summary, generated message file
   path, generated script path, and that verification was skipped because
   `/ze-commit` is intentionally unchecked.

## Rules

- **NEVER run `git add` or `git commit` as bare tool calls.** Route them through the commit script, then run the script by the path its `script=` line printed.
- Use `scripts/dev/commit_helper.py create` unless the commit shape cannot be expressed by the helper.
- Do not run `make ze-verify`, `make ze-verify-changed`, lint, health checks, completeness audits, recent-commit style reviews, or remaining-work scans for `/ze-commit`.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- Protect unrelated work: include only explicit paths. Ask one narrow question if scope cannot be determined from context and the concise file list.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.