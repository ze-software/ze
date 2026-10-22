# Benchmarking

Ze includes `le perf`, a tool for measuring BGP route propagation latency
through a device under test (DUT). It works with any BGP implementation,
including Ze.

<!-- source: internal/perf/cli/register.go -- perf CLI entry point -->

`le perf send` runs one benchmark against a DUT, `le perf report` compares
result files, and `le perf track` follows the history. Every word after the verb reaches the program
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
| FRR | quay.io/frrouting/frr:10.7.1, pinned by digest (`interoplab.FRRImage`) | `test/perf/configs/frr.conf` | route-map PERMIT |
| BIRD | bird-interop (built) | `test/perf/configs/bird.conf` | import/export all |
| GoBGP | gobgp-interop (built) | `test/perf/configs/gobgp.toml` | default accept policy |
| rustbgpd | rustbgpd-interop (built from source) | `test/perf/configs/rustbgpd.toml` | route_server_client |
| rustybgp | rustybgp-interop (built) | `test/perf/configs/rustybgp.toml` | default accept policy |
| FreeRtr | freertr-interop (built) | `test/perf/configs/freertr-sw.txt` | eBGP + redistribute connected |
| OpenBGPD | openbgpd-interop (built) | `test/perf/configs/openbgpd.conf` | allow from/to any |

The Ze DUT runs on the Docker daemon's kernel, so a run that tests it first
builds `test/interop/ze-linux`, the ze the image carries, for the daemon's
architecture, then checks that kernel with it and refuses one that lacks any
feature Ze enrolls, naming each. The check and its
routes are in `docs/architecture/testing/interop.md`, "The Docker host kernel
check".

<!-- source: internal/test/perfrunner/run.go -- native Docker benchmark runner -->
<!-- source: internal/le/perf/bench.go -- Bench.Run -->
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

An image that fails to build or pull fails the run, and the run prints an error
naming the DUT and the cause. In a `step build` run the image is the whole
answer, so the run names every DUT that has no image, prints the rebuild
command, exits 1, and never reports the images as built. In a run that also
measures, the test step still measures every DUT whose image was made, but it
never measures a DUT whose build failed, even from an image an earlier run
left. The summary line reads `FAIL`, counts that DUT as failed, gives its
cause, and the run exits 1.

A DUT is skipped only when the run built no image (`NO_BUILD=1`, or
`step test`) and none exists. The summary names the skipped DUTs and the
`./le perf run step build dut "..."` command that builds them. Under
`NO_BUILD=1` that command is `env -u NO_BUILD ./le perf run step build dut
"..."`, because the plain form would build nothing. Skips beside a measured DUT
still pass, but a run that measured no DUT prints `FAIL  no DUT was measured`
and exits 1, because a run of skips has no result.

`NO_BUILD=1` with `step build` is refused: the run would build nothing, so it
names `NO_BUILD` as the reason, prints the `env -u NO_BUILD ./le perf run step
build` command that unsets it, and exits 1 without claiming any image was built.
<!-- source: internal/test/perfrunner/run.go -- Runner.buildImages, Runner.buildCommand, Execute, Runner.measure, Runner.summarize, measurement.failedRun -->

The runner builds no host benchmark program. It cross-builds `le` for linux
with `CGO_ENABLED=0` into `tmp/perf-run/linux-<arch>/le`, and prints how long
that build took. It mounts the file at `/usr/local/bin/le` in the sender
container, which runs `le perf send`. The `le` that started the run renders
the report as `le perf report`. The terminal-demo recorder container runs a
linux `le` built by the same recipe (`docs/contributing/gh-pages.md`).
<!-- source: internal/test/perfrunner/run.go -- Execute -->
<!-- source: internal/le/linuxle/linuxle.go -- Tags -->

Every container, network and image tag the run creates carries the lab's
owner labels. Ctrl-C (SIGINT) or SIGTERM stops the run, and the run still
removes the DUT, the sender and the network before it exits 1. A removal that
fails is printed on stderr with the resource's name, and the run exits 1; a
run that died before its removals (SIGKILL, a crash) leaves resources that
`./le test integration cleanup`, or the next lab start, removes. The DUT
images themselves are kept, so the next `step test` finds them.
<!-- source: internal/test/perfrunner/run.go -- Runner.removeContainer, Runner.cleanup -->
<!-- source: internal/le/perf/actions.go -- signalRooted -->

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

### Before you report a number

`le perf send` already discards the warmup run, drops outliers and reports a
median with its standard deviation. That makes one run repeatable. It does
not say what the number measured, and a result that looks like a win can be a
measurement of something else. Answer these before you report a number, act on
it, or pick between two builds or two implementations.

1. **Write the claim first**, in the words you would ship: "ze converges 100k
   IPv4 routes 25% faster with change X". The questions below test that
   sentence.
2. **Why not twice as good?** Name what bounds the result: one core, a lock,
   the socket, the sender, or the receiver. Read it from per-process CPU
   (`top`, `pidstat`) or a CPU profile taken during a run you do not report.
   Start ze with `--pprof 127.0.0.1:6060` to profile it. If the sender or the
   receiver saturates first, you measured `le perf`, not the DUT.
3. **Was every side tuned?** Compare release builds with the same flags, the
   same family and encoding mode, and the same route count. When one side runs
   on its defaults, you compared configurations, not implementations.
4. **Did the work happen?** Check that `routes-lost` is zero and that
   `routes-received` matches `routes-sent`. A run that dropped or rejected routes
   is often the fastest run.
5. **Is the gap bigger than the noise?** To compare two builds, alternate them:
   A, B, A, B, at least five times each, appending every `--json` result. Then
   warm caches and machine drift favour neither side. A gap smaller than the
   spread between runs of the same build is no measurable difference.
6. **Does it matter end to end?** A change to a step that takes 10% of
   convergence can make convergence at most about 11% faster. A result past
   that limit measured a cache, a no-op or a bug.

Report the verdict first (faster, slower, no measurable difference, or
inconclusive). Then give the number with its unit, the run count, the spread
and the bound you found. Without a run count, a spread and a bound, call the
result inconclusive and say which answer is missing.
