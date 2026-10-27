# Help text describes superseded behaviour

A command changes what it accepts, and its help line, completion hint and help
page are written in other files, so they keep describing the old command. The
reader learns the command from the help and types what it says, and the
command refuses it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | terminal-demo-showcase | SSH config editor help: `internal/component/cli/model_render.go` (help overlay), `internal/component/cli/completer.go` (discard ShortHelp), `internal/component/config/cli/cmd_edit.go` (edit help page) | All three say discard means "Revert all changes". In a session, `discard` alone is refused with errDiscardRequiresPathOrAllIn (`cmdDiscardSession`, model_commands_commit.go) and takes `all` or a path; only the no-session editor (`cmdDiscard`) reverts everything. The refusal also names no fix command. Not one line: the wording differs by mode | open: not fixed |
| 2026-10-17 | - | `rpc dns-cache` description in `internal/component/cmd/clear/yang/ze-cli-clear-api.yang` | It says the hit and miss counters keep their values, while `cache.Clear()` (`internal/component/resolve/dns/cache.go`), which `handleClearDNSCache` reaches, zeroes all four, as the command YANG says. It also names the bare form, which now needs `all` | open: not fixed. Say all four counters reset, and name `clear dns cache all` |
| 2026-10-27 | lan-dns-resolver | `le mock dns`, `Run` in `internal/test/mock/dns/dns.go` | `./le help mock dns` says a trailing help word asks the tool for its own help; `./le mock dns help` starts the mock and serves until killed, because `Run` answers only `--help`. Other forwarded harness tools not checked | open: phase 2 of lan-dns-resolver edits that file for its new flags and answers the help word there, with a test |
