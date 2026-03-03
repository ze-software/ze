# 195 — IPC2 JSON Restructure

## Objective

Restructure the Ze BGP JSON event envelope: move `peer` to the bgp level (not nested per event type) and simplify state events to use a plain string.

## Decisions

Mechanical refactor, no design decisions.

## Patterns

- None.

## Gotchas

- None.

## Files

- `internal/plugins/bgp/event/` — JSON envelope restructure
- `internal/ipc/` — updated consumers
