# 869 -- ze-test one-based ids

## Context

`ze-test ... --list` used a one-based progress column next to a zero-based runnable id. Operators saw entries like `1/N 0 name`, which made `--start` and positional selection look off by one.

## Decisions

- Changed generated test ids to start at `1`, so the runnable selector matches the progress number shown by list and run output.
- Kept numeric ids as the only generated id format. No letter short-code phase.
- Kept `N/TOTAL id name` in `--list`, because it matches per-test run output and makes interrupted-run resume points easy to copy.
- Replaced printf-style list/help output in touched files with `textbuf.Buffer`, `os.Stdout.Write`, and `os.Stderr.WriteString` to match Ze allocation and output rules.

## Consequences

- `ze-test bgp plugin --list` now starts with `1/N 1 first-test`.
- Existing scripts or docs that selected test `0` must use `1`.
- `--start N` now has the same user-visible number as the progress line `N/TOTAL`.

## Gotchas

- Tests that build trees manually must use `SetSlice("name-server", ...)` for leaf-lists and `AddListEntry` only for keyed lists. A unit test can otherwise mask a web display bug.
- Avoid `fmt.Fprintf` and `fmt.Printf` in test runner output, even in cold CLI paths, unless a rule explicitly allows it.

## Files

- `internal/test/runner/record.go` -- one-based `GenerateNick`
- `internal/test/runner/base.go` -- shared list rendering helpers
- `internal/test/runner/record_collection.go` -- generic record list output
- `internal/test/cli/*.go` -- one-based help examples
- `docs/functional-tests.md` -- selector contract update
- `ai/rules/testing.md` -- agent-facing selector contract
