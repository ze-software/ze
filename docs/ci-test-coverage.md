# .ci Test Coverage Analysis

Cross-reference of ze features against functional `.ci` test coverage.

**Total .ci tests:** ~269 across 8 directories.

## Coverage by Area

| Area | Features | With .ci | Gap | Coverage |
|------|----------|----------|-----|----------|
| Wire encode (attributes, messages) | 22 | 22 | 0 | 100% |
| Wire decode (all families) | 17 | 17 | 0 | 100% |
| NLRI encode/decode per family | 17 | 17 | 0 | 100% |
| Plugin lifecycle & features | 20 | 20 | 0 | 100% |
| Config parsing (valid/invalid) | 15 | 15 | 0 | 100% |
| Config reload & signals | 7 | 7 | 0 | 100% |
| CLI completion | 65 | 65 | 0 | 100% |
| ExaBGP compatibility | 45 | 45 | 0 | 100% |
| CLI commands | 18 | 7 | 11 | 39% |
| API runtime commands | 25 | 4 | 21 | 16% |
| Config behavior (beyond parsing) | 7 | 0 | 7 | 0% |
| Plugin behavior (beyond features) | 3 | 0 | 3 | 0% |

## Gaps — CLI Commands Without .ci

| Command | Description | Suggested .ci Location | Priority |
|---------|-------------|----------------------|----------|
| `ze config check` | Check deprecated patterns | test/parse/cli-config-check.ci | Medium |
| `ze config fmt` | Format/normalize config | test/parse/cli-config-fmt.ci | Medium |
| `ze config set` | Set config value | test/parse/cli-config-set.ci | High |
| `ze schema handlers` | Handler→module map | test/parse/cli-schema-handlers.ci | Low |
| `ze schema protocol` | Protocol version info | test/parse/cli-schema-protocol.ci | Low |
| `ze signal quit` | SIGQUIT goroutine dump | test/reload/signal-quit.ci | Low |
| `ze status` | Check daemon running | test/parse/cli-status.ci | Medium |
| `ze cli` | Interactive CLI / single command | test/plugin/cli-run-command.ci | High |
| `ze show` | Read-only daemon queries | test/plugin/cli-show.ci | High |
| `ze run` | Execute daemon commands | test/plugin/cli-run.ci | High |
| `ze exabgp migrate` | ExaBGP→ze config convert | test/parse/cli-exabgp-migrate.ci | Medium |

## Gaps — API Commands Without .ci

| Command | Description | Suggested .ci Location | Priority |
|---------|-------------|----------------------|----------|
| `bgp peer * list` | List peers | test/plugin/api-peer-list.ci | High |
| `bgp peer * show` | Peer details + stats | test/plugin/api-peer-show.ci | High |
| `bgp peer * add` | Dynamic peer addition | test/plugin/api-peer-add.ci | High |
| `bgp peer * remove` | Remove peer | test/plugin/api-peer-remove.ci | High |
| `bgp peer * pause` | Pause reading | test/plugin/api-peer-pause.ci | Medium |
| `bgp peer * resume` | Resume reading | test/plugin/api-peer-resume.ci | Medium |
| `bgp peer * capabilities` | Negotiated caps | test/plugin/api-peer-capabilities.ci | Medium |
| `bgp summary` | Summary table | test/plugin/api-bgp-summary.ci | High |
| `rib show-in` | Show Adj-RIB-In | test/plugin/api-rib-show-in.ci | High |
| `rib show-out` | Show Adj-RIB-Out | test/plugin/api-rib-show-out.ci | High |
| `rib clear-in` | Clear Adj-RIB-In | test/plugin/api-rib-clear-in.ci | Medium |
| `rib clear-out` | Clear Adj-RIB-Out | test/plugin/api-rib-clear-out.ci | Medium |
| `cache list` | List cached messages | test/plugin/api-cache-list.ci | Medium |
| `cache retain/release` | Cache management | test/plugin/api-cache-ops.ci | Medium |
| `cache forward` | Forward cached | test/plugin/api-cache-forward.ci | Medium |
| `subscribe` | Event subscription | test/plugin/api-subscribe.ci | High |
| `unsubscribe` | Unsubscribe | test/plugin/api-unsubscribe.ci | Medium |
| `commit start/end` | Named update window | test/plugin/api-commit-workflow.ci | High |
| `commit rollback` | Rollback changes | test/plugin/api-commit-rollback.ci | Medium |
| `route-refresh` | Send route refresh | test/plugin/api-route-refresh.ci | Medium |
| `bgp peer * raw` | Raw message injection | test/plugin/api-raw.ci | Low |

