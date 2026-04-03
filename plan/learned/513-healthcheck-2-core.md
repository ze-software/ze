# 513 -- healthcheck-2-core

## Context

Ze needed a BGP healthcheck plugin to monitor service availability and control route announcement/withdrawal via watchdog groups. ExaBGP's healthcheck.py was the reference implementation. The goal was feature parity with a cleaner model: route attributes in BGP config, only MED controlled per health state.

## Decisions

- Plugin placed under `internal/component/bgp/plugins/healthcheck/` with Dependencies: ["bgp-watchdog"], ensuring watchdog starts first.
- 8-state FSM matching ExaBGP: INIT, RISING, UP, FALLING, DOWN, DISABLED, EXIT, END. All transitions through `trigger()` with rise/fall shortcuts.
- Used DispatchCommand for watchdog communication over direct import, maintaining plugin isolation.
- Probe execution uses `exec.CommandContext` with `Setpgid: true` and `cmd.Cancel` killing the process group, over letting Go's default context cancellation kill only the direct child (which leaves orphaned subprocesses).
- YANG schema includes all leaves for all phases upfront (ip-setup, hooks, metrics) to avoid schema changes in later phases.

## Consequences

- Healthcheck is a first-class BGP plugin with YANG validation, editor completions, and config reload support.
- The `cmd.Cancel` process group kill pattern should be used for any future shell execution in plugins.
- ProbeConfig has slice fields (IPs, hooks) requiring `reflect.DeepEqual` for config change detection instead of struct `==`.

## Gotchas

- The `check-existing-patterns.sh` hook rejects `SetLogger` in new files because it exists in other plugins. Each plugin package must have its own -- create a minimal file first, then edit.
- `TestAllPluginsRegistered` and `TestAvailablePlugins` in two separate locations both need updating when adding a plugin (all_test.go and cmd/ze/main_test.go).
- `make generate` must run after creating the plugin directory to update `all/all.go` with the blank import.

## Files

- `internal/component/bgp/plugins/healthcheck/` -- entire plugin package (11 source files)
- `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` -- YANG schema
- `internal/component/plugin/all/all.go` -- blank import
