# 502 -- Signal Status Command

## Context

The documentation claimed SIGUSR1 dumps status, and the YANG schema had a `daemon status` RPC registered, but there was no `ze signal status` CLI command. Additionally, the SIGUSR1 `OnStatus` callback was never wired in production (always nil, making SIGUSR1 a no-op). The `ze signal` CLI also had hardcoded help/dispatch and was sending wrong exec strings: "reload" and "quit" instead of "daemon reload" and "daemon quit" (the actual YANG dispatch keys). Shell completions were missing `restart` and `status`.

## Decisions

- Added `ze signal status` sending `daemon status` via SSH exec (RPC path), not SIGUSR1 directly. Consistent with how `ze signal reload` sends an RPC, not SIGHUP.
- Replaced hardcoded switch+help with a `commands` registry slice. Help text, dispatch, suggestions, and completion tests all derive from this list.
- Made the registry unexported (`commands`) with a `Commands()` accessor returning a copy, over an exported `var`, to prevent callers from mutating the internal list.
- Fixed exec strings: `reload` now sends `daemon reload`, `quit` sends `daemon quit`. The previous strings were silently failing at the dispatcher.
- Wired `OnStatus` in reactor's `startSignalHandler` to log status via slog (uptime, peers, router-id, local-as, start-time), making SIGUSR1 functional in production.
- Fixed nushell extern declarations from wrong `config?: path` to correct `--host`/`--port` flags (pre-existing bug on all signal entries).

## Consequences

- `ze signal reload` and `ze signal quit` now actually work (were broken since introduction).
- SIGUSR1 now logs status to stderr via slog in production.
- Shell completions now include all five signal subcommands: reload, stop, restart, status, quit.

## Gotchas

- `stop` and `restart` are handled by SSH server hardcoded checks before the dispatcher, so they send bare strings ("stop", "restart"). The other three go through the dispatcher and need the full YANG dispatch key ("daemon reload", "daemon status", "daemon quit").
- Pre-existing: `resolveHost`/`resolvePort` use `os.Getenv` for ze-prefixed vars instead of `env.Get()`. Left for a separate commit.

## Files

- `cmd/ze/signal/main.go` -- registry-based dispatch, added status command, fixed exec strings
- `cmd/ze/signal/main_test.go` -- registry, exec mapping, defensive copy tests
- `cmd/ze/completion/{bash,fish,zsh,nushell}.go` -- added restart + status to all shells, fixed nushell extern params
- `cmd/ze/completion/main_test.go` -- signal subcommand verification
- `internal/component/bgp/reactor/reactor.go` -- wired OnStatus callback
- `docs/guide/operations.md`, `docs/guide/command-reference.md`, `docs/features/cli-commands.md`, `docs/architecture/behavior/signals.md` -- added status command, fixed signal table
