# 200 — Decode Ze Format

## Objective

Convert `ze bgp decode` output from ExaBGP legacy format to Ze native JSON format.

## Decisions

Mechanical format conversion, no design decisions. Key field changes: envelope `{"type":"bgp","bgp":{...}}`, peer flat `{"address":"..","asn":N}`, AS_PATH becomes a simple integer array, `attribute` key renamed to `attr`.

## Patterns

- None.

## Gotchas

- None.

## Files

- `internal/component/bgp/decode/` — Ze JSON formatter replacing ExaBGP formatter
- `test/decode/` — updated .ci expectations
