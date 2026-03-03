# 139 — ze-test Rename

## Objective

Rename `ze-test run` → `ze-test bgp` and the subcommands `encoding/decoding/parsing` → `encode/decode/parse` for consistency.

## Decisions

Mechanical rename, no design decisions.

## Patterns

None.

## Gotchas

None.

## Files

- `cmd/ze-test/main.go` — `run` → `bgp` in switch
- `cmd/ze-test/run.go` → `cmd/ze-test/bgp.go` — renamed and updated
- `Makefile` — 4 target names updated
