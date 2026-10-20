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
| 2026-10-20 | exabgp-6-api-compatibility (AC-4) | config parser, `parse` in `internal/component/config/parser.go` (line 95), reached by `ze exabgp migrate` | A `}` that closes no section (old-5.0 `conf-extended-attributes.conf` line 36) is refused as `line 36: expected keyword, got RBRACE`: a lexer token name, no reason, no fix. ExaBGP 6.0 says "this '}' closes no section" | open: name the stray brace and say to delete it |
| 2026-10-20 | - (plugin dependency census) | `config.LoadConfig` unknown-field refusal, `LoadBackend` "unknown backend" (iface, traffic, firewall), `ErrMissingDependency` | Read, not run: a feature compiled out of this build causes each refusal. None names the `ze_` tag or the fix (rebuild with the tag, or remove the config). The operator cannot tell a typo from a missing feature | open: name the missing tag and both fixes |
