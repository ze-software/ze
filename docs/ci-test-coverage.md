# .ci Test Coverage Priorities

For current counts and uncovered RPCs, run `make ze-inventory`.

## Gaps -- Config Behavior Without .ci

Config options that parse correctly (tested) but whose runtime effect is not proven by .ci tests.

| Feature | Description | Suggested .ci Location | Priority |
|---------|-------------|----------------------|----------|
| Connection mode | passive/active/both | test/plugin/config-connection-mode.ci | Medium |
| Router ID override | Per-peer router-id | test/plugin/config-router-id.ci | Low |
| Group updates | Enable/disable grouping | test/plugin/config-group-updates.ci | Medium |
| ADD-PATH per-family | Send/receive per family | test/plugin/config-addpath-mode.ci | High |
| Extended next-hop | Per-family NH mapping | test/plugin/config-ext-nexthop.ci | Medium |
| Role strict | Require peer role | test/plugin/config-role-strict.ci | Medium |
| Adj-RIB flags | Per-peer adj-rib config | test/plugin/config-adj-rib.ci | Medium |

## Gaps -- Plugin Behavior Without .ci

Plugin features are advertised (tested), but actual behavior is not exercised end-to-end.

| Plugin | Feature Tested | Behavior Gap | Suggested .ci |
|--------|---------------|--------------|---------------|
| bgp-persist | Features advertisement | Persistence across restart | test/plugin/persist-across-restart.ci |
| bgp-adj-rib-in | Features advertisement | Query/clear via API | test/plugin/adj-rib-in-query.ci |
| role | Capability + features | Strict mode enforcement | test/plugin/role-strict-enforcement.ci |

## Priority Order for Gap Closure

### P0 -- Critical (features users interact with daily)

1. `bgp peer * list` / `bgp summary` -- peer visibility
2. `bgp peer * show` -- peer diagnostics
3. `rib show-in` / `rib show-out` -- route visibility
4. `subscribe` -- event monitoring
5. `commit start/end` -- atomic updates
6. `ze show` / `ze run` / `ze cli` -- runtime CLI
7. `ze config set` -- programmatic config

### P1 -- Important (operational features)

8. `bgp peer * add/remove` -- dynamic peers
9. ADD-PATH per-family mode -- affects wire encoding
10. `ze status` -- daemon health check
11. `ze config check` / `ze config fmt` -- config tooling
12. Role strict enforcement -- security feature
13. `route-refresh` -- operational procedure

### P2 -- Nice to have

14. `bgp peer * pause/resume` -- flow control
15. Cache operations -- message management
16. `ze exabgp migrate` -- migration tooling
17. Schema handlers/protocol -- discovery
18. Connection mode behavior -- mostly defaults
19. Persistence across restart -- durability
