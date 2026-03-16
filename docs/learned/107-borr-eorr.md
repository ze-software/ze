# 107 — BoRR/EoRR (RFC 7313 Enhanced Route Refresh)

## Objective

Implement complete RFC 7313 Enhanced Route Refresh support: capability check when sending BoRR/EoRR (Level 1), handling incoming ROUTE-REFRESH messages (Level 2), and RIB plugin responding to refresh requests with BoRR → routes → EoRR (Level 3).

## Decisions

- Mechanical implementation, no design decisions.

## Patterns

- Levels 1 and 2 were already implemented in `reactor.go` and `session.go`. Only Level 3 (RIB plugin handling) was missing.
- RIB responds to `refresh` event by iterating `adj-rib-out` for the requested peer/family, sending BoRR, replaying routes, then EoRR.

## Gotchas

- Critical bug found during functional test: `internal/component/config/loader.go` was not adding `RouteRefresh{}` and `EnhancedRouteRefresh{}` capability objects when `route-refresh` was configured. Negotiation succeeded (both sides "supported" it) but the marker capabilities were never included in OPEN, so negotiation failed silently and BoRR/EoRR were never sent.
- Levels 1 and 2 were "already done" per review, but had never been exercised by a functional test. The bug above existed undetected.

## Files

- `internal/component/plugin/rib/rib.go` — `handleRefresh`, dispatch cases for refresh/borr/eorr events
- `internal/component/plugin/json.go` — `RouteRefresh()` JSON encoding
- `internal/component/plugin/text.go` — `FormatRouteRefresh()` text formatting
- `internal/component/config/loader.go` — bug fix: now adds `RouteRefresh{}` and `EnhancedRouteRefresh{}` when configured
- `test/data/plugin/refresh.ci` — functional test: send ROUTE-REFRESH, expect BoRR/route/EoRR
