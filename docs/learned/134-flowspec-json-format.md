# 134 — FlowSpec JSON Format

## Objective

Refactor FlowSpec JSON output to use nested arrays for OR/AND grouping and consistent `=`/`!` prefix notation for bitmask fields, and remove the redundant `"string"` field.

## Decisions

- Outer array = OR groups, inner arrays = AND groups. Example: `[["=ack"], ["=cwr", "!fin"]]` = ACK OR (CWR AND NOT FIN).
- `=` prefix always for match operations (consistency with `!` for NOT).
- `+` joins multiple bits in a single AND match value (e.g., `fin+push` means both bits must be set).
- Removed `"string"` field — the JSON structure is self-documenting.

## Patterns

- `groupFlowMatches()` and `groupTCPFlagsMatches()` group FlowMatch entries by the AND bit in the operator byte.

## Gotchas

None.

## Files

- `cmd/ze/bgp/decode.go` — `groupFlowMatches()`, `groupTCPFlagsMatches()`, `formatSingleTCPFlag()`, `formatSingleFragmentFlag()`; removed `formatFlowSpecString()`
- `docs/architecture/wire/nlri-flowspec.md` — JSON format documentation updated
