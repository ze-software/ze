# Git Safety Rationale

Why: `ai/rules/git-safety.md`

## Why Pre-Commit Gate Exists
"Should pass" is not evidence. Run it, paste it, ask. Never commit with ANY lint issues, even pre-existing ones.

## Work Preservation Procedure
When tests fail or approach isn't working:
1. Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`
2. ASK user: "Tests failing. Options: (a) keep debugging, (b) save and try different approach, (c) revert?"
3. WAIT for response before any destructive action

## Scope Discipline
- Only include files related to the current task unless explicitly told otherwise
- Always confirm scope with `git diff --stat` before running `git commit`
- Never include unrelated changes (e.g., spec files when fixing editor bugs)

## Forge CLI Examples
```bash
gh pr list
gh pr create --title "..." --body "..."
gh issue list
gh issue create --title "..."
```

## Why `./le verify current mode full` Runs Foreground

Running foreground means the tool result IS the completion signal -- no
polling, no missed notifications, log is ready to read on return. The
two-pass strategy (cached full pass + `-race` only on changed groups) is
what keeps the common case short enough for that to be practical. How
short is not stated here: `_release` (`internal/le/job/answer.go`)
appends the real elapsed seconds to `tmp/.ze-verify-duration.txt`, and a
duration typed into a document is a claim, not a measurement.

If a previous run is still going, the admission wrapper blocks the second
invocation inside the same foreground Bash call until a slot frees. There is
no lock file: `internal/le/verify/lock/register.go` is an alias for
`internal/le/job/answer.go`, which keeps one entry per running job under
`tmp/.ze-jobs/` and admits `ZE_RUN_SLOTS` of them at a time.

Anti-patterns that look like "smart" backgrounding but break:

| Anti-pattern | What actually happens |
|--------------|-----------------------|
| `run_in_background: true` + `until pgrep -f ./le verify current mode full; do sleep 2; done` | The polling loop becomes the "running" task; you never see the completion notification for the real run |
| `run_in_background: true` + `stat -c %Y` mtime check on `tmp/ze-verify.log` | Log is written continuously during the run; mtime never "settles" reliably |
| `run_in_background: true` then assume you'll be notified | You will be, but a concurrent polling/sleep loop in Bash can swallow the notification |

There is no legitimate reason to background it. `ai/rules/git-safety.md`
("Running ./le verify current mode full") says foreground, wait, never poll, and `ai/skills/ze-verify.md`
step 2 says the same. This paragraph used to carve out an exception for
"genuinely independent work to do for >60s while it runs", contradicting the
rule it exists to explain, and enumerating "both cases" under a single bullet.
Editing the tree during a run is what makes that exception unsafe: the gate
reads the working tree, so independent work invalidates the run it is waiting
for.
