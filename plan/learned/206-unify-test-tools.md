# 206 — Unify Test Tools

## Objective

Consolidate `ze-peer` and `ze config test` under a single `ze-test` binary with subcommands (`ze-test peer`, `ze-test editor`).

## Decisions

- `--server` debug mode uses the peer library directly (not a subprocess `go build`) — eliminates 2-3 second build delay per debug session; binary already has the peer code.
- Single binary preferred over two separate tools — reduces build surface and deployment artifacts.

## Patterns

- None beyond consolidation.

## Gotchas

- Flag aliases via `fs.String()` called twice for same flag name caused a panic — use a single flag with multiple names via a custom `Value` implementation.
- Exit code was wrong (always 0) — test runner must propagate peer's exit code explicitly.
- Signal handler not cleaned up after peer exits — leaked goroutine; always defer signal handler removal when using `signal.Notify`.
- Four implementation bugs total; unified tool was harder to test than the two separate tools it replaced.

## Files

- `cmd/ze-test/` — unified test binary
- `cmd/ze-test/peer/` — peer subcommand (was ze-peer)
- `cmd/ze-test/editor/` — editor subcommand (was ze config test)
