# An error names no fix command

A message tells the operator something failed, and maybe guesses why, but it
never gives the command that fixes it. `ai/rules/user-facing-errors.md` requires
the problem, the reason and the exact fix command. The operator has to work
out the next step without help.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | - | `ze cli` client, `Run` in `internal/component/cli/client/main.go` (lines 370 and 392) | When credentials fail to load, or `show version` fails at the connection level, the client prints `hint: is the daemon running?` and exits 1. It is a question, not a fix: it gives no command to check the daemon or start it | open: not fixed |
