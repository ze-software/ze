# Git Safety

**When:** Read before any git operation or writing a commit script; covers the AI-tool git bans, the user-run commit-script path, and verify-status handling.

Rationale: `ai/rationale/git-safety.md`

## Commit Rules

**FORBIDDEN from AI tool calls:** `git commit`, `git add`, `git rm`,
`git restore --staged`, `git stash`. Sessions share staging -- a direct
`git add` is visible to every other session's `git commit` and files
cross-commit. Package add + commit into a single user-triggered script.

**Explicit commit requests are a fast path.** When the user asks for a
commit, the implementation/review phase is over. Prepare the user-run
commit script immediately. Do not re-audit the implementation, run late
completeness/remaining-work tables, inspect speculative companion artifacts,
or rerun lint/tests just because commit was requested. Inspect only enough
state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
If scope is ambiguous, ask one narrow question; otherwise proceed.

**Commit workflow:**
1. Use `scripts/dev/commit_helper.py session` to create or reuse the 8-char session ID stored in `tmp/commit-session-id`.
2. Use `scripts/dev/commit_helper.py create` to write `tmp/commit-msg-<SESSION>-<tag>.txt` and `tmp/commit-<SESSION>.sh`. Pass `--file` once per explicit file, `--remove` for tracked deletions, `--replace` for the first logical commit, and `--append` for later commits in the same user-run script.
3. The helper writes executable scripts, uses `git commit -F <message-file>`, rejects ignored/generated paths, and refuses to overwrite an existing script unless `--replace` or `--append` is explicit.
4. Lesson learned check: when a commit changes agent workflow, rules, tooling, verification, or discovery surfaces, include `plan/learned/NNN-<name>.md` and `plan/learned/.counter` in `--file`. If no reusable lesson is useful, pass `--lesson-not-needed "<reason>"`. For known-required lessons, pass `--lesson-required`.
5. If the helper cannot express the commit shape, hand-write the same `tmp/commit-<SESSION>.sh` pattern and `chmod +x` it. Do not use heredocs. Always use `git commit -F <file>`.
6. Never end an output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. Put path on its own line or follow with a space.
7. Report included files, message file, script path, and verification evidence or skip reason. Do not add a late completeness or remaining-work review unless the user explicitly asked for one.
8. Before writing a commit script, read `.gitignore` and never `git add` ignored paths. Key ignored paths: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/`, `.codex/skills/`, `.agents/skills/`, `tmp/`, `/bin/`. Only add canonical sources (e.g., `ai/skills/`, `ai/INSTRUCTIONS.md`).

`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when the user
runs it. `git restore --staged <file>` is allowed inside a commit
script only; all other `git restore` variants remain forbidden.

**`git rm` safety:** before using `git rm` in a commit script, verify
the file is tracked (`git ls-files --error-unmatch <file>`). For files
modified during implementation (specs, stubs), use `git rm -f` to avoid
"has local modifications" errors. Never `git rm -f` without first
committing the file's current state (see Spec Closure in planning rules).

**Helper format:**
```bash
# Single commit (most common):
scripts/dev/commit_helper.py create \
  --replace \
  --subject "hook: allow tee pipe, per-session log paths" \
  --body "Explanation of why the change was made." \
  --file .claude/hooks/pretool-bash.py \
  --file ai/rules/bash-output.md \
  --lesson-not-needed "hook fix, no novel pattern"

# Second commit in the same script:
scripts/dev/commit_helper.py create \
  --append \
  --subject "feat: add widget support" \
  --body "Implements widget rendering for the dashboard." \
  --file internal/component/web/widget.go \
  --file internal/component/web/widget_test.go

# Spec closure (remove spec file):
scripts/dev/commit_helper.py create \
  --append \
  --subject "spec: close spec-widget" \
  --remove plan/spec-widget.md

# With a learned summary:
scripts/dev/commit_helper.py create \
  --replace \
  --subject "rules: add goroutine lifecycle rule" \
  --file ai/rules/goroutine-lifecycle.md \
  --file plan/learned/042-goroutine-lifecycle.md \
  --file plan/learned/.counter
```

Key flags: `--replace` for the first commit in a session, `--append`
for subsequent commits. `--file` per path to add, `--remove` per
tracked path to delete. `--lesson-not-needed "<reason>"` when no
learned summary applies; `--lesson-required` to enforce one.
Body lines are wrapped to 72 characters. Subjects are single-line and
must be at most 72 characters.

