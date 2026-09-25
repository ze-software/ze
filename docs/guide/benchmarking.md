# Benchmarking

Ze includes `le perf`, a tool for measuring BGP route propagation latency
through a device under test (DUT). It works with any BGP implementation,
including Ze.

<!-- source: internal/perf/cli/register.go -- perf CLI entry point -->

`le perf send`, `le perf report` and `le perf track` run what `ze-perf run`,
`ze-perf report` and `ze-perf track` run. The separate `ze-perf` binary keeps
working until it is removed. Every word after the verb reaches the program
unchanged, and the program's exit code is le's.
<!-- source: internal/le/perf/actions.go -- programVerbs -->

## Architecture

```
                  +-----------+
  le perf send    |           |         le perf send
  (sender)  ----> |    DUT    | ---->  (receiver)
  AS 65001        |  AS 65000 |         AS 65002
                  +-----------+
```

The sender establishes a BGP session with the DUT, injects routes, and the
receiver measures when those routes arrive. Both sessions are managed by
`le perf send` in a single process. Timing starts when the sender writes the first
UPDATE and stops when the receiver has collected all expected prefixes.

<!-- source: internal/perf/sender.go -- sender UPDATE construction -->
<!-- source: internal/perf/receiver.go -- receiver prefix extraction -->

## Quick Start

Run a benchmark against Ze on localhost:

```bash
# Start ze with a config that accepts peers from 127.0.0.1 and 127.0.0.2
ze start test-config.conf &

# Run the benchmark
./le perf send --dut-addr 127.0.0.1 --dut-asn 65000 --dut-name ze --routes 1000
```

With JSON output saved to a file:

```bash
./le perf send --dut-addr 127.0.0.1 --dut-asn 65000 --dut-name ze \
  --routes 1000 --output result-ze.json
```

## Running Benchmarks

