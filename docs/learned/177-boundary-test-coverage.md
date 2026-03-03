# 177 — Boundary Test Coverage

## Objective

Audit and fill gaps in boundary tests for numeric inputs following a TDD rule update; add missing tests without modifying existing validation code.

## Decisions

- Most boundary tests already existed — only IPv6 prefix length 129 was missing. Adding one test was sufficient.
- Functional tests not applicable for boundary validation — invalid values are rejected at parse time before reaching functional test scope.
- 65536 boundaries for uint16 fields (message length, hold time) are prevented at the type level; `TypeUint16` schema test covers parser-level rejection.

## Patterns

- Boundary audit before writing tests — grep for existing coverage first, then only fill gaps.
- Spec is test-coverage-only: implementation code already correct, spec documents that tests verify existing behavior.

## Gotchas

- None.

## Files

- `internal/plugin/bgp/nlri/inet_test.go` — added `TestINETPrefixLengthBoundary` (IPv4 32/33, IPv6 128/129)
