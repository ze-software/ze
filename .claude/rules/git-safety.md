# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

**NEVER run `git commit` or `git push`.** These commands are in the deny list and blocked by hooks.

**Commit workflow:**
1. **Do NOT run `git add`.** Multiple Claude sessions share the staging area; `git add`
   from one session contaminates others.
2. Write a commit script to `tmp/commit-SESSION.sh` where SESSION = 8-char unique ID
   (e.g., `head -c4 /dev/urandom | xxd -p` at session start). The script contains
   the `git add` commands, the commit message as a heredoc, and the `git commit` command.
3. Report what was done and what is left (including deferred).
4. The user reviews and runs `bash tmp/commit-SESSION.sh` themselves.

**Script format:**
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

**Unstaging files:** `git restore --staged <file>` is allowed. All other `git restore` variants are forbidden.

**Never suggest, ask about, or hint at committing.** Complete ALL work first (testing, spec,
docs, learned summary). Then report what was done and what is left (including deferred).
The user decides when to commit. Banned phrases: "ready to commit?", "shall I commit?",
"we could commit now", "want me to commit?".

## Commit Granularity

Changes to the same system go in one commit. Disjoint systems (e.g., CLI and BGP encoding)
get separate commits. A feature + its tests + its docs = one commit. A bug fix in an
unrelated package = separate commit.

## Before Any Commit

### Step 0: Decide whether `ze-verify` applies (THINK FIRST)

`make ze-verify` exists to catch code regressions. It is BLOCKING **only when the
commit could plausibly affect the build, tests, or generated code**. Apply it to
the actual files being committed, not as a reflex.

| Files in the commit | Run `ze-verify`? |
|---------------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES, BLOCKING |
| `Makefile`, `scripts/**`, build config, CI config | YES, BLOCKING |
| `*.yang`, generated code, codegen templates | YES, BLOCKING |
| Anything that runs at build time or affects a binary | YES, BLOCKING |
| `.claude/**/*.md` (rules, patterns, rationales, hooks docs) | NO, skip |
| `plan/**/*.md` (specs, learned summaries) | NO, skip |
| `docs/**/*.md`, `README.md`, other prose-only markdown | NO, skip |
| Pure docs/rule edits with zero code touched | NO, skip |

**Mixed commit:** if even one file in the commit falls in a YES row, run
`ze-verify`. Do not split a commit just to skip verification.

**Reasoning rule:** ask "could this change make a Go test fail or break the
build?" If the honest answer is "no, it is impossible," skip `ze-verify` and say
so in the commit summary. If unsure, run it.

### Step 1: If `ze-verify` applies (BLOCKING)

**`make ze-verify-fast` (timeout 180s). Not `go test`, not any subset.**

`make ze-verify-fast` must pass before presenting the commit script. Fix what fails.
Output is auto-captured to `tmp/ze-verify.log`; on failure, a short index
is written to `tmp/ze-verify-failures.log` (read this FIRST, not the full log).

```
[ ] 0. Run `scripts/dev/verify-status.sh check`. If it prints `FRESH`, the last PASS
      covered this exact tree -- skip step 1 and note the timestamp in the commit summary.
      If `STALE`, continue to step 1.
[ ] 1. Run `make ze-verify-fast` (180s timeout). On failure read
      tmp/ze-verify-failures.log first; fall back to tmp/ze-verify.log for detail.
[ ] 2. If failures caused by current work: fix them before proceeding. Re-run.
      If pre-existing failures: do not block current work. Fix them after the primary task
      completes, in a separate commit script. If fix needs >10 min, log to
      `.claude/known-failures.md` so the next session picks it up.
```

### Concurrent Verify Runs (BLOCKING)

**Only one `make ze-verify*` (or `ze-chaos-verify`) runs at a time across the whole repo.**
Multiple parallel sessions share the build cache, ports, and `bin/ze` processes -- two
verify runs at once trash each other and make both slower than either alone.

All verify variants (`ze-verify`, `ze-verify-fast`, `ze-verify-changed`, `ze-chaos-verify`)
are wrapped by `scripts/dev/verify-lock.sh`, which acquires `tmp/.ze-verify.lock` via
`flock` before running. **A second invocation blocks automatically** until the first one
releases the lock. You do not need to poll or check the lockfile yourself.

| Do | Don't |
|----|-------|
| Invoke `make ze-verify-fast` and let it block if another is running | Kill the running verify to "skip ahead" |
| If the run is clearly yours (same source state), read `tmp/ze-verify.log` when it finishes instead of re-running | Delete `tmp/.ze-verify.lock` to bypass the wait |
| If you see the "waiting for lock" message, do other work until it releases | Start `go test`, `golangci-lint`, or `bin/ze-test` directly in parallel with a verify run (bypasses the lock) |

The lock is held only during the verify run; it releases automatically when the command
exits (success or failure). Stale locks are handled by `flock` (fd-backed, not PID-backed),
so there is no cleanup needed after a crash.

### Step 2: Always (regardless of ze-verify)

```
[ ] 3. Spec completion gate. If work was driven by a spec in plan/:
      [ ] Learned summary written to plan/learned/NNN-<name>.md
      [ ] Spec file staged for deletion (git rm)
      If not done: STOP. Do it before proceeding.
[ ] 4. Executive Summary Report (rules/planning.md). Present what was done and what is left.
      Do NOT ask to commit. The user will tell you when.
```

**Forbidden:** `git diff --stat`, `git status` dumped raw into output. Summarise for the user.
Never commit with lint issues. Never commit without test evidence when code changed.
`make ze-test` includes fuzz tests, use only when specifically needed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`, `git stash drop`, `git push --force`

## Before Destructive Actions

Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch` — then write the destructive command(s) to `tmp/delete-SESSION.sh`, tell the user, and STOP. Same pattern as commit scripts so commands aren't lost in scrolling output.

## Branch Integration: Rebase Only

When worktree branches need to land on main, always instruct the user to use
`git rebase <branch>`, not `git merge`. Linear history, no merge commits.

## GPG Signing Recovery

When `git commit` fails with "gpg failed to sign the data" / "cannot open /dev/tty",
ask the user to run `! echo test | gpg --clearsign` to unlock the GPG agent, then retry.
Never bypass signing with `--no-gpg-sign` or `-c commit.gpgsign=false`.

## Codeberg CLI

Use `tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`
