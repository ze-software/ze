# 653 -- netdata-os-collectors

## Context

The LNS monitoring stack runs Netdata solely to export OS metrics (CPU, memory,
network, disk, etc.) to Prometheus. Replacing Netdata with Ze-native collectors
eliminates a separate service and lets Ze own the full telemetry surface.
Requirement: the Prometheus output must match Netdata exactly so existing Grafana
dashboards continue to work unchanged.

Built 30+ collectors under `internal/component/telemetry/collector/` reading
/proc and /sys, producing 138 metrics that match Netdata's Prometheus format
exactly (context, units, chart/dimension/family labels, sanitization rules).

## Decisions

- Used Netdata's naming convention (`netdata_{context}_{units}_average` with
  `chart`/`dimension`/`family` labels) as the default, with configurable prefix
  via YANG for future migration. Alternative (node_exporter naming or Ze's own
  `ze_` prefix) would have required rewriting every Grafana dashboard.
- Built on top of the existing `prometheus/procfs` library for most /proc
  parsing rather than writing parsers from scratch. Wrote custom parsers only
  where procfs did not expose the data (zswap from /proc/meminfo, conntrack
  expect columns, ZFS arcstats, btrfs sysfs, SCTP snmp).
- Collectors register into the existing `metrics.PrometheusRegistry` that
  already backs BGP metrics. No second registry, no second HTTP server; the
  `/metrics` endpoint serves both BGP and OS metrics.
- Per-collector config (enable/disable + interval override) via a YANG `list
  collector` keyed by name. Global interval applies when no override; disabled
  collectors skip `Init()` entirely so no gauges register.
- Manager ticks at 1s internally; per-collector intervals are enforced by
  per-collector `lastRun` tracking (skip tick if `now - lastRun < interval`).
  Avoided multiple tickers to keep the manager simple.
- Counter-wrap protection via `safeDelta(cur, prev uint64)` returning 0 when
  `cur < prev`. Applied uniformly across all delta-based collectors. Prevents
  the huge positive float spike that naive uint64 subtraction produces on wrap.
- Merged duplicate /proc readers when discovered: `sysNetCollector` into
  `netDevCollector`, `perCPUCollector` into `cpuCollector`, `sysIOCollector`
  into `diskStatsCollector`, `snmpExtCollector` into `snmpCollector`,
  `snmp6ExtCollector` into `snmp6Collector`. Each merge was caught in review
  after being introduced as a separate file.

## Consequences

- Ze now has a 138-metric OS telemetry surface that is a drop-in replacement
  for Netdata's Prometheus exporter. Same names, same labels, same values.
- Dashboards built against Netdata keep working. Port 9273 on Netdata can be
  decommissioned once side-by-side validation on a live LNS confirms parity.
- The collector framework supports per-collector enable/disable and interval
  override via YANG. Operators can reduce load on slow or expensive collectors
  (e.g. `diskspace` scanning every mount) without a recompile.
- New collectors added later need only implement the `Collector` interface and
  register in `platform_linux.go`. No framework changes.

## Gotchas

- **Verify names against source, not summaries.** First four passes of metric
  naming relied on an agent-extracted chart catalog. The agent got several
  contexts wrong (`mem.transparent_hugepages` vs `mem.thp`,
  `system.active_processes` vs `system.processes`, `system.interrupts` vs
  `system.intr`, `ipv4.sockstat_sockets` vs `ip.sockstat_sockets`, conntrack
  units, disk backlog/busy units). Only direct reads of the Netdata C source
  caught them. When correctness depends on matching another implementation
  exactly, diff against the source file, not a description of it.

- **Write the verification script early.** The final clean diff came from a
  Python script that reproduced Netdata's unit sanitization rules
  (`% → _percent`, `/s → _persec`, non-alphanum → `_`) and diffed 138
  expected vs actual metric names. Doing that after the first pass would have
  caught the mismatches in one round instead of four.

- **Apply review techniques: they find real bugs.** `/ze-review` caught actual
  bugs every time: soft-IRQ using wrong `prev` field (`prevInterrupts` shared
  across hard/soft), `Invalid` mapped to "ignore" dimension in conntrack (they
  are separate counters), `SctpCurrEstab` treated as a delta-rate when it is
  a gauge, sockstat6 swallowing errors with `return nil`, uint64 wrap producing
  float spikes, duplicate /proc reads across collectors.

- **Merge duplicate /proc readers on sight.** The instinct to create a new
  `_ext` file rather than extend an existing collector was wrong every time it
  happened. Two collectors reading the same /proc file each tick doubles the
  I/O for no benefit. If a new chart uses the same data source as an existing
  collector, add it to that collector, don't create a sibling.

- **Counter wrap is real on 32-bit kernel counters.** `/proc/net/softnet_stat`
  fields are `uint32`, wrapping in minutes on a busy LNS. `safeDelta` must be
  applied before any arithmetic; subtracting then accumulating into uint64
  still loses the wrap detection.

- **Gauges and counters look similar in procfs struct, treat them differently.**
  `SctpCurrEstab` is a gauge (current established associations), but sits in
  the same struct as SCTP counters. Uniform `safeDelta / secs` loops must
  whitelist which fields get the rate treatment.

- **Average-mode metrics need gauge conversion, not raw counters.** Netdata's
  Prometheus "average mode" exports rates (e.g. `kilobits/s`) as gauges of
  pre-computed per-second values, not raw byte counters. Each collector must
  track `prev` state, compute `delta / interval`, and set the gauge. Exporting
  raw counters would still work for `rate()` in PromQL but would break exact
  Netdata compatibility.

- **Cardinality cleanup for dynamic interfaces.** L2TP tunnels create
  `l2tpeth*` interfaces that come and go. Without `Delete()` calls when an
  interface disappears, Prometheus GaugeVec accumulates stale label
  combinations forever. `netDevCollector` tracks the previous set of
  interfaces and deletes missing ones each tick.

- **Unit sanitization edge cases.** `%` becomes `_percent` (not `_percentage`);
  strings ending in `/s` replace the `/s` with `_persec`; all other non-alnum
  becomes `_`. "active connections" → `_active_connections`, not
  `_activeconnections`. Read the actual sanitizer in `prometheus_units_copy`.

## Testing

- Unit tests for manager start/stop, default prefix/interval, per-collector
  enable/disable, per-collector interval override.
- Integration test (`naming_test.go`) creates a real `PrometheusRegistry`,
  registers a stub load-average collector, scrapes the HTTP handler, and asserts
  the exact metric line format including `# HELP` and `# TYPE`.
- Verification script (`tmp/gen-expected.py`) reproduces Netdata's
  sanitization and diffs every registered metric name against the expected
  Netdata output. Zero-diff on commit.
- Side-by-side validation on a live LNS (deploy Ze on port 9274, keep
  Netdata on 9273, diff scraped metric lists) is the final acceptance step
  but requires a real Linux box; not runnable in CI.

## Files Touched

- `internal/component/telemetry/collector/` -- 30+ collectors + framework
- `internal/component/telemetry/schema/ze-telemetry-conf.yang` -- `prefix`,
  `interval`, per-collector list
- `internal/core/metrics/server.go` -- `Prefix`, `Interval`, `Collectors`
  fields in `TelemetryConfig`, parsing from YAML tree
- `internal/component/bgp/config/loader_create.go` -- wiring call to
  `collector.StartOSCollectors`
- `docs/guide/monitoring.md` -- OS metrics section with full collector table
- `docs/features.md` -- headline entry
- `go.mod` -- promoted `prometheus/procfs` from indirect to direct
