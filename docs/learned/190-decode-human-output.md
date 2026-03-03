# 190 — Decode Human Output

## Objective

Change `ze bgp decode` default output from JSON to human-readable text; keep JSON accessible via `--json` flag.

## Decisions

- Default changes to human-readable; `--json` restores old behavior — matches standard CLI convention (humans first, machines opt-in).

## Patterns

- Test runner must parse flags from `.ci` command lines: added `OutputJSON` field and `parseDecodeCmdLine()` to extract `--json` before invoking decoder.

## Gotchas

- Test runner was not parsing `--json` from `.ci` files — all decode tests silently ran in JSON mode even after the default changed. Always verify test runner flag parsing when adding new flags.

## Files

- `cmd/ze/bgp/decode/` — human output formatter, --json flag
- `internal/component/config/editor/testing/` — parseDecodeCmdLine, OutputJSON field