For a complete flag reference, see [le perf send](command-reference.md#le-perf-send).

### Encoding Modes

Three encoding modes measure different code paths through the DUT:

| Mode | Flag | What It Tests |
|------|------|---------------|
| IPv4 inline NLRI | `--family ipv4/unicast` (default) | Standard IPv4 unicast path with NLRI at the end of the UPDATE |
| IPv4 force-MP | `--family ipv4/unicast --force-mp` | IPv4 encoded via MP_REACH_NLRI attribute (exercises multiprotocol path) |
| IPv6 MP | `--family ipv6/unicast` | IPv6 unicast via MP_REACH_NLRI (standard for non-IPv4 families) |

<!-- source: internal/perf/cli/cmd_run.go -- family and force-mp validation -->

### Multi-Iteration

By default, `le perf send` executes 5 iterations with 1 warmup run. The warmup
run is discarded, and outliers beyond 2 standard deviations from the median
convergence time are removed (minimum 3 iterations kept). Final results report
median and standard deviation across the kept iterations.

<!-- source: internal/perf/metrics.go -- RemoveOutliers and Aggregate -->

| Flag | Default | Purpose |
|------|---------|---------|
| `--repeat` | `5` | Total iterations (including warmup) |
| `--warmup-runs` | `1` | Warmup iterations discarded from results |
| `--iter-delay` | `3s` | Pause between iterations for clean separation |

More iterations improve statistical confidence. For reliable results, use at
least `--repeat 10`:

```bash
./le perf send --dut-addr 172.31.0.2 --dut-asn 65000 --repeat 10 --warmup-runs 2
```

### Timing

| Flag | Default | Purpose |
|------|---------|---------|
| `--warmup` | `2s` | Delay after session establishment before injecting routes |
| `--connect-timeout` | `10s` | TCP connection timeout |
| `--duration` | `60s` | Maximum wait time for convergence per iteration |
| `--iter-delay` | `3s` | Delay between iterations |

The `--iter-delay` flag controls whether iterations run back-to-back or with
a pause. Longer delays give the DUT time to settle between measurements.

## Cross-Implementation Comparison

### Automated Docker Runner

The included test runner benchmarks all eight supported implementations in Docker:

| DUT | Image | Config | Forwarding mechanism |
|-----|-------|--------|---------------------|
| Ze | ze-interop (built) | `test/perf/configs/ze.conf` | bgp-rs plugin |
| FRR | quay.io/frrouting/frr:10.3.1 | `test/perf/configs/frr.conf` | route-map PERMIT |
| BIRD | bird-interop (built) | `test/perf/configs/bird.conf` | import/export all |
| GoBGP | gobgp-interop (built) | `test/perf/configs/gobgp.toml` | default accept policy |
| rustbgpd | rustbgpd-interop (built from source) | `test/perf/configs/rustbgpd.toml` | route_server_client |
| rustybgp | rustybgp-interop (built) | `test/perf/configs/rustybgp.toml` | default accept policy |
| FreeRtr | freertr-interop (built) | `test/perf/configs/freertr-sw.txt` | eBGP + redistribute connected |
| OpenBGPD | openbgpd-interop (built) | `test/perf/configs/openbgpd.conf` | allow from/to any |

<!-- source: internal/test/perfrunner/run.go -- native Docker benchmark runner -->
<!-- source: test/interop/Dockerfile.rustbgpd -- rustbgpd Docker image -->

```bash
# Build every DUT image, measure every DUT, and record the run.
./le perf run

# Measure one DUT, or several.
./le perf run dut ze
./le perf run dut "ze bird rustbgpd"

# Run one step: build the images only, or measure with the images that exist.
./le perf run step build
DUT_ROUTES=10000 DUT_REPEAT=5 ./le perf run step test dut ze

# Append the results of the last measurement to the committed NDJSON history.
./le perf history-record

# The release evidence gate: measure ze, append the result, fail on a regression.
./le perf evidence-record
```
<!-- source: internal/le/perf/actions.go -- runParameters -->
<!-- source: internal/le/perf/bench.go -- Bench -->

A run with no `step` keyword builds the images and measures. `step build`
records nothing, because it measured nothing. Any other `step` value exits 2
and names the value. A run that measured records the commit, so the perf nudge
(`./le perf suggest`) stops asking for one.

The runner builds no host benchmark program. It cross-builds `le` for linux
with `CGO_ENABLED=0` into `tmp/perf-run/linux-<arch>/le`, and prints how long
that build took. It mounts the file at `/usr/local/bin/le` in the sender
container, which runs `le perf send`. The `le` that started the run renders
the report as `le perf report`. The terminal-demo recorder container runs a
linux `le` built by the same recipe (`docs/contributing/gh-pages.md`).
<!-- source: internal/test/perfrunner/run.go -- Execute -->
<!-- source: internal/le/linuxle/linuxle.go -- Tags -->

`go run ./cmd/ze-perf-run --build --test [<dut>...]` drives the same runner
until it is removed. It still renders the report with `bin/ze-perf`, which
`ZE_PERF_BIN` can move.
<!-- source: cmd/ze-perf-run/main.go -- main -->
<!-- source: internal/test/perfrunner/run.go -- RunCLI -->

Results are written to `test/perf/results/` as JSON files. An HTML comparison report is generated automatically.

### Manual Reports

After running benchmarks, generate reports from the result files:

```bash
# Markdown report (default)
./le perf report result-ze.json result-gobgp.json result-rustbgpd.json

# HTML report
./le perf report --html result-ze.json result-gobgp.json > comparison.html
```

<!-- source: internal/perf/cli/cmd_report.go -- report subcommand -->
<!-- source: internal/perf/report/markdown.go -- Markdown report generation -->
<!-- source: internal/perf/report/html.go -- HTML report generation -->

For the full flag reference, see [le perf report](command-reference.md#le-perf-report).

## Tracking Performance Over Time

### NDJSON History

Each `le perf send --json` invocation produces a single JSON object. Append
results to an NDJSON (newline-delimited JSON) file to build a history:

```bash
./le perf send --dut-addr 127.0.0.1 --dut-asn 65000 --json >> history.ndjson
```

<!-- source: internal/perf/result.go -- ReadNDJSON and WriteNDJSON -->

### Trend Reports

Generate a trend report from a history file:

```bash
./le perf track history.ndjson
./le perf track --html history.ndjson > trend.html
```

<!-- source: internal/perf/cli/cmd_track.go -- track subcommand -->
<!-- source: internal/perf/report/trend.go -- trend report generation -->

### Regression Detection

Use `--check` in CI to detect performance regressions. The tool compares the
most recent entry against the previous one using stddev-aware thresholds:

```bash
./le perf track --check history.ndjson
```

<!-- source: internal/perf/regression.go -- CheckRegression and CheckHistory -->

Exit code 0 means no regression. Exit code 1 means a regression was detected.

Regressions are flagged when a metric exceeds its threshold percentage AND the
delta exceeds the combined standard deviation of the two measurements. This
prevents false positives from noisy measurements.

Default thresholds:

| Metric | Default Threshold | Direction |
|--------|-------------------|-----------|
| Convergence time | 20% | Higher is worse |
| Throughput (avg) | 20% | Lower is worse |
| P99 latency | 30% | Higher is worse |
| Routes lost | Any loss | Always flagged |

<!-- source: internal/perf/regression.go -- DefaultThresholds -->

Custom thresholds:

```bash
./le perf track --check --threshold-convergence 15 --threshold-throughput 10 --threshold-p99 25 history.ndjson
```

Limit the comparison window to the last N entries with `--last`:

```bash
./le perf track --check --last 5 history.ndjson
```

For the full flag reference, see [le perf track](command-reference.md#le-perf-track).

## Understanding Results

### JSON Output Fields

The JSON result object contains these key fields:

<!-- source: internal/perf/result.go -- Result struct -->

| Field | Unit | Description |
|-------|------|-------------|
| `convergence-ms` | ms | Time from first UPDATE sent to last route received (median) |
| `convergence-stddev-ms` | ms | Standard deviation of convergence across iterations |
| `first-route-ms` | ms | Time to first route arrival |
| `throughput-avg` | routes/s | Average route propagation rate |
| `throughput-avg-stddev` | routes/s | Standard deviation of throughput |
| `throughput-peak` | routes/s | Peak routes/s in any 1-second window |
| `latency-p50-ms` | ms | 50th percentile per-route latency |
| `latency-p90-ms` | ms | 90th percentile per-route latency |
| `latency-p99-ms` | ms | 99th percentile per-route latency |
| `latency-p99-stddev-ms` | ms | Standard deviation of P99 across iterations |
| `latency-max-ms` | ms | Maximum per-route latency |
| `routes-sent` | count | Routes injected by sender |
| `routes-received` | count | Routes received by receiver |
| `routes-lost` | count | Difference between sent and received |
| `repeat` | count | Total iterations run |
| `repeat-kept` | count | Iterations kept after outlier removal |

### Interpreting Results

**Convergence time** is the primary metric. It measures how long the DUT takes
to forward all injected routes from sender to receiver. Lower is better.

**Standard deviation** indicates measurement stability. A high stddev relative
to the median suggests noisy measurements. Increase `--repeat` and
`--iter-delay` for more stable results.

**Throughput** shows sustained forwarding rate. The average is computed over the
full convergence window. The peak shows the maximum 1-second burst rate.

<!-- source: internal/perf/metrics.go -- CalculateThroughput -->

**Latency percentiles** show per-route propagation time distribution. P50 is
the typical latency, P99 captures tail latency, and max shows the worst case.

<!-- source: internal/perf/metrics.go -- CalculateLatencies -->

**Routes lost** should always be zero. Any loss indicates the DUT dropped
routes, which the regression checker always flags.
