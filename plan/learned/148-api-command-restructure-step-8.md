# 148 — API Command Restructure Step 8: BGP Cache Commands

## Objective

Migrate `msg-id` and `forward update-id` commands to `bgp cache <id> *` namespace, completing the BGP-centric command restructure by grouping all UPDATE cache operations under the BGP namespace.

## Decisions

- Mechanical refactor — chose `bgp cache` as prefix because the cache stores BGP UPDATE messages and forward targets BGP peers. Belongs in BGP subsystem namespace.
- Single `bgp cache` handler with subcommand dispatch (retain/release/expire/forward/list) rather than separate registrations per subcommand — simpler parsing since `<id>` position varies.

## Patterns

- None beyond standard handler patterns.

## Gotchas

None.

## Files

- `internal/component/plugin/cache.go` — new, replaces msgid.go and forward.go
- `internal/component/plugin/msgid.go`, `internal/component/plugin/forward.go` — deleted
