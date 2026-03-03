# 104 — Remove "announce route" Syntax

## Objective

Complete migration from `announce route` to `update text` syntax, removing the old command handler and updating plugins and tests to use the canonical form.

## Decisions

- Mechanical refactor, no design decisions.

## Patterns

- `handleAnnounceRoute` was dead code (not registered) — removed. `announceRouteImpl` was kept because it was used internally by `announce ipv4/ipv6` handlers (which still existed at the time).
- `commit announce route` handler removed — users must use `update text` directly.
- RIB output format: `peer X update text nhop set Y nlri Z add P`. RR withdraw: `peer !X update text nlri Z del P`.

## Gotchas

- Bug found: `internal/reactor/reactor.go` needed a nil check for Origin extraction — was panicking on routes without explicit ORIGIN attribute.

## Files

- `internal/plugin/route.go` — removed `handleAnnounceRoute`
- `internal/plugin/commit.go` — removed `commit announce route` handler
- `internal/plugin/rib/rib.go`, `internal/plugin/rr/server.go` — updated output to `update text` syntax
