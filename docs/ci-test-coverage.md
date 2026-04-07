# .ci Test Coverage Priorities

<!-- source: scripts/inventory/inventory.go -- ze-inventory implementation -->

For current counts and uncovered RPCs, run `make ze-inventory`.

## Gaps -- Config Behavior Without .ci

All config behavior gaps closed by spec-ci-gaps (2026-03-17):

| Feature | Description | .ci Test | Status |
|---------|-------------|----------|--------|
| Connection mode | connect/accept booleans | test/plugin/config-connection-mode.ci | Covered |
| Router ID override | Per-peer router-id | test/encode/router-id-override.ci | Covered |
| Group updates | Enable/disable grouping | test/plugin/config-group-updates.ci | Covered |
| ADD-PATH per-family | Send/receive per family | test/plugin/config-addpath-mode.ci | Covered |
| Extended next-hop | Per-family NH mapping | test/plugin/config-ext-nexthop.ci | Covered |
| Role strict | Require peer role | test/plugin/config-role-strict.ci | Covered |
| Adj-RIB flags | Per-peer adj-rib config | test/plugin/config-adj-rib.ci | Covered |

## Gaps -- Plugin Behavior Without .ci

All plugin behavior gaps closed by spec-ci-gaps (2026-03-17):

| Plugin | Feature | .ci Test | Status |
|--------|---------|----------|--------|
| bgp-persist | Persistence across restart | test/reload/persist-across-restart.ci | Covered |
| bgp-adj-rib-in | Query/clear via API | test/plugin/adj-rib-in-query.ci | Covered |
| role | Strict mode enforcement | test/plugin/role-strict-enforcement.ci | Covered |

## Priority Order for Gap Closure

All gaps closed. 41 new .ci tests added across 5 phases:

- Phase 1: 10 CLI command tests (config validate/fmt/set, schema, status, show/run, exabgp migrate)
- Phase 2: 10 API peer management tests (list/detail/add/remove/pause/resume/capabilities/subscribe/unsubscribe/route-refresh)
- Phase 3: 11 API operation tests (rib show/clear, cache, commit, raw, CLI dispatch)
- Phase 4: 7 config runtime behavior tests (connect/accept mode, router-id, group-updates, addpath, ext-nexthop, role-strict, adj-rib)
- Phase 5: 3 plugin behavior tests (persist, adj-rib-in query, role strict enforcement)

1 test deferred: `signal-quit.ci` -- `ze signal` has no quit handler, test framework has no `action=sigquit`.
