# Help text describes superseded behaviour

A command changes what it accepts, and its help line, completion hint and help
page are written in other files, so they keep describing the old command. The
reader learns the command from the help and types what it says, and the
command refuses it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | terminal-demo-showcase | SSH config editor help: `internal/component/cli/model_render.go` (help overlay), `internal/component/cli/completer.go` (discard ShortHelp), `internal/component/config/cli/cmd_edit.go` (edit help page) | All three say discard means "Revert all changes". In a session, `discard` alone is refused with errDiscardRequiresPathOrAllIn (`cmdDiscardSession`, model_commands_commit.go) and takes `all` or a path; only the no-session editor (`cmdDiscard`) reverts everything. The refusal also names no fix command. Not one line: the wording differs by mode | not fixed |
