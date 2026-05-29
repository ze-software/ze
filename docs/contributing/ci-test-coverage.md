# .ci Test Coverage Priorities

<!-- source: scripts/inventory/inventory.go -- ze-inventory implementation -->

For current counts and uncovered RPCs, run `make ze-inventory`.

## Evidence Caveat

98 `.ci` files under `test/plugin/` use the observer `sys.exit(1)` pattern for
failure signaling. The parse test runner does not treat observer exit codes as
authoritative test failure (P0-3 in release-readiness-review-2026-04-23). Until
the parse runner is fixed to execute full `.ci` semantics, tests relying on
this pattern provide partial evidence only. "Covered" entries below that cite
`test/plugin/*.ci` files using this pattern should not be treated as fully
closed for release-gating purposes.

## Gaps -- Config Behavior Without .ci

All config behavior gaps closed by spec-ci-gaps (2026-03-17):

| Feature | Description | .ci Test | Status |
|---------|-------------|----------|--------|
| Connection mode | connect/accept booleans | test/plugin/config-connection-mode.ci | Covered |
| Router ID override | Per-peer router-id | test/encode/router-id-override.ci | Covered |
| Group updates | Enable/disable grouping | test/plugin/config-group-updates.ci | Covered (observer-exit) |
| ADD-PATH per-family | Send/receive per family | test/plugin/config-addpath-mode.ci | Covered (observer-exit) |
| Extended next-hop | Per-family NH mapping | test/plugin/config-ext-nexthop.ci | Covered (observer-exit) |
| Role strict | Require peer role | test/plugin/config-role-strict.ci | Covered |
| Adj-RIB flags | Per-peer adj-rib config | test/plugin/config-adj-rib.ci | Covered (observer-exit) |

## Gaps -- Plugin Behavior Without .ci

All plugin behavior gaps closed by spec-ci-gaps (2026-03-17):

| Plugin | Feature | .ci Test | Status |
|--------|---------|----------|--------|
| bgp-persist | Persistence across restart | test/reload/persist-across-restart.ci | Covered |
| bgp-adj-rib-in | Query/clear via API | test/plugin/adj-rib-in-query.ci | Covered (observer-exit) |
| role | Strict mode enforcement | test/plugin/role-strict-enforcement.ci | Covered |

## Priority Order for Gap Closure

All gaps have corresponding .ci tests (41 added across 5 phases), but 5 of
them rely on observer-exit and are not fully authoritative until P0-3 is
resolved:

- Phase 1: 10 CLI command tests (config validate/fmt/set, schema, status, show/run, exabgp migrate)
- Phase 2: 10 API peer management tests (list/detail/add/remove/pause/resume/capabilities/subscribe/unsubscribe/route-refresh)
- Phase 3: 11 API operation tests (rib show/clear, cache, commit, raw, CLI dispatch)
- Phase 4: 7 config runtime behavior tests (connect/accept mode, router-id, group-updates, addpath, ext-nexthop, role-strict, adj-rib)
- Phase 5: 3 plugin behavior tests (persist, adj-rib-in query, role strict enforcement)

1 test deferred: `signal-quit.ci` -- `ze signal` has no quit handler, test framework has no `action=sigquit`.
