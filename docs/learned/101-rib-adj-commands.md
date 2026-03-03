# 101 — RIB Adjacent Commands with Full Attributes

## Objective

Add Adj-RIB inspection and manipulation commands to the RIB plugin, with peer selector filtering and full attribute storage for route resend capability.

## Decisions

- Mechanical refactor, no design decisions.

## Patterns

- Peer selector (`*`, IP, `!IP`, comma-separated list) extracted into `internal/selector/` package for reuse.
- Text formatting functions (`FormatASPath`, `FormatCommunities`, etc.) moved into `internal/bgp/attribute/text.go` — consolidated from a now-deleted `internal/parse/` package.

## Gotchas

None.

## Files

- `internal/selector/selector.go` — reusable peer selector
- `internal/bgp/attribute/text.go` — attribute text formatters (consolidated from `internal/parse/`)
- `internal/plugin/rib/rib.go` — `rib adjacent inbound/outbound show/empty/resend` commands
