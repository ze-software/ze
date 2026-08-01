# 736 -- Interface Rate Tracking

## Context

Ze had no per-interface rate computation. Operators could see cumulative counters via `show interface <name> counters` but had no bytes-per-second or packets-per-second view. The iface component registered zero Prometheus metrics while every other telemetry-enabled component (host, BFD, traffic) published gauges. The goal was a single 1s sampler with CLI, Prometheus, and web consumers.

## Decisions

- Rate tracker lives in iface component over a standalone telemetry collector, because iface already owns interface stats (counters.go, dispatch.go, Backend.ListInterfaces).
- Uses raw backend stats (bypassing baseline-delta from counters.go) over baseline-adjusted stats, because rate computation needs monotonic kernel counters and the baseline mechanism is for "since last clear" display.
- 12 GaugeVec metrics (4 rates + 8 raw counters) over fewer, because Netdata collector covers different metrics (netdata_net_*) at a different interval (10s) with different naming.
- Hardcoded 1s interval over configurable, because SONiC and Arista precedent is 1-2s and YAGNI applies.
- `show interface rate [<name>]` grammar (action before identifier) over `show interface <name> rate`, following the cli-grammar.md rule that eliminates keyword/name ambiguity.
- atomic.Pointer for metrics registry injection (BFD pattern) over passing registry through function args, for consistency with existing plugin patterns.
- Counter wrap returns 0 over attempting wrap-around math, matching the existing safeDelta convention in telemetry/collector/delta_linux.go.

## Consequences

- The iface component now has a long-lived goroutine (rate tracker) that must be stopped during shutdown, following the existing ticker+stop-channel pattern from host/metrics.go.
- Stale interface cleanup deletes Prometheus labels, unlike BFD which leaves vanished labels at their last value. This is correct for interfaces (they can be created/destroyed frequently) but means label cardinality is bounded by current kernel interfaces, not historical.
- The `show interface` handler now has 5 dispatch keywords before the catch-all name check: brief, type, errors, rate, scan. If more are added, consider switching to a map dispatch.

## Gotchas

- Rate tracker must call `GetBackend().ListInterfaces()` directly, not the package-level `iface.ListInterfaces()` which applies baseline subtraction. Using baseline-adjusted values would produce incorrect deltas when baselines exist.
- The fakeRegistry in tests must return `metrics.Counter`, `metrics.Gauge`, etc. (named interface types), not anonymous interfaces with the same methods. Go's type system does not consider them equivalent.
- `formatCountersTable` in the web page gained a `name` parameter to look up rate data. All callers must be updated together since Go has no default parameters.

## Files

- `internal/component/iface/iface.go` -- added InterfaceRate type
- `internal/component/iface/rate.go` -- rate tracker, metrics, delta computation (new)
- `internal/component/iface/rate_test.go` -- 11 unit tests (new)
- `internal/component/iface/dispatch.go` -- added ListRates(), GetRate()
- `internal/component/iface/register.go` -- added ConfigureMetrics, rate tracker lifecycle
- `internal/component/cmd/show/show.go` -- added rate dispatch in handleShowInterface
- `internal/component/iface/cmd/interface_rate.go` -- show + monitor handlers (new)
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- rate container
- `internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang` -- interface rate container
- `internal/component/web/page_interfaces.go` -- rate columns in table, rate data in detail
- `docs/features.md` -- interface rate tracking mention
- `docs/guide/command-reference.md` -- show/monitor interface rate docs
- `docs/architecture/api/commands.md` -- WireMethod documentation
