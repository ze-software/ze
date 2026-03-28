# 446 — Feature Inventory and .ci Coverage Gaps

## Objective

Create a comprehensive feature inventory cross-referenced against .ci functional test coverage to identify gaps, and promote `ze status` to a top-level command.

## Decisions

- Features document (`docs/features.md`) organized by: protocol (families, capabilities, attributes), configuration, plugins, CLI commands, API commands
- Coverage analysis (`docs/ci-test-coverage.md`) groups gaps by: CLI commands, API commands, config behavior, plugin behavior — with priority tiers
- `ze status` promoted from `ze signal status` to top-level — status checks daemon liveness via PID file, doesn't need socket (daemon may be down)

## Patterns

- Wire protocol features (encode/decode/NLRI) have 100% .ci coverage — naturally exercised through `ze bgp encode/decode`
- Operational features (peer management, RIB queries, commit workflows) have ~16% .ci coverage — require running daemon, harder to test
- Moving a subcommand to top-level touches: dispatch table, usage text, suggest list, bash completion, zsh completion, completion tests, unit tests — all must be updated together

## Gotchas

- Initial response to "ze signal status should be ze status" was to change documentation instead of code — twice. Documentation must match code, not the other way around.
- Completion scripts (bash.go, zsh.go) and their tests maintain independent command lists that must stay in sync with the dispatch table
- 42 features lack .ci tests. The biggest gap is API runtime commands (peer list/show, RIB queries, subscribe, commit) — the commands operators use daily

## Files

- `docs/features.md` — comprehensive feature inventory
- `docs/ci-test-coverage.md` — gap analysis with priority tiers
- `cmd/ze/main.go` — added `status` top-level dispatch
- `cmd/ze/signal/main.go` — moved status to `RunStatus()`, removed from `Run()`
- `cmd/ze/completion/bash.go`, `zsh.go` — updated command lists
