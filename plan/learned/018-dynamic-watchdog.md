# 018 — Dynamic Watchdog

## Objective

Implement watchdog API commands (`announce watchdog <name>` / `withdraw watchdog <name>`) and config-based watchdog routes, enabling conditional route announcement tied to named health groups.

## Decisions

Mechanical implementation of a planned feature. The core design (WatchdogManager with pool-based route control) was documented in spec 019 and implemented together.

## Patterns

- Config-based watchdog: `route 77.77.77.77 next-hop 1.2.3.4 watchdog dnsr withdraw;` — routes start in withdrawn state, `announce watchdog dnsr` announces them.
- API group control: `announce watchdog <name>` / `withdraw watchdog <name>` — flip all routes in the named group.

## Gotchas

- Dynamic watchdog route creation via API (not just toggling config-defined routes) was deferred — see spec 019 for the pool-based architecture that would support this.

## Files

- `internal/reactor/watchdog.go` — WatchdogManager, WatchdogPool, PoolRoute
