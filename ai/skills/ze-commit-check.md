---
name: ze-commit-check
description: Scoped Commit with Verification
---

# Scoped Commit with Verification

Prepare a commit script (verification only when needed) and run it yourself.
Does NOT run git commit or git add as direct tool calls -- everything goes
through the script. This is not a late implementation review.

See also: `/ze-commit` (commit without verification), `/ze-verify` (standalone verification)

## Steps

1. **Commit scope:** Use the user's request and current session context. If the
   exact files are known, do not rediscover them. If not, inspect only a concise
   changed-file list needed to avoid staging unrelated work. Ask one narrow
   question only when files cannot be safely classified.
2. **Verification decision:** Apply `ai/rules/git-safety.md`.
   - If the scope is only `docs/`, `ai/`, `.claude/`, `plan/`, or `README.md`
     files per the NO row, skip `ze-verify` entirely and record the skip reason.
   - Before any verify target, run `scripts/dev/verify-status.sh check`.
   - If it prints FRESH, MUST NOT run `make ze-verify` or
     `make ze-verify-changed` again. Use the reported timestamp as evidence.
   - If it prints STALE and `ze-verify` applies, run `make ze-verify` once.
     Do not substitute `make ze-verify-changed` unless the user explicitly
     requested a changed-only gate.
3. **Failure handling:** If verification fails, read
   `tmp/ze-verify-failures.log` first, report the blocking failure groups, and
   stop. Do not prepare a commit script for failing code.
4. **Draft commit message:** Base the subject and body on the scoped changes.
   Do not run `git log` just to imitate style unless the user explicitly asks.
5. **Lesson check:** If the commit ADDS content to agent workflow, rules, tooling,
   verification, or discovery paths -- not merely moves, renames, or reformats it --
   reserve the number and create the file with
   `python3 scripts/dev/commit_helper.py learned-next <name>`, write the
   `plan/learned/NNN-<name>.md` summary into it, and `--file` it. If the added
   content taught nothing reusable, pass `--lesson-not-needed "<reason>"` instead.
   A summary records a lesson; it is not an artifact of committing.
6. **Generate commit script:** Use `scripts/dev/commit_helper.py create` so the
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

7. **Run and report:** Run the generated script yourself
   (`bash tmp/commit-<SESSION>.sh`), then show the resulting commit SHA(s),
   the included files, verification evidence or skip reason, commit
   subject/body summary, generated message file path, and generated script
   path.

## Rules

- **NEVER run `git add` or `git commit` as bare tool calls.** Route them through the commit script, then run the script (`bash tmp/commit-<SESSION>.sh`).
- Use `scripts/dev/commit_helper.py create` unless the commit shape cannot be expressed by the helper.
- Always run `scripts/dev/verify-status.sh check` before any verify target. A FRESH PASS is authoritative and forbids rerunning `make ze-verify` or `make ze-verify-changed`.
- If verification is STALE and required, run one required gate, then proceed from its result. Do not stack extra health checks or speculative gates.
- Do not run late completeness audits, health checks, recent-commit style reviews, remaining-work tables, or companion-artifact reviews unless the user explicitly asks for them.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- Protect unrelated work: include only explicit paths. Ask one narrow question if scope cannot be determined from context and the concise file list.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.
