# Deferrals: fixit-cli-credential-resolution

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-cli-credential-resolution (R-5) | `--user` / `-u` flag for `ze l2tp show`/`tunnel`/`session`: the shared `forwardToDaemon` (`internal/component/l2tp/cli/show.go`) calls `LoadCredentialsWithFlags("")` with a hardcoded empty user, and its three callers (`cmdShow` :18, `cmdTunnelTeardown` :26, `cmdSessionTeardown` :37) parse no flags at all -- they append `args` VERBATIM to the daemon command string. After that spec made the zefs store optional, an operator who cannot read the store can name themselves with `--user` on every other client CLI but not these three | Pre-existing gap, not work this spec chose to skip: the spec's scope was the shared resolver. Bigger than adding a flag -- `l2tp/cli` has no FlagSet outside `decode.go`, so this means introducing flag parsing in three commands without breaking arg pass-through, with its own grammar/completion/test surface. `ze.ssh.username` + `ze.ssh.password` remain a working workaround | `plan/spec-finish-l2tp.md` (work item added, with the corrected scope and the right reference pattern) | done |

Evidence for the `done` row: `clientFlags` (`internal/component/l2tp/cli/show.go`) owns
`--user`/`-u` for all three verbs and `forwardToDaemon` passes the parsed name to
`sshclient.LoadCredentialsWithFlags`. Tokens the FlagSet does not own stay the daemon's
grammar and forward unchanged. Shell completion reads
`registry.RegisterCommandFlags("l2tp show"|"l2tp tunnel"|"l2tp session")`
(`internal/component/l2tp/cli/register.go`). Tests: `show_test.go`, `register_test.go`.