The generated script has this shape:
```bash
#!/bin/bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Commit a: type: subject line
# Lesson: not needed - hook fix, no novel pattern
git add -- \
  file1.go \
  file2.go \
  file3_test.go
git commit -F tmp/commit-msg-<SESSION>-a.txt
```

**Never suggest / ask / hint at committing.** Complete ALL work first
(testing, spec, docs, learned summary), then report. User decides.
Banned phrases: "ready to commit?", "shall I commit?", "we could
commit now", "want me to commit?".

**Never bypass hooks.** No `--no-verify`, no `--no-gpg-sign`. On GPG
failure (`gpg failed to sign` / `cannot open /dev/tty`), ask the user
to run `! echo test | gpg --clearsign` to unlock the agent, then
re-run the script.

## Commit Granularity

Single-focus commits: one logical change per commit. Same system =
one commit (feature + tests + docs). Multiple unrelated changes =
multiple commits, not one bundle. Unrelated bug fix = separate commit.
Review fixes from a review pass = one commit.

## Before Any Commit

### Step 0: Does `ze-verify` apply?

BLOCKING only when the commit could plausibly affect build, tests, or
generated code.

| Files in commit | Run `ze-verify`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `Makefile`, `scripts/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

Mixed commit: one YES row -> run. Do not split a commit to skip.
Decision rule: "could this make a Go test fail or break the build?"
No = skip and note in commit summary. Unsure = run.

### Step 1: If `ze-verify` applies (BLOCKING)

`make ze-verify` (timeout 240s). Not `go test`, not any subset.
Before any verify target, check freshness. A FRESH status covers the
byte-identical tree and forbids rerunning `make ze-verify` or
`make ze-verify-changed`. `ze-verify` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `ze-verify-changed` scopes to packages with uncommitted
`.go` changes PLUS packages committed since the last green verify
(`scripts/dev/changed-pkgs.sh`, baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. For reactor concurrency changes, also run `make
ze-race-reactor`. Output writes: `tmp/ze-verify.log`, per-stage logs
under `tmp/verify/`, `tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`.

```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> MUST NOT run `make ze-verify` or `make ze-verify-changed` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `make ze-verify` (240s) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open that group's `tmp/verify/<nn>-<stage>.log`.
[ ] 2. Failure from current work: fix + re-run. Pre-existing: fix after primary task in separate commit; if >10 min, log to `plan/known-failures.md`.
```

### Concurrent Verify Runs (BLOCKING)

One `make ze-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. All variants are wrapped by
`scripts/dev/verify-lock.sh` (`flock` on `tmp/.ze-verify.lock`); a
second invocation blocks automatically.

| Do | Don't |
|----|-------|
| Let the second invocation block | Kill the running verify |
| If the run is yours (same tree), read `tmp/ze-verify.log` instead of re-running | Delete the lockfile |
| If "waiting for lock" appears, do other work | Start `go test` / `golangci-lint` / `bin/ze-test` in parallel (bypasses lock) |

Lock releases when the command exits. `flock` is fd-backed, not
PID-backed -- no cleanup after a crash.

### Running ze-verify

Foreground with 240s timeout. No background execution, no polling
loops. Wait for completion.

### Step 2: Always

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Learned summary written to plan/learned/NNN-<name>.md (NNN from .counter)
      [ ] plan/learned/.counter bumped to NNN+1
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left.
```

Never commit with lint issues. Never commit without test evidence when code changed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`,
`git stash drop`, `git push --force`.

## Branch Changes Are Forbidden

Stay on the branch you started on. Never change branches, create
branches, delete branches, rename branches, or integrate branches from
an AI tool call.

Forbidden branch-changing commands include `git switch`, `git checkout
<branch>`, `git branch`, `git rebase`, `git merge`, and
`git cherry-pick`.

If branch movement or integration is needed, stop and ask the user to do
it manually.

## Before Destructive Actions

Save: `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Forbidden Raw Output

`git diff --stat` / `git status` dumped raw in output -- summarise.

## Branch Integration

When the user integrates a worktree branch manually, it lands on main
via `git rebase <branch>`, never `git merge`. Linear history.

## GPG Signing

Never `--no-gpg-sign` / `-c commit.gpgsign=false`.
Never `--no-verify`.

On `gpg failed to sign` / `cannot open /dev/tty`, ask user to unlock
the agent, then retry.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
