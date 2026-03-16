# 099 — Rename `persist` Plugin to `rib` with Adj-RIB-In

## Objective

Rename the `persist` plugin to `rib` and add Adj-RIB-In support (routes received FROM peers), complementing the existing Adj-RIB-Out (routes sent TO peers). In-memory only, no best-path selection.

## Decisions

- Mechanical refactor, no design decisions beyond the feature split.

## Patterns

- RIB event format: received events use `message.type: "update"` with a message wrapper; sent events use `type: "sent"` directly. Both use array format for prefixes.
- Route key: `family:prefix` — ADD-PATH not yet supported (limitation noted; path-id support done in spec 100).

## Gotchas

- Bug found during critical review: `handleReceived` was written expecting a nested map but actual format is an array. Silent failure — no error, just wrong behavior.
- Logging was added for silent ignores (`slog`) after bug hunt to prevent future silent failures.

## Files

- `internal/component/plugin/rib/rib.go` (renamed from `persist/persist.go`) — Adj-RIB-In handling
- `internal/component/plugin/rib/event.go` — unified event parsing for both received and sent formats
- `cmd/ze/bgp/plugin_rib.go` (renamed from `plugin_persist.go`)
