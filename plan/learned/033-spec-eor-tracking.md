# 033 — EOR Tracking

## Objective

Fix End-of-RIB marker logic to send EOR only for address families where routes were actually advertised, instead of for all negotiated families.

## Decisions

- Chose to track families via a `map[family.Family]bool` (`familiesSent`) populated as routes are sent, then iterate the map at the end to send EORs. Simple and local to `sendInitialRoutes`.
- RFC 4724 says EOR MUST be sent even for families with no updates, but the implementation intentionally deviates to match ExaBGP encoding test expectations. This is a practical optimization.

## Patterns

- `routeFamily(route StaticRoute) family.Family` helper function extracts the NLRI family from a route, needed for both family tracking and pack context selection.

## Gotchas

- RFC 4724 Section 2 explicitly says EOR must be sent even when there are no updates — the implementation deviates from this for test compatibility. Future implementations of graceful restart must be aware of this deviation.

## Files

- `internal/reactor/peer.go` — `sendInitialRoutes()` family tracking, `routeFamily()` helper
