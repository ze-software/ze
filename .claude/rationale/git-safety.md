# Git Safety Rationale

Why: `.claude/rules/git-safety.md`

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

## Codeberg CLI Examples
```bash
tea pr list
tea pr create --title "..." --description "..."
tea issue list
tea issue create --title "..."
```

## Why `ze-verify-fast` Runs Foreground

`ze-verify-fast` finishes well under the 240s Bash timeout in normal cases.
Running foreground means the tool result IS the completion signal -- no
polling, no missed notifications, log is ready to read on return.

If a previous run is still going, `verify-lock.sh` blocks the second
invocation inside the same foreground Bash call until the lock releases.

Anti-patterns that look like "smart" backgrounding but break:

| Anti-pattern | What actually happens |
|--------------|-----------------------|
| `run_in_background: true` + `until pgrep -f ze-verify-fast; do sleep 2; done` | The polling loop becomes the "running" task; you never see the completion notification for the real run |
| `run_in_background: true` + `stat -c %Y` mtime check on `tmp/ze-verify.log` | Log is written continuously during the run; mtime never "settles" reliably |
| `run_in_background: true` then assume you'll be notified | You will be, but a concurrent polling/sleep loop in Bash can swallow the notification |

Legitimate reasons to background `ze-verify-fast`:
- Genuinely independent work to do for >60s while it runs (rare).
- Run expected to exceed 240s (use full `make ze-verify`, not fast).

In both cases: launch with `run_in_background: true` and **stop**. No
polling loop. Ever.
