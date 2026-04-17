# 612 -- vpp-6 Telemetry and Counters

## Context

VPP exposes per-interface counters, per-graph-node CPU cycles, and system-wide
metrics through a shared-memory stats segment (separate from the binary API).
Ze needed to poll this segment on a configurable interval and export the data
as Prometheus metrics, giving operators visibility into VPP's forwarding
performance that the kernel intermediary path cannot provide. fibvpp also
needed its own route-count metrics (installed gauge, installs/withdraws
counters) independent of the stats segment.

## Decisions

- **Stats poller in VPP component** (`telemetry.go`) over a separate plugin.
  The poller is tightly coupled to the VPP connection lifecycle (start on
  connect, cancel on disconnect) and the stats socket path comes from the
  same YANG config. Putting it in a separate plugin would add dependency
  wiring for no benefit.
- **fibvpp owns its own metrics** via `ConfigureMetrics` callback (same
  pattern as fibkernel). No reverse dependency from telemetry into fibvpp.
  `stats.go` registers `ze_fibvpp_routes_installed` gauge and
  `ze_fibvpp_route_installs_total` / `withdraws_total` / `errors_total`
  counters.
- **`ze_vpp_stats_up` health gauge** (1 when stats client connected, 0
  when disconnected). Operators can alert on this without scraping every
  individual metric.
- **Counter delta tracking** for interface metrics. VPP stats segment
  counters are monotonic; the poller computes deltas to feed Prometheus
  counters correctly across VPP restarts where the raw counter resets.
- **Functional test deferred**: `test/vpp/012-telemetry.ci` blocked on
  stats segment emulation in `vpp_stub.py` (tracked in `plan/deferrals.md`
  under spec-vpp-7 Phase 4). All 14 ACs covered by unit tests.

## Consequences

- `ze_vpp_interface_{rx,tx}_{packets,bytes}`, `ze_vpp_interface_drops`,
  `ze_vpp_interface_{rx,tx}_errors` counters per interface name.
- `ze_vpp_node_{clocks,vectors}` gauges per graph node.
- `ze_vpp_system_{vector_rate,input_rate}` system-wide gauges.
- Stats poll interval configurable via `vpp { stats { poll-interval 30; } }`
  YANG leaf (1-3600 seconds, default 30).
- When stats segment emulation lands in the stub, `012-telemetry.ci` can
  be written to validate the full pipeline end-to-end.

## Gotchas

- Stats client connects to a different socket than the binary API
  (`stats.sock` vs `api.sock`). Both connections have independent
  lifecycles. A stats connection failure does not affect route programming.
- GoVPP's `GetInterfaceStats` returns cumulative counters. On VPP restart,
  counters reset to zero. The delta tracker handles this by detecting
  when a new value is less than the previous and treating the new value
  as the delta (counter wrapped or reset).
- The `ConfigureMetrics` callback in `register.go` fires before OnStarted.
  The metrics registry is stored via atomic pointer so the stats poller
  (started later in OnStarted) picks it up without a race.

## Files

- `internal/component/vpp/telemetry.go` -- Stats poller + metric registration
- `internal/component/vpp/telemetry_test.go` -- 8 unit tests
- `internal/component/vpp/stats_conn.go` -- Stats socket connection helper
- `internal/plugins/fibvpp/stats.go` -- FIB route count metrics
- `internal/plugins/fibvpp/stats_test.go` -- FIB stats tests
- `internal/component/vpp/schema/ze-vpp-conf.yang` -- poll-interval leaf
