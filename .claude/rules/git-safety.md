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
3. **`chmod +x tmp/commit-SESSION.sh` in the same turn you write it.** User runs it
   directly (`./tmp/commit-...sh`), not via `bash`. Same rule for every script you hand
   the user (`tmp/delete-SESSION.sh`, any ad-hoc helper): executable bit set at creation.
4. **Never follow a script path with `.`, `,`, `:`, or `)` in chat output.** The user
   copy-pastes the path straight from the response, and trailing punctuation becomes
   part of the pasted string and breaks the command. Put the path at the end of a line
   on its own, or follow it with a space and a word. Applies to every path/URL/command
   you hand the user, not just commit scripts.
5. Report what was done and what is left (including deferred).
6. The user reviews and runs `./tmp/commit-SESSION.sh` themselves.

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

**`make ze-verify-fast` (timeout 240s). Not `go test`, not any subset.**

Race detector lives in `make ze-verify` (the full sequential variant), NOT in
`ze-verify-fast`. Run `make ze-verify` before commit when touching reactor
concurrency code or any path where data races matter.

`make ze-verify-fast` must pass before presenting the commit script. Fix what fails.
Output is auto-captured to `tmp/ze-verify.log`; on failure, a short index
is written to `tmp/ze-verify-failures.log` (read this FIRST, not the full log).

```
[ ] 0. Run `scripts/dev/verify-status.sh check`. If it prints `FRESH`, the last PASS
      covered this exact tree -- skip step 1 and note the timestamp in the commit summary.
      If `STALE`, continue to step 1.
[ ] 1. Run `make ze-verify-fast` (240s timeout). On failure read
      tmp/ze-verify-failures.log first; fall back to tmp/ze-verify.log for detail.
[ ] 2. If failures caused by current work: fix them before proceeding. Re-run.
      If pre-existing failures: do not block current work. Fix them after the primary task
      completes, in a separate commit script. If fix needs >10 min, log to
      `plan/known-failures.md` so the next session picks it up.
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

### Running ze-verify in the Background (BLOCKING)

**`make ze-verify-fast` MUST be invoked foreground with a 240s timeout. Do not use
`run_in_background`. Do not write a polling loop. Do not `pgrep`/`stat`/`flock`-watch.**

`ze-verify-fast` finishes in well under the 240s Bash timeout in normal cases. Running
it foreground means the tool result IS the completion signal -- no polling needed,
no missed notifications, log is ready to read when Bash returns.

If a previous run is still going (lock held), `verify-lock.sh` blocks the second
invocation inside the same foreground Bash call until the lock releases. You still
do not need to poll.

| Anti-pattern | What actually happens | Do instead |
|--------------|-----------------------|------------|
| `run_in_background: true` + `until pgrep -f ze-verify-fast; do sleep 2; done` | The polling loop becomes the "running" task; you never see the completion notification for the real verify run | Foreground Bash call, 240s timeout, read log on return |
| `run_in_background: true` + `stat -c %Y` mtime check on `tmp/ze-verify.log` | Log is written continuously during the run; mtime never "settles" in a way the loop detects reliably | Foreground Bash call |
| `run_in_background: true` then assume you'll be notified | You will be -- but a concurrent polling/sleep loop in Bash can swallow that notification. Just run foreground | Foreground Bash call |

Only legitimate reasons to background `ze-verify-fast`:
- You genuinely have independent work to do for >60s while it runs (rare -- usually you just need the result).
- The run is expected to exceed 240s (use full `make ze-verify` in that case, not fast).

In both cases: launch with `run_in_background: true` and **stop**. Do nothing else related
to the verify run. The runtime will notify you when it exits; read the log then. No polling
loop, ever.

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
