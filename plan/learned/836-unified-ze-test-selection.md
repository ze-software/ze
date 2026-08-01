# 836 -- Unified ze-test Selection

## Context

Functional test runners had different selection and progress conventions. Some suites used BGP subcommands with ids, top-level `.ci` suites used their own path, editor and web tests had separate defaults, and ExaBGP compatibility still ran through a Python script outside `ze-test`.

## Decisions

- Chose one shared `runner.Selection` contract for `--list`, `--all`, `--start ID`, `--pattern TEXT`, and positional ids or names.
- Chose decimal test ids and `N/TOTAL` progress for every listed and completed test line.
- Chose per-test completion lines plus periodic progress from the shared display path for `.ci` suites and the new ExaBGP runner.
- Chose to keep the legacy Python ExaBGP runner as a reference, but make `make ze-exabgp-test` call `bin/ze-test exabgp --all`.

## Consequences

- Long functional runs can be resumed with `--start ID` after a timeout or interrupted run.
- Agents and humans can learn one test syntax and apply it to BGP, top-level `.ci`, editor, web, VPP, and ExaBGP compatibility suites.
- ExaBGP compatibility benefits from Ze's normal test display, list output, parallelism, and make target wiring.

## Gotchas

- `N/TOTAL` is the run position, not the selector. The selector is the printed id.
- ExaBGP fixtures still depend on the repository Python helper scripts and `paramiko`; the Go runner orchestrates them rather than reimplementing the BGP mock protocol.
- Commit scripts for this area should include the learned summary because test tooling and discovery surfaces changed.

## Files

- `internal/test/runner/selection.go`
- `internal/test/runner/display.go`
- `internal/test/cli/cmd_exabgp.go`
- `Makefile`
- `mk/test-functional.mk`
- `docs/functional-tests.md`
- `docs/contributing/testing.md`
- `ai/patterns/functional-test.md`
- `ai/rules/testing.md`
