# 072 — CLI Run Merge

## Objective

Remove the standalone `ze bgp run` command and merge its functionality into `ze bgp cli --run "command"`, so there is one entry point for both interactive and single-command socket communication.

## Decisions

- `ze bgp run` removed entirely (user-approved breaking change)
- `ze bgp cli --run "peer list"` executes single command; `ze bgp cli` (no flag) launches interactive bubbletea TUI
- `run.go` deleted; `cliClient` methods moved to `cli.go` — no-layering rule: replace, don't keep both

## Patterns

- None.

## Gotchas

- Pre-existing `make functional` test failure for test C was already present before this change — not introduced here

## Files

- `cmd/ze/bgp/cli.go` — `--run` flag added, cliClient merged in
- `cmd/ze/bgp/main.go` — `run` case removed, help updated
- Deleted: `cmd/ze/bgp/run.go`, `cmd/ze/bgp/run_test.go` (merged into cli_test.go)
