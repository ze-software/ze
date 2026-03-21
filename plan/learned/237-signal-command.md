# 237 — Signal Command

## Objective

Implement PID file management and `ze signal` CLI for sending signals (reload, shutdown) to a running ze instance.

## Decisions

- PID file location cascade: `XDG_RUNTIME_DIR` / `/var/run/ze` / `/tmp/ze` — mirrors `DefaultSocketPath` cascade.
- `acquirePIDFile` returns error on lock failure (fatal); prevents duplicate instances.
- `pidfile.Noop()` added for stdin/skip cases to satisfy `nilnil` linter.
- .ci functional tests cannot verify filesystem state or run `ze signal` mid-test — 4/5 functional tests covered by unit tests.

## Patterns

- PID file lock failure is fatal (not silently ignored) — fail-safe over degraded operation.
- `Noop()` sentinel satisfies interface for cases where no PID file is needed.

## Gotchas

- .ci format limitation: cannot observe filesystem state (PID files) or orchestrate two ze processes; use unit tests for those cases.

## Files

- `internal/pidfile/` — PID file acquire/release/noop
- `cmd/ze/signal/` — `ze signal` CLI subcommand
