# 122 — FlowSpec Command-Style String Output

## Objective

Change FlowSpec `String()` output from diagnostic format (`flowspec(dest-prefix=10.0.0.0/24 dest-port[=80])`) to API command syntax (`flowspec destination 10.0.0.0/24 destination-port =80`), enabling round-trip parsing.

## Decisions

- Chose to add `ComponentString()` on `FlowSpec` to emit components without the `flowspec` prefix, so `FlowSpecVPN.String()` can embed it without duplicating the prefix.

## Patterns

- Numeric components must NOT output `&` AND prefix — the parser infers AND from position. Only bitmask components (TCP flags, fragment) use `&` to join values.
- Protocol component is a special case: the parser accepts bare numerics (`6`) or named values (`tcp`) but NOT operator-prefixed (`=6`). Protocol output must omit the `=` prefix.

## Gotchas

- Initial implementation output `&<=65535` for numeric AND — wrong. Parser doesn't handle `&` prefix for numeric operators. Silent value dropping resulted. Fixed by removing `&` prefix entirely from numeric output.
- Protocol component: `=6` output breaks the parser, which uses custom logic for protocol parsing. Must output plain `6` without operator prefix.

## Files

- `internal/bgp/nlri/flowspec.go` — `String()`, `ComponentString()`, `bitmaskString()`, helper functions
- `internal/bgp/nlri/flowspec_test.go` — comprehensive TDD tests including round-trip
