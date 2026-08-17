---
name: ze-commit-check
description: Scoped Commit with Verification
---

# Scoped Commit with Verification

Prepare a commit script (verification only when needed) and run it yourself.
Does NOT run git commit or git add as direct tool calls -- everything goes
through the script. This is not a late implementation review.

See also: `/ze-commit` (commit without verification), `/ze-precommit-verify` (standalone verification)

## Steps

1. **Commit scope:** Use the user's request and current session context. If the
   exact files are known, do not rediscover them. If not, inspect only a concise
   changed-file list needed to avoid staging unrelated work. Ask one narrow
   question only when files cannot be safely classified.
2. **Style gate (only when the scope carries a `.go` file):** A bounded check,
   never a review. Run it, then proceed.

   ```bash
   git diff -U0 HEAD -- '*.go' | grep -nE '^\+.*\bpanic\('
   ```

   For each hit, trace the input back to its source. A state that only a Ze defect
   reaches keeps its `panic("BUG:")`. A state a peer can produce returns an error
   instead (`docs/contributing/ze-style.md`). A `panic()` a peer can reach STOPS
   the commit. Report it and wait.

   Then read the added lines once and ask three questions. What bounds each new
   loop, queue, retry, and cache? Does each new name say what the value IS rather
   than its Go type? Does each new lifecycle or paired call state its obligation
   with MUST on BOTH sides? Fix what they find before you draft the message.
3. **Verification decision:** Apply `ai/rules/git-safety.md`.
   - If the scope is only `docs/`, `ai/`, `.claude/`, `plan/`, or `README.md`
     files per the NO row, skip `ze-precommit-verify` entirely and record the skip reason.
   - Before any verify target, run `scripts/dev/verify-status.sh check`.
   - If it prints FRESH, MUST NOT run `make ze-precommit-verify` or
     `make ze-precommit-verify-changed` again. Use the reported timestamp as evidence.
   - If it prints STALE and `ze-precommit-verify` applies, run `make ze-precommit-verify` once.
     Do not substitute `make ze-precommit-verify-changed` unless the user explicitly
     requested a changed-only gate.
4. **Failure handling:** If verification fails, read
   `tmp/ze-verify-failures.log` first, report the blocking failure groups, and
   stop. Do not prepare a commit script for failing code.
5. **Draft commit message:** Base the subject and body on the scoped changes.
   Do not run `git log` just to imitate style unless the user explicitly asks.
6. **Generate commit script:** Use `scripts/dev/commit_helper.py create` so the
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

7. **Run and report:** Run the generated script yourself, with `bash` and the
   path from the helper's `script=` line (never a path you built from the
   session id), then show the resulting commit SHA(s),
   the included files, verification evidence or skip reason, commit
   subject/body summary, generated message file path, and generated script
   path.

## Rules

- **NEVER run `git add` or `git commit` as bare tool calls.** Route them through the commit script, then run the script by the path its `script=` line printed.
- Use `scripts/dev/commit_helper.py create` unless the commit shape cannot be expressed by the helper.
- Always run `scripts/dev/verify-status.sh check` before any verify target. A FRESH PASS is authoritative and forbids rerunning `make ze-precommit-verify` or `make ze-precommit-verify-changed`.
- If verification is STALE and required, run one required gate, then proceed from its result. Do not stack extra health checks or speculative gates.
- Do not run late completeness audits, health checks, recent-commit style reviews, remaining-work tables, or companion-artifact reviews unless the user explicitly asks for them.
- Step 2's style gate is the ONE exception, and it stays inside its bounds: one grep, one read of the added lines, four questions. It never grows into `/ze-review`.
- Never include spec files unless the user explicitly asks.
- Never include documentation changes unless they're part of the task.
- Protect unrelated work: include only explicit paths. Ask one narrow question if scope cannot be determined from context and the concise file list.
- Same system = one commit. Disjoint systems = separate commit scripts.
- Never suggest, ask about, or hint at committing. Complete ALL work first. The user decides when.
