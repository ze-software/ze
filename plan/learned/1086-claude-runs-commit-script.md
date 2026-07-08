# 1086 -- Claude Runs the Commit Script

## Context

The commit rule used to end the flow at "prepare a user-run commit
script": Claude generated `tmp/commit-<SESSION>.sh` and stopped, and a
human ran it. The point of the script was never that a human runs it --
it was atomicity. A bare `git add` from one AI tool call sits in the
shared index where a concurrent session's `git commit` can sweep it into
the wrong commit. Thomas asked to change the rule: Claude may commit, but
only through a single script that bundles add + delete + commit "in one
go and not left dangling," and Claude runs that script itself.

## Decisions

- Chose to let Claude run the commit script itself
  (`bash tmp/commit-<SESSION>.sh`) over the old "hand the script to the
  user" step. The script -- not the human -- is what makes add + delete +
  commit atomic, so nothing is left staged between tool calls.
- Kept the ban on bare `git commit`/`git add`/`git rm`/`git restore
  --staged`/`git stash` as direct AI tool calls. "Committing is allowed;
  committing outside a script is not." The script boundary is the whole
  safety mechanism and it stays.
- Made NO hook change. `check_destructive_git` in
  `.claude/hooks/pretool-bash.py` still blocks a raw `git commit` from
  Bash; running `bash tmp/commit-<SESSION>.sh` passes because the command
  string is `bash <script>`, not `git commit`. That block is exactly what
  forces the script path, so relaxing it would defeat the rule.
- Left `commit_helper.py` logic untouched: it already only generates the
  script and reports paths; who runs it is documentation policy, not code.
  Keeping this a docs-only change avoided dragging the `scripts/**`
  ze-verify gate into a governance edit.
- Kept "never suggest/ask/hint at committing -- user decides when." The
  change is the mechanism (Claude runs the script), not the trigger.
  Commits still happen when the user asks or a workflow reaches its commit
  step, not autonomously.

## Consequences

- The commit-flow reporting is now post-hoc: run the script, then report
  the resulting commit SHA(s), included files, message/script paths, and
  verification evidence. The pre-run human review of the script is gone --
  that tradeoff was explicitly requested.
- `/ze-commit`, `/ze-commit-check`, and `/ze-implement` step 16 all end in
  "run and report" instead of "present to user."
- The commit script is still the atomic boundary; verify/structural/index
  gates in `commit_helper.py` are unchanged and still apply.

## Gotchas

- Do NOT relax the hook to allow a bare `git commit` -- the block is the
  enforcement of "via a script." Running the script is the sanctioned path.
- Historical learned entries (833, 835) still say "user runs the script."
  They are a record of what was true then; this entry supersedes that
  mechanism rather than rewriting them.
- `ai/skills/ze-doc-update.md` (wiki repo, `../wiki`) still hands its
  commit script to the user. That is a separate repo with no shared-index
  concern here and was left as-is.

## Files

- `ai/rules/git-safety.md`
- `ai/rules/planning.md`
- `ai/INSTRUCTIONS.md`
- `ai/INDEX.md`
- `ai/skills/ze-commit.md`
- `ai/skills/ze-commit-check.md`
- `ai/skills/ze-implement.md`
- `plan/learned/1086-claude-runs-commit-script.md`
- `plan/learned/.counter`
