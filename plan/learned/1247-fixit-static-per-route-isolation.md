# 1247 -- fixit-static-per-route-isolation

## Context

One unresolvable static route used to fail the WHOLE `static { }` section:
`routeManager.applyRoutes` joined every per-route error with `errors.Join`
(`internal/plugins/static/inject.go`) and returned it, so `OnConfigure` aborted daemon
startup and `OnConfigApply` rolled the whole edit back. A single bad next-hop (an
interface next-hop whose backend is absent, a device that does not exist yet) took down
every other static route with it. This whole-section-fail was a deliberate
keep-and-document choice in `fixit-static-interface-nexthops` (650); this spec is the
follow-up that implements per-route isolation: log-and-skip the bad route, keep the rest
programmed. The open A-2 product-contract decision was resolved by an autonomous default
(Thomas may override) rather than blocking on a ruling.

## Decisions

- **A-2 autonomous default: per-route apply/remove failures are skipped, not
  section-fatal.** `applyRoutes` applies what resolves, records the failed route in a new
  `rm.skipped` map, logs WARN, and returns `nil`. Chosen over whole-section-fail (the 650
  status quo) and over a distinct sentinel error. Config VALIDATION is untouched:
  `OnConfigVerify`/`parseStaticConfig` still reject syntactic errors before apply (AC-4).
- **The skipped route is kept OUT of the diff baseline `rm.routes`** (deleted on program
  failure, recorded in `rm.skipped`), so it is re-attempted on the next apply and resolves
  automatically once the device/backend appears, while the `routesEqual` short-circuit
  still stops an unrelated interface edit from reprogramming the good routes (650 R-10 /
  AC-5). A route that later programs clears its `rm.skipped` entry.
- **The contract composes with the `OnConfigApply` journal rollback** rather than fighting
  it: `applyRoutes` returning `nil` means `j.Record`'s forward func sees nil, so no
  spurious rollback fires; the rollback path stays intact for a genuine failure.
- **Observability is fail-closed** (`ai/rules/evidence.md`): a skip is never
  silent. An always-on WARN log, a `static show` `skipped`/`skip-reason` column, and a
  `doctor-static-route-skipped` doctor check (reaching the live manager via an
  `activeRouteManager atomic.Pointer` singleton, set/cleared on the plugin lifecycle).

## Consequences

- One unresolvable static route no longer aborts startup or drops the rest of the routing
  table; it is skipped, shown, and retried.
- The failure contract changed for ALL static routes (documented in 650): callers see a
  now-always-nil per-route result; genuine config errors still fail at validation.
- The doctor check is in-process only (the `show doctor` RPC path); offline `ze doctor` and
  forked static are silent there, with `static show` + the WARN log as the always-on
  surfaces.

## Gotchas

- **The naive isolation introduced a routing BLACKHOLE, caught in review and fixed.** A
  forward->forward replacement to an unresolvable next-hop, when skipped, left the OLD
  route's redistribute announcement standing AND orphaned its kernel FIB entry:
  `teardownRouteLocked` does NOT touch the backend, and the failed `RouteReplace` leaves the
  old kernel route; because the prefix is then deleted from `rm.routes`, re-apply and
  `shutdown()` never reclaim it (it survives a daemon restart), while `static show` reports
  the prefix "skipped/unrouted" -- an active lie, worse than a silent skip. Before
  isolation, `OnConfigApply` rollback kept announcement+FIB consistent; returning nil
  removed that safety net. **Fix:** on a skip that discarded an existing route,
  `backend.removeRoute` the orphan AND emit `ActionRemove` when it was emitted -- guarded so
  forward->non-forward (which already emits at the `existing.emitted && r.Action !=
  actionForward` branch) does not double-emit. **The design mistake was extending the 650
  flap-avoidance ("no Remove on forward->forward") to the SKIP case.** Flap-avoidance
  applies only to a SUCCESSFUL replacement (the prefix stays reachable via a new next-hop);
  a skipped replacement means the route is genuinely GONE, so a Remove is correct, not a
  flap. A "documented limitation" that ships a blackhole is not a limitation, it is a bug.
- **`teardownRouteLocked` does not remove the kernel route** -- a torn-down route's FIB
  entry survives until `backend.removeRoute` is called explicitly. Easy to miss when
  reasoning about "the old route is gone."
- **The functional `.ci` is `needs-linux`**: on darwin the `interface` component has no
  OS-default backend, so the static daemon cannot boot at all. The AC discrimination
  (good programmed vs bad skipped) is carried by unit tests over a fake backend; the `.ci`
  proves the end-to-end path under QEMU/CI.

## Files

`internal/plugins/static/inject.go` (skip-and-record + orphan reclaim on skipped replace +
`rm.skipped` + `showRoutes` marking + `activeRouteManager`), `register.go` (publish the
manager), `doctor.go` (`static-route-skipped` check), `internal/core/diagnostic/codes.go`
(code), `isolation_test.go` (new), `doctor_test.go`/`register_test.go` (additions),
`test/static/007-per-route-isolation.ci` (needs-linux), `plan/learned/650-static-routes.md`
(blast-radius note corrected).
