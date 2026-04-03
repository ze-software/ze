# 516 -- healthcheck-0-umbrella

## Context

Ze needed a healthcheck plugin with feature parity to ExaBGP's healthcheck.py. The plugin monitors service availability via shell commands and controls BGP route announcement/withdrawal through watchdog groups. The umbrella spec designed the full system across 5 phases: watchdog MED extension, core plugin (FSM + probe + announce), MED/debounce/lifecycle modes, IP/hooks/CLI, and external mode validation.

## Decisions

- Chose a single watchdog group with MED override per health state over multiple route definitions per probe. Simpler model: route attributes live in BGP config, only MED varies per state.
- Dropped ExaBGP's per-state community and as-path variation. ExaBGP's per-state as-path has a bug (uses `options.as_path` instead of resolved variable), validating the decision.
- Dropped labels from ip-setup (Ze uses netlink, not `ip` command; tracks IPs internally).
- Disable via config reload (`ze config set ... disable true`) over ExaBGP's file-poll mechanism, fitting Ze's config-driven model.
- 5-phase decomposition over monolithic implementation: each phase produced a wired, testable, user-reachable feature. Phases 3-5 were consolidated into one spec when most logic was already in Phase 2's probe loop.
- Default behavior is metric mode (MED override), matching ExaBGP defaults. Withdraw-on-down requires explicit opt-in.

## Consequences

- Healthcheck is a first-class BGP plugin with YANG validation, editor completions, config reload, CLI commands, and both internal/external plugin modes.
- The watchdog MED override extension (`watchdog announce <name> med <N>`) is reusable by any future plugin needing per-dispatch MED control.
- External plugins cannot use ip-setup (rejected at configure/config-verify callback).
- Users needing per-state communities or as-paths must define separate watchdog groups -- deliberate simplification documented in the spec.

## Gotchas

- YANG schema was created with all leaves upfront (all phases) to avoid schema changes in later phases. This front-loaded work but prevented cross-phase YANG migrations.
- The `cmd.Cancel` process group kill pattern (for probe and hook timeouts) should be the standard for any future shell execution in plugins.
- Config change detection uses `reflect.DeepEqual` on ProbeConfig structs -- leaf-list reordering triggers reconfigure. Acceptable because config reordering is rare.
- MED override must bypass watchdog dedup because the pool tracks announced/withdrawn state, not command content. Without bypass, MED changes on already-announced routes are silently dropped.

## Files

- `internal/component/bgp/plugins/healthcheck/` -- entire plugin (11 source files + tests)
- `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` -- YANG schema
- `internal/component/bgp/plugins/watchdog/server.go` -- MED override extension
- `internal/component/bgp/plugins/watchdog/pool.go` -- Route field on PoolEntry
- `plan/learned/512-healthcheck-1-watchdog-med.md` -- Phase 1 summary
- `plan/learned/513-healthcheck-2-core.md` -- Phase 2 summary
- `plan/learned/514-healthcheck-3-5-modes-ip-hooks-cli-external.md` -- Phases 3-5 summary
