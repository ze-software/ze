# An error names no fix command

A message tells the operator something failed, and maybe guesses why, but it
never gives the command that fixes it. `ai/rules/user-facing-errors.md` requires
the problem, the reason and the exact fix command. The operator has to work
out the next step without help.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | - | `ze cli` client, `Run` in `internal/component/cli/client/main.go` (lines 370 and 392) | When credentials fail to load, or `show version` fails at the connection level, the client prints `hint: is the daemon running?` and exits 1. It is a question, not a fix: it gives no command to check the daemon or start it | open: not fixed |
| 2026-10-15 | plugin-reload-applies-or-refuses (review round 4 NOTE-3) | as112 reload, `refuse` in `internal/plugins/as112/config_tx.go`, first branch (registration restore failed) | The error wraps `sdk.ErrRunningConfigLost` and stops there: no `sdk.RunningConfigLostNext`, no next step. The engine's `Rollback:` lines now say what became of the plugin, but the operator is not told what to do about it. Pre-existing, outside the listener-loss texts the spec changed | open: not fixed |
| 2026-10-19 | lab-docker-cleanup-and-go-test-containers (S-1) | TACACS+ client, `sendToServers` in `internal/component/tacacs/client.go` (final return) | When every server fails, the returned error is `all TACACS+ servers unreachable for <purpose>`, with no cause and no fix: a wrong shared secret (`ErrBadSecret`, observed against tac_plus-ng by `TestTacPlusNGInterop`), a refused connection and a timeout read the same to the caller. Only the per-server WARN log names the cause, and neither names `tacacs.server <ip>.key` as the thing to correct | open: not fixed |
