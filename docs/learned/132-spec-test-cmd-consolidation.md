# 132 — Test Command Consolidation

## Objective

Consolidate test commands into `cmd/ze-test` (subcommand dispatch: `run`, `syslog`) and move `ze-peer` from `test/cmd/` to `cmd/`, deleting obsolete tools (`migrate-ci`, `ci-fix-order`).

## Decisions

Mechanical refactor, no design decisions.

## Patterns

- Libraries (`internal/test/runner`, `internal/test/syslog`, `internal/test/peer`) remained unchanged — only the cmd entry points moved. This is the correct boundary: cmd packages are thin wrappers around library packages.
- Subcommand pattern (`ze-test run`, `ze-test syslog`) is cleaner than two separate binaries sharing no code.

## Gotchas

None.

## Files

- `cmd/ze-test/main.go`, `cmd/ze-test/run.go`, `cmd/ze-test/syslog.go` — new unified test runner
- `cmd/ze-peer/main.go` — moved from `test/cmd/ze-peer/`
- `test/cmd/` — deleted entirely
- `Makefile` — functional targets updated