## Gaps — Config Behavior Without .ci

These config options parse correctly (tested) but their runtime effect is not proven by .ci tests.

| Feature | Description | Suggested .ci Location | Priority |
|---------|-------------|----------------------|----------|
| Connection mode | passive/active/both | test/plugin/config-connection-mode.ci | Medium |
| Router ID override | Per-peer router-id | test/plugin/config-router-id.ci | Low |
| Group updates | Enable/disable grouping | test/plugin/config-group-updates.ci | Medium |
| ADD-PATH per-family | Send/receive per family | test/plugin/config-addpath-mode.ci | High |
| Extended next-hop | Per-family NH mapping | test/plugin/config-ext-nexthop.ci | Medium |
| Role strict | Require peer role | test/plugin/config-role-strict.ci | Medium |
| Adj-RIB flags | Per-peer adj-rib config | test/plugin/config-adj-rib.ci | Medium |

## Gaps — Plugin Behavior Without .ci

Plugin features are advertised (tested), but actual behavior is not exercised end-to-end.

| Plugin | Feature Tested | Behavior Gap | Suggested .ci |
|--------|---------------|--------------|---------------|
| bgp-persist | Features advertisement | Persistence across restart | test/plugin/persist-across-restart.ci |
| bgp-adj-rib-in | Features advertisement | Query/clear via API | test/plugin/adj-rib-in-query.ci |
| role | Capability + features | Strict mode enforcement | test/plugin/role-strict-enforcement.ci |

## Existing .ci Coverage Summary

| Directory | Count | What It Covers |
|-----------|-------|----------------|
| test/encode/ | 85 | Wire encoding: all attributes, capabilities, families, message types |
| test/plugin/ | 76 | Plugin lifecycle, registration, NLRI decode, route operations, reconnection |
| test/ui/ | 65 | CLI completion for all config paths, commands, values |
| test/exabgp-compat/ | 45 | ExaBGP config migration parity for all encoding scenarios |
| test/parse/ | 36 | Config validation, CLI schema/validate/config commands |
| test/decode/ | 34 | Wire decoding: all families, capabilities, message types |
| test/reload/ | 9 | SIGHUP reload scenarios, SIGTERM, peer add/remove/restart |
| test/chaos-web/ | 4 | Web dashboard endpoints, visualization, reporting |

## Priority Order for Gap Closure

### P0 — Critical (features users interact with daily)

1. `bgp peer * list` / `bgp summary` — peer visibility
2. `bgp peer * show` — peer diagnostics
3. `rib show-in` / `rib show-out` — route visibility
4. `subscribe` — event monitoring
5. `commit start/end` — atomic updates
6. `ze show` / `ze run` / `ze cli` — runtime CLI
7. `ze config set` — programmatic config

### P1 — Important (operational features)

8. `bgp peer * add/remove` — dynamic peers
9. ADD-PATH per-family mode — affects wire encoding
10. `ze status` — daemon health check
11. `ze config check` / `ze config fmt` — config tooling
12. Role strict enforcement — security feature
13. `route-refresh` — operational procedure

### P2 — Nice to have

14. `bgp peer * pause/resume` — flow control
15. Cache operations — message management
16. `ze exabgp migrate` — migration tooling
17. Schema handlers/protocol — discovery
18. Connection mode behavior — mostly defaults
19. Persistence across restart — durability
