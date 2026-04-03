# 514 -- healthcheck-3-5-modes-ip-hooks-cli-external

## Context

Phases 3-5 added MED mode, debounce, fast-interval, config reload lifecycle, disable toggle, IP management, hooks, CLI commands, and external mode validation to the healthcheck plugin. Most MED/debounce/lifecycle logic was already implemented in Phase 2's probe loop; these phases added tests, IP/hook wiring, and CLI.

## Decisions

- Config change detection uses `reflect.DeepEqual` on ProbeConfig (which has slice fields) over field-by-field comparison. Simpler but reordering IPs or hooks triggers spurious reconfigure. Acceptable tradeoff.
- IP management uses a local `ipManager` interface wrapping `iface.AddAddress/RemoveAddress` for test injection, over importing iface directly in tests (which requires CAP_NET_ADMIN).
- Hooks run in goroutines (async, don't block FSM) with 30s timeout and process group kill, over ExaBGP's synchronous `subprocess.call` which can hang the main loop.
- CLI commands registered as `healthcheck show` and `healthcheck reset` via `sdk.CommandDecl`, matching the existing watchdog pattern.
- External mode detection via a `bool internal` flag set at `newProbeManager` time, over runtime detection via SDK (no public accessor for bridge presence).
- `dispatchFn` field on probeManager enables test isolation without requiring a real SDK plugin.

## Consequences

- IP management is fully testable without root privileges via mock ipManager.
- Reset deconfigures and restarts the probe (heavier than in-place FSM reset) but functionally correct.
- External plugins get a clear error message when ip-setup is configured.

## Gotchas

- `probeManager` with bare `&sdk.Plugin{}` panics on DispatchCommand (nil MuxConn). Injectable `dispatchFn` was required for test isolation.
- Hooks are async -- tests need `time.Sleep` to wait for completion. Not ideal but acceptable for shell execution tests.
- `block-silent-ignore.sh` hook rejects `default:` in switch statements. Use explicit case listing or move unreachable return outside switch.
- `goconst` linter flags repeated `"error"` and `"done"` strings -- must use package-level constants.

## Files

- `internal/component/bgp/plugins/healthcheck/ip.go` -- ipManager interface, ipTracker
- `internal/component/bgp/plugins/healthcheck/hooks.go` -- async hook execution
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` -- CLI commands, IP/hook wiring, external mode validation
- `internal/component/bgp/plugins/healthcheck/lifecycle_test.go` -- lifecycle + CLI tests
