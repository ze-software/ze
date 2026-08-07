---
kind: table
level:
stage:
---
| Dispatcher | Runs on | Contains |
|---|---|---|
| `.claude/hooks/pretool-bash.py` | PreToolUse `Bash` | every Bash check below |
| `.claude/hooks/pretool-writeedit.py` | PreToolUse `Write\|Edit\|MultiEdit\|NotebookEdit` | every Write/Edit check below |
| `.claude/hooks/pretool-agent-skill.py` | PreToolUse `Task\|Agent` | two gates: skills-over-raw-agents (`ai/rules/cli.md`), and review-runs-on-Opus-5 (`ai/rules/planning.md`) |
| `.claude/hooks/posttool-writeedit.py` | PostToolUse `Write\|Edit` | the formatters (gofmt/goimports/golangci, ruff) + cheap advisory checks |
