# Spec: ze-perf

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/chaos/peer/session.go` - BGP session setup pattern
4. `internal/chaos/peer/sender.go` - UPDATE building pattern
5. `internal/chaos/peer/simulator_reader.go` - UPDATE parsing pattern (NOTE: parseUpdatePrefixes takes *EventBuffer, not reusable as-is -- ze-perf must reimplement prefix extraction)
6. `internal/chaos/scenario/routes.go` - route generation pattern
7. `test/interop/` - Docker infrastructure for DUT testing
8. `internal/component/bgp/message/update_build.go:177-291` - BuildUnicast IPv4 inline vs MP_REACH_NLRI decision

## Task

Build `ze-perf`, a BGP propagation latency benchmarking tool that measures how long a Device Under Test (DUT) takes to forward routes from a sender peer to a receiver peer. Supports cross-implementation comparison (Ze, GoBGP, FRR, BIRD) with structured reporting and historical regression tracking.

### Architecture

ze-perf (sender) establishes a BGP session with the DUT, announces routes, while ze-perf (receiver) establishes a second BGP session and records when those routes arrive. Propagation time = t_received - t_sent.

Ze-perf is a single binary with three subcommands:

| Subcommand | Purpose |
|------------|---------|
| `ze-perf run` | Single benchmark run against one DUT, outputs JSON |
| `ze-perf report` | Aggregates multiple JSON results into cross-implementation comparison |
| `ze-perf track` | Reads NDJSON history file, shows performance over time, detects regressions |

### Design Decisions

- **TCP-only**: no in-process mode. Fair cross-implementation comparison requires real network.
- **Duplication over abstraction**: ze-perf writes its own lean peer simulation, importing the same wire packages (`component/bgp/message`, `capability`, `attribute`, `nlri`) as ze-chaos but not importing ze-chaos code. The overlap is small and the purposes differ (timing vs correctness).
- **Two goroutines**: sender and receiver run concurrently in one process. Sender announces routes, receiver parses incoming UPDATEs and records arrival times.
- **Prefix-only matching**: sender records `map[prefix]time.Time` at send. Receiver records arrival time per prefix. Matching is on prefix only, not full route attributes -- DUT may modify AS_PATH, NEXT_HOP, etc. If a DUT aggregates prefixes, the test is invalid (DUT configs must not aggregate).
- **Deterministic routes**: same seed + route count = same prefixes. Reproducible across runs.
- **Multiple iterations by default**: `--repeat N` (default 5) runs the benchmark N times, reports median and stddev. `--warmup-runs` (default 1) runs throwaway iterations first to warm caches. Single-run noise is not a benchmark.
- **Aggregated results**: each JSON result stores the median and stddev of N iterations, not individual run data. Individual runs are noise; the aggregate is the signal.
- **Outlier removal**: after all measured iterations complete, discard any run whose convergence time is beyond 2 standard deviations from the median, then recompute all stats from the remaining runs. This prevents a single CPU spike or GC pause from inflating stddev. The JSON result records how many runs were kept vs discarded.
- **History is append-only NDJSON**: one JSON object per line per benchmark (N iterations aggregated into one result). Small (~600 bytes/entry), committed to repo.
- **TCP_NODELAY on both connections**: disables Nagle's algorithm so small UPDATEs are sent immediately. Without this, TCP may coalesce writes and make latency look artificially low.
- **Timestamp after socket write returns**: sender records time after `conn.Write()` returns, not before. This measures DUT processing time excluding local socket buffer fill time. Receiver records time on prefix extraction from the read buffer.
- **Three test modes for encoding overhead**: ipv4/unicast (non-MP, legacy NLRI field), ipv4/unicast with `--force-mp` (same AFI=1/SAFI=1 but encoded in MP_REACH_NLRI attribute), ipv6/unicast (MP_REACH_NLRI with 128-bit prefixes). Comparing ipv4 non-MP vs ipv4 MP isolates the MP attribute overhead for identical prefixes. IPv6 adds address-size overhead on top of MP.
- **Human output to stderr, JSON to stdout**: allows piping JSON to file while seeing human progress. `--json` switches stdout from human summary to JSON.

### MP vs Non-MP Encoding Paths

IPv4/unicast has two valid wire encodings per the BGP spec:
1. **Inline NLRI** (RFC 4271): prefixes in the UPDATE NLRI field, next-hop in NEXT_HOP attribute
2. **MP_REACH_NLRI** (RFC 4760): prefixes in attribute type 14, next-hop inside the attribute

Ze's `UpdateBuilder.BuildUnicast` (update_build.go:276-291) chooses inline for IPv4/unicast by default. There is no clean code path through UpdateBuilder to force IPv4/unicast into MP_REACH_NLRI with an IPv4 next-hop -- the decision tree requires either IPv6 prefix, non-unicast SAFI, or extended next-hop with IPv6 NH.

The `--force-mp` flag makes ze-perf construct the MP_REACH_NLRI attribute directly (bypassing UpdateBuilder for that attribute), encoding IPv4 prefixes with AFI=1/SAFI=1 inside attribute type 14. The rest of the UPDATE (ORIGIN, AS_PATH, etc.) still uses the builder. This is ~30 lines of manual attribute construction in `sender.go`.

| Mode | Encoding path | What it measures |
|------|---------------|-----------------|
| `--family ipv4/unicast` | Inline NLRI field + NEXT_HOP attr | Baseline, fastest path |
| `--family ipv4/unicast --force-mp` | MP_REACH_NLRI (AFI=1/SAFI=1) | MP attribute overhead for same IPv4 prefixes |
| `--family ipv6/unicast` | MP_REACH_NLRI (AFI=2/SAFI=1) | Address size overhead (128-bit) on top of MP |

Both sender and receiver negotiate the multiprotocol capability in OPEN for all modes. The receiver must parse both inline NLRI and MP_REACH_NLRI to extract prefixes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - chaos simulator architecture (pattern reference)
  -> Constraint: ze-perf imports wire packages directly, not chaos packages
- [ ] `docs/architecture/core-design.md` - BGP subsystem overview
  -> Decision: ze-perf is a standalone tool, not a plugin or subsystem component
- [ ] `docs/architecture/update-building.md` - UPDATE building, UnicastParams
  -> Constraint: BuildUnicast uses inline NLRI for IPv4/unicast, MP_REACH for IPv6 and non-unicast

### Source Files (pattern reference, not import)
- [ ] `internal/chaos/peer/session.go` (137L) - OPEN building, capability negotiation
  -> Decision: ze-perf duplicates this pattern, ~50 lines for session setup
- [ ] `internal/chaos/peer/sender.go` (228L) - UPDATE building for all families
  -> Decision: ze-perf uses BuildUnicast with UnicastParams for all three modes
- [ ] `internal/chaos/peer/simulator.go` (507L) - peer loop with keepalive
  -> Decision: ze-perf needs a much simpler loop (no chaos, no route dynamics)
- [ ] `internal/chaos/peer/simulator_reader.go` (378L) - UPDATE parsing, prefix extraction
  -> Constraint: parseUpdatePrefixes takes *EventBuffer param, NOT extractable as-is. Ze-perf must reimplement prefix extraction (~100 lines: parse withdrawn, walk attributes for MP_REACH/MP_UNREACH, parse inline NLRI)
- [ ] `internal/chaos/scenario/routes.go` - deterministic IPv4 route generation
  -> Decision: ze-perf duplicates the generation algorithm (~40 lines per family)
- [ ] `internal/component/bgp/message/update_build.go` (400+L) - UnicastParams, BuildUnicast
  -> Constraint: No clean code path for IPv4/unicast + IPv4 NH through MP_REACH. `--force-mp` must construct MP_REACH_NLRI attribute directly, bypassing UpdateBuilder for that attribute only

### Interop Infrastructure (DUT deployment)
- [ ] `test/interop/interop.py` - Docker container management, daemon query classes
  -> Decision: ze-perf uses a different subnet (172.31.0.0/24) to avoid conflicts with interop tests
- [ ] `test/interop/scenarios/18-ebgp-gobgp/` - GoBGP config example
  -> Constraint: DUT configs must set up two peers with route redistribution/forwarding

**Key insights:**
- Wire packages (`component/bgp/message`, `capability`, `attribute`, `nlri`) are the shared layer
- Chaos peer code is pattern reference, not a dependency
- Interop uses Docker network 172.30.0.0/24 -- ze-perf uses 172.31.0.0/24 to avoid conflicts
- Each DUT needs a config with two eBGP peers that forwards routes between them
- Prefix extraction from UPDATEs is ~100 lines, not trivially extractable from chaos code

## Current Behavior (MANDATORY)

**Source files read:** (no existing ze-perf code)
- [ ] `cmd/ze-chaos/main.go` - chaos CLI structure (pattern reference)
- [ ] `internal/chaos/peer/session.go` - current session setup
- [ ] `internal/chaos/peer/sender.go` - current UPDATE building
- [ ] `internal/chaos/peer/simulator_reader.go` - current prefix extraction (EventBuffer-coupled)
- [ ] `internal/chaos/scenario/routes.go` - current route generation
- [ ] `internal/component/bgp/message/update_build.go` - UnicastParams, MP vs inline decision

**Behavior to preserve:**
- Interop Docker network layout (172.30.0.0/24) -- ze-perf uses separate subnet
- Wire package APIs (ze-perf is a consumer, does not modify them)
- ze-chaos code (ze-perf does not touch it)

**Behavior to change:**
- None -- this is a new tool

## Data Flow (MANDATORY)

### Entry Point
- CLI flags specify DUT address, ASN, route count, seed, family
- ze-perf establishes two TCP connections to the DUT

### Transformation Path

**`ze-perf run`:**
1. Parse CLI flags into run config
2. For each iteration (warmup-runs + repeat):
   a. Connect receiver peer to DUT first (TCP, BGP OPEN exchange with matching family capabilities, KEEPALIVE) -- receiver connects first so it is ready to capture routes before sender starts; avoids race where routes arrive before receiver OPEN completes
   b. Connect sender peer to DUT (TCP, BGP OPEN exchange with matching family capabilities, KEEPALIVE)
   c. Set TCP_NODELAY on both connections
   d. Wait for warmup period (both sessions established)
   e. Sender: generate N deterministic prefixes, build UPDATEs (inline NLRI or MP depending on family/force-mp), send. Record `map[prefix]time.Time` AFTER each conn.Write returns
   f. Receiver: read BGP messages, parse UPDATEs, extract prefixes from inline NLRI and/or MP_REACH_NLRI, record `map[prefix]time.Time` on extraction
   g. Wait for convergence (all N prefixes received) or timeout
   h. Join sender/receiver maps on prefix, compute per-route latency for this iteration
   i. Send NOTIFICATION Cease, close both connections
   j. Discard result if warmup iteration; collect result if measured iteration
   k. Wait iter-delay before next iteration (0 = immediate reconnect)
3. Outlier removal: compute median and stddev of convergence-ms across measured iterations; discard any iteration whose convergence-ms is beyond 2 stddev from the median
4. Compute median and stddev across remaining iterations for each metric
5. Output aggregated JSON to stdout (human progress for each iteration to stderr)

**`ze-perf report`:**
1. Read multiple JSON result files from disk
2. Parse into result structs
3. Sort by DUT name
4. Render comparison tables (markdown or HTML)

**`ze-perf track`:**
1. Read NDJSON history file
2. Parse into time-ordered result list
3. Compare consecutive runs, compute deltas
4. Detect regressions (configurable thresholds)
5. Render trend tables (markdown or HTML) or exit non-zero for CI

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze-perf <-> DUT | TCP, BGP wire protocol (OPEN, KEEPALIVE, UPDATE) | [ ] |
| Results <-> Disk | JSON files (run output), NDJSON (history) | [ ] |
| Results <-> Report | Read JSON, render markdown/HTML | [ ] |

### Integration Points
- `internal/component/bgp/message/` - UPDATE/OPEN building, UpdateBuilder, UnicastParams
- `internal/component/bgp/attribute/` - path attribute encoding, MPReachNLRI
- `internal/component/bgp/capability/` - OPEN capabilities (Multiprotocol per family)
- `internal/component/bgp/nlri/` - family constants, AFI/SAFI

### Architectural Verification
- [ ] No bypassed layers (ze-perf uses wire packages directly, same as ze-chaos)
- [ ] No unintended coupling (ze-perf does not import chaos packages)
- [ ] No duplicated functionality (ze-perf duplicates small amounts intentionally, per design decision)
- [ ] Zero-copy preserved where applicable (wire encoding uses WriteTo pattern)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-perf run --dut-addr ... --routes 10` | -> | sender/receiver/metrics | `TestRunSmallBenchmark` in `internal/perf/perf_test.go` (uses a trivial BGP forwarder goroutine as DUT) |
| `ze-perf report *.json --md` | -> | report/markdown.go | `TestMarkdownReport` in `internal/perf/report/report_test.go` |
| `ze-perf report *.json --html` | -> | report/html.go | `TestHTMLReport` in `internal/perf/report/report_test.go` |
| `ze-perf track history.ndjson --check` | -> | regression.go | `TestRegressionDetection` in `internal/perf/regression_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-perf run` with valid DUT | Establishes two BGP sessions, sends routes, measures propagation, outputs JSON to stdout |
| AC-2 | `ze-perf run --routes 100 --seed 42` twice | Same prefixes generated, results reproducible within noise |
| AC-3 | `ze-perf run` with unreachable DUT | Exits non-zero with error to stderr within connection timeout |
| AC-4 | `ze-perf run` DUT does not forward routes | Exits with code 1, JSON contains routes-sent > 0 AND routes-received < routes-sent AND convergence-ms equals duration timeout |
| AC-5 | `ze-perf report` with 2+ JSON files | Produces comparison table with all DUTs, sorted by convergence time |
| AC-6 | `ze-perf report --html` | Produces self-contained HTML with styled tables and inline SVG bar charts |
| AC-7 | `ze-perf report --md` | Produces valid CommonMark markdown with comparison tables |
| AC-8 | `ze-perf track` with NDJSON history | Shows trend table with per-run metrics and deltas |
| AC-9 | `ze-perf track --check` with regression | Exits non-zero, prints regression details to stderr |
| AC-10 | `ze-perf track --check` with two runs within threshold | Exits zero AND prints "no regression" to stderr |
| AC-11 | JSON output from `ze-perf run` | Contains at least these keys: dut-name, convergence-ms, throughput-avg, latency-p50-ms, all kebab-case |
| AC-12 | `ze-perf run --dut-version` | Version string included in JSON output for history tracking |
| AC-13 | `ze-perf run --family ipv4/unicast --force-mp` | UPDATE wire bytes contain attribute type 14 (MP_REACH_NLRI) with AFI=1/SAFI=1, and do NOT contain trailing NLRI field |
| AC-14 | `ze-perf run --family ipv6/unicast` | Sends IPv6 prefixes via MP_REACH_NLRI |
| AC-15 | `ze-perf run -h` | Prints usage with flag descriptions and examples |
| AC-16 | `ze-perf run --repeat 5` | Runs 5 measured iterations, JSON contains median and stddev for each metric |
| AC-17 | `ze-perf run --warmup-runs 2 --repeat 3` | Runs 2 throwaway + 3 measured iterations, only measured results in JSON |
| AC-18 | `ze-perf run --repeat 1` against forwarder | JSON contains repeat=1, repeat-kept=1, convergence-stddev-ms=0, AND convergence-ms > 0, routes-received > 0 |
| AC-19 | `ze-perf run --repeat 5` with one outlier iteration | Outlier discarded, repeat-kept < repeat, stats computed from remaining |
| AC-20 | `ze-perf run --repeat 3 --iter-delay 0` against forwarder | All 3 iterations complete successfully, JSON contains iter-delay-ms=0 AND routes-received > 0 |
| AC-21 | `ze-perf run --repeat 2 --iter-delay 1s` | Total wall-clock time is at least 1s longer than with --iter-delay 0 for same workload, JSON contains iter-delay-ms=1000 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildOpen` | `internal/perf/session_test.go` | OPEN message with correct capabilities per family | |
| `TestBuildOpenIPv6` | `internal/perf/session_test.go` | OPEN for ipv6/unicast negotiates multiprotocol AFI=2/SAFI=1 | |
| `TestBuildRoutes` | `internal/perf/routes_test.go` | Deterministic route generation from seed | |
| `TestBuildRoutesIPv6` | `internal/perf/routes_test.go` | IPv6 prefix generation from seed | |
| `TestRoutesDeterministic` | `internal/perf/routes_test.go` | Same seed+count = same prefixes | |
| `TestSenderInlineNLRI` | `internal/perf/sender_test.go` | IPv4/unicast UPDATE uses inline NLRI field (no MP) | |
| `TestSenderForceMP` | `internal/perf/sender_test.go` | IPv4/unicast with force-mp encodes in MP_REACH_NLRI (AFI=1/SAFI=1) | |
| `TestPrefixExtractionInline` | `internal/perf/receiver_test.go` | Extract IPv4 prefixes from inline NLRI field | |
| `TestPrefixExtractionMP` | `internal/perf/receiver_test.go` | Extract prefixes from MP_REACH_NLRI attribute | |
| `TestLatencyCalculation` | `internal/perf/metrics_test.go` | Percentile calculation (p50, p90, p99) | |
| `TestLatencyCalculationEdgeCases` | `internal/perf/metrics_test.go` | 0 routes returns zeros, 1 route returns that value for all percentiles | |
| `TestThroughputCalculation` | `internal/perf/metrics_test.go` | Routes/sec from timestamps | |
| `TestJSONResult` | `internal/perf/result_test.go` | JSON marshal/unmarshal round-trip, kebab-case keys | |
| `TestMarkdownReport` | `internal/perf/report/report_test.go` | Comparison table from multiple results | |
| `TestHTMLReport` | `internal/perf/report/report_test.go` | Self-contained HTML output | |
| `TestMedianCalculation` | `internal/perf/metrics_test.go` | Median of odd/even count, single value, empty | |
| `TestStddevCalculation` | `internal/perf/metrics_test.go` | Stddev of identical values = 0, known distribution | |
| `TestAggregateIterations` | `internal/perf/metrics_test.go` | N iteration results aggregated to median + stddev per metric | |
| `TestOutlierRemoval` | `internal/perf/metrics_test.go` | Run with 3x convergence time is discarded, remaining stats recomputed | |
| `TestOutlierRemovalKeepsAll` | `internal/perf/metrics_test.go` | When all runs are within 2 stddev, none are discarded | |
| `TestOutlierRemovalMinRuns` | `internal/perf/metrics_test.go` | Never discard below 3 remaining runs (keep at least 3) | |
| `TestRegressionDetection` | `internal/perf/regression_test.go` | Threshold-based regression flagging | |
| `TestRegressionStddevAware` | `internal/perf/regression_test.go` | Delta within combined stddev does not trigger even if above percentage threshold | |
| `TestRegressionNoFalsePositive` | `internal/perf/regression_test.go` | Small variations do not trigger | |
| `TestNDJSONParsing` | `internal/perf/result_test.go` | Read append-only NDJSON history | |
| `TestRunSmallBenchmark` | `internal/perf/perf_test.go` | End-to-end with trivial forwarder: send 10 routes, get metrics | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| routes | 1-1000000 | 1000000 | 0 | N/A (capped) |
| seed | 0-maxuint64 | maxuint64 | N/A | N/A |
| dut-port | 1-65535 | 65535 | 0 | 65536 |
| warmup | 0s+ | 0s | N/A | N/A |
| duration | 1s+ | 1s | 0s | N/A |
| repeat | 1-1000 | 1000 | 0 | N/A (capped) |
| warmup-runs | 0-100 | 100 | N/A | N/A (capped) |
| iter-delay | 0s+ | 0s | N/A | N/A |
| threshold-convergence | 1-1000 (percent) | 1000 | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-perf-loopback` | `internal/perf/perf_test.go` | Sender -> trivial forwarder -> receiver, verify metrics | |
| `test-perf-report-md` | `internal/perf/report/report_test.go` | Generate markdown from sample JSON files | |
| `test-perf-report-html` | `internal/perf/report/report_test.go` | Generate HTML from sample JSON files | |
| `test-perf-track-regression` | `internal/perf/regression_test.go` | Detect regression from NDJSON history | |

### Future (deferred with user approval)
- Docker-based functional tests (`test/perf/*.ci`) running against real DUTs -- requires Docker in CI
- Line charts in HTML trend report (SVG)
- Additional families beyond ipv4/ipv6 unicast (VPN, EVPN, FlowSpec)
- Configurable outlier threshold (currently hardcoded at 2 stddev)

## Files to Modify
- `Makefile` - add ze-perf build/bench/track targets

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | Yes | `cmd/ze-perf/main.go` (new binary) |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add ze-perf benchmarking tool |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add ze-perf subcommands |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/benchmarking.md` - new page |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - add benchmarking capability |
| 12 | Internal architecture changed? | No | |

## Files to Create

### Binary
- `cmd/ze-perf/main.go` - subcommand dispatch (run, report, track), help handling
- `cmd/ze-perf/run.go` - `ze-perf run` flag parsing and orchestration
- `cmd/ze-perf/report.go` - `ze-perf report` flag parsing and file loading
- `cmd/ze-perf/track.go` - `ze-perf track` flag parsing and history loading

### Core
- `internal/perf/sender.go` - sender peer: connect, OPEN exchange, send UPDATEs with timestamps
- `internal/perf/receiver.go` - receiver peer: connect, OPEN exchange, parse UPDATEs (inline NLRI + MP_REACH_NLRI), extract prefixes with timestamps
- `internal/perf/session.go` - shared BGP session setup (OPEN exchange, keepalive goroutine)
- `internal/perf/routes.go` - deterministic route generation from seed (IPv4 and IPv6)
- `internal/perf/metrics.go` - timestamp matching, latency percentiles, throughput calculation
- `internal/perf/result.go` - Result struct, JSON marshal/unmarshal, NDJSON read/append
- `internal/perf/regression.go` - threshold comparison, delta calculation, exit code logic
- `internal/perf/forwarder_test.go` - BGP forwarder test helper (~150-200 lines): listens on a port, accepts exactly 2 connections, completes OPEN/KEEPALIVE handshake on each (needs its own ASN, router-id, capabilities), runs keepalive goroutines, identifies UPDATE messages by type byte, copies UPDATEs from first connection to second (does not copy KEEPALIVE/OPEN between them)

### Reporting
- `internal/perf/report/markdown.go` - comparison tables in CommonMark
- `internal/perf/report/html.go` - self-contained HTML (inline CSS + SVG bar charts)
- `internal/perf/report/trend.go` - time-series markdown/HTML for track subcommand

### Tests
- `internal/perf/session_test.go`
- `internal/perf/sender_test.go`
- `internal/perf/routes_test.go`
- `internal/perf/receiver_test.go`
- `internal/perf/metrics_test.go`
- `internal/perf/result_test.go`
- `internal/perf/regression_test.go`
- `internal/perf/perf_test.go` - integration test (trivial forwarder as DUT)
- `internal/perf/report/report_test.go`

### DUT Configs
- `test/perf/configs/ze.conf` - Ze config: two eBGP peers, bgp-rs plugin for route forwarding
- `test/perf/configs/gobgp.toml` - GoBGP config: two neighbors with route redistribution policy
- `test/perf/configs/frr.conf` - FRR config: two neighbors with route-map permitting all
- `test/perf/configs/bird.conf` - BIRD config: two BGP protocols with export filter accepting all
- `test/perf/run.sh` - Docker orchestration script (builds images, creates 172.31.0.0/24 network, runs ze-perf per DUT)

### History Storage
- `test/perf/history/.gitkeep` - directory for NDJSON history files

## CLI Specification

### `ze-perf run`

Usage: `ze-perf run [flags]`

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--dut-addr` | string | yes | | DUT address to connect to |
| `--dut-port` | int | no | 179 | DUT BGP port |
| `--dut-asn` | int | yes | | DUT autonomous system number |
| `--dut-name` | string | no | `"unknown"` | DUT name for reports (e.g., "ze", "gobgp") |
| `--dut-version` | string | no | `""` | DUT version string (e.g., git hash, release tag) |
| `--sender-addr` | string | no | `"127.0.0.1"` | Local address for sender peer |
| `--sender-asn` | int | no | 65001 | Sender AS number |
| `--receiver-addr` | string | no | `"127.0.0.2"` | Local address for receiver peer |
| `--receiver-asn` | int | no | 65002 | Receiver AS number |
| `--routes` | int | no | 1000 | Number of routes to inject |
| `--family` | string | no | `"ipv4/unicast"` | Address family: ipv4/unicast or ipv6/unicast |
| `--force-mp` | bool | no | false | Encode IPv4/unicast via MP_REACH_NLRI instead of inline NLRI |
| `--seed` | uint64 | no | random | Deterministic route generation seed |
| `--warmup` | duration | no | `2s` | Wait after sessions establish before sending |
| `--connect-timeout` | duration | no | `10s` | Max time to wait for TCP connect + OPEN exchange per peer |
| `--duration` | duration | no | `60s` | Max time to wait for convergence after sending starts |
| `--repeat` | int | no | 5 | Number of measured iterations (median + stddev reported) |
| `--warmup-runs` | int | no | 1 | Throwaway iterations before measurement (warm caches) |
| `--iter-delay` | duration | no | `3s` | Pause between iterations (0 = back-to-back, tests DUT peer-cycling recovery) |
| `--batch-size` | int | no | 0 (pack max) | Routes per UPDATE message |
| `--json` | bool | no | false | JSON output to stdout (human progress always on stderr) |
| `--output` | string | no | `""` | Write JSON result to file (implies `--json`; human summary still printed to stderr) |

Exit codes: 0 = success (all routes propagated), 1 = partial/timeout, 2 = connection failure.

All subcommands handle `-h`/`--help` per cli-patterns.md, printing usage with flag descriptions and examples.

### `ze-perf report`

Usage: `ze-perf report [flags] <file1.json> [file2.json ...]`

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--md` | bool | no | true | Markdown output (default) |
| `--html` | bool | no | false | Self-contained HTML output |

Reads one or more JSON result files. Groups results by family (ipv4/unicast, ipv6/unicast, etc.) and produces a separate comparison table per family group, sorted by convergence time. Warns to stderr if results span multiple families.

### `ze-perf track`

Usage: `ze-perf track [flags] <history.ndjson>`

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--md` | bool | no | true | Markdown output (default) |
| `--html` | bool | no | false | HTML output with trend visualization |
| `--check` | bool | no | false | CI mode: exit non-zero on regression |
| `--last` | int | no | 0 | Only check last N runs (0 = all) |
| `--threshold-convergence` | int | no | 20 | Regression threshold for convergence (percent increase) |
| `--threshold-throughput` | int | no | 20 | Regression threshold for throughput (percent decrease) |
| `--threshold-p99` | int | no | 30 | Regression threshold for p99 latency (percent increase) |

## JSON Result Format

| Field | Type | Description |
|-------|------|-------------|
| `dut-name` | string | DUT identifier (e.g., "ze", "gobgp", "frr", "bird") |
| `dut-version` | string | DUT version string |
| `dut-addr` | string | DUT IP address |
| `dut-port` | int | DUT BGP port |
| `dut-asn` | int | DUT AS number |
| `routes` | int | Number of routes sent |
| `family` | string | Address family used (e.g., "ipv4/unicast") |
| `force-mp` | bool | Whether IPv4 was encoded via MP_REACH_NLRI |
| `seed` | uint64 | Route generation seed (JSON number; values above 2^53 lose precision in JavaScript) |
| `timestamp` | string | ISO 8601 timestamp of run |
| `repeat` | int | Number of measured iterations (before outlier removal) |
| `repeat-kept` | int | Number of iterations kept after outlier removal |
| `warmup-runs` | int | Number of warmup iterations (discarded) |
| `iter-delay-ms` | int | Pause between iterations in milliseconds (0 = back-to-back) |
| `session-setup-ms` | object | `{"sender": N, "receiver": N}` -- median session establishment time |
| `first-route-ms` | int | Median time from first UPDATE sent to first UPDATE received |
| `convergence-ms` | int | Median time from first UPDATE sent to last UPDATE received |
| `convergence-stddev-ms` | int | Standard deviation of convergence across iterations |
| `routes-sent` | int | Routes announced by sender per iteration |
| `routes-received` | int | Median routes received by receiver |
| `routes-lost` | int | Median routes-sent minus routes-received |
| `throughput-avg` | int | Median average routes/second received |
| `throughput-avg-stddev` | int | Standard deviation of throughput across iterations |
| `throughput-peak` | int | Median peak routes/second in any 1-second window |
| `latency-p50-ms` | int | Median of per-iteration 50th percentile per-route latency |
| `latency-p90-ms` | int | Median of per-iteration 90th percentile per-route latency |
| `latency-p99-ms` | int | Median of per-iteration 99th percentile per-route latency |
| `latency-p99-stddev-ms` | int | Standard deviation of p99 across iterations |
| `latency-max-ms` | int | Maximum per-route latency across all iterations |

## Regression Detection

Regression is only flagged when the difference between medians exceeds both the percentage threshold AND the combined standard deviation of both runs. This prevents false positives from normal noise.

| Metric | Direction | Default Threshold | Regression When |
|--------|-----------|-------------------|-----------------|
| convergence-ms | lower is better | 20% | current_median > previous_median * 1.20 AND delta > sqrt(current_stddev^2 + previous_stddev^2) |
| throughput-avg | higher is better | 20% | current_median < previous_median * 0.80 AND delta > sqrt(current_stddev^2 + previous_stddev^2) |
| latency-p99-ms | lower is better | 30% | current_median > previous_median * 1.30 AND delta > sqrt(current_stddev^2 + previous_stddev^2) |
| routes-lost | zero is required | 0 | current_median > 0 |

When stddev is 0 (single-run data or zero variance), only the percentage threshold applies.

## DUT Configuration Pattern

Each DUT must be configured with:
1. Two eBGP peers: sender (ASN 65001) and receiver (ASN 65002)
2. A policy/mechanism to forward routes from sender peer to receiver peer
3. The DUT's own ASN (65000 by default)
4. No route aggregation (prefixes must pass through unchanged for matching)

### Route Forwarding Mechanism per DUT

| DUT | Forwarding mechanism | Key config element |
|-----|---------------------|--------------------|
| Ze | bgp-rs (route server) plugin | `plugin bgp-rs;` in config |
| FRR | Route-map permitting all, applied to both neighbors | `route-map PERMIT permit 10` + `neighbor X route-map PERMIT in/out` |
| BIRD | Export filter accepting all on both protocols | `export all; import all;` on both BGP protocol blocks |
| GoBGP | Global policy accepting and exporting all routes | `[global.apply-policy.config]` with default accept |

### Docker Network Layout

| Host | Address | Purpose |
|------|---------|---------|
| ze-perf sender | 172.31.0.10 | Sender peer local address |
| ze-perf receiver | 172.31.0.11 | Receiver peer local address |
| Ze DUT | 172.31.0.2 | Ze container |
| FRR DUT | 172.31.0.3 | FRR container |
| BIRD DUT | 172.31.0.4 | BIRD container |
| GoBGP DUT | 172.31.0.5 | GoBGP container |

Network: 172.31.0.0/24 (separate from interop tests which use 172.30.0.0/24).

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Result types and metrics** -- data structures, math, aggregation
   - Tests: `TestJSONResult`, `TestLatencyCalculation`, `TestLatencyCalculationEdgeCases`, `TestThroughputCalculation`, `TestNDJSONParsing`, `TestMedianCalculation`, `TestStddevCalculation`, `TestAggregateIterations`, `TestOutlierRemoval`, `TestOutlierRemovalKeepsAll`, `TestOutlierRemovalMinRuns`
   - Files: `internal/perf/result.go`, `internal/perf/metrics.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Route generation** -- deterministic prefix generation for IPv4 and IPv6
   - Tests: `TestBuildRoutes`, `TestBuildRoutesIPv6`, `TestRoutesDeterministic`
   - Files: `internal/perf/routes.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Session and sender** -- BGP session setup and UPDATE sending (all three family modes)
   - Tests: `TestBuildOpen`, `TestBuildOpenIPv6`
   - Files: `internal/perf/session.go`, `internal/perf/sender.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Receiver** -- UPDATE parsing: both inline NLRI and MP_REACH_NLRI prefix extraction with timestamps
   - Tests: `TestPrefixExtractionInline`, `TestPrefixExtractionMP`
   - Files: `internal/perf/receiver.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Integration** -- end-to-end test with trivial forwarder
   - Tests: `TestRunSmallBenchmark`
   - Files: `internal/perf/perf_test.go`, `internal/perf/forwarder_test.go`
   - The forwarder is a test helper (~150-200 lines): listens on a port, accepts two BGP connections, completes OPEN/KEEPALIVE on both, copies UPDATE messages (identified by type byte) from first to second. Receiver connects first so it is the second accepted connection (receives UPDATEs)
   - Verify: send 10 routes through forwarder, verify all metrics populated and routes-received == routes-sent

6. **Phase: Report generation** -- markdown and HTML output
   - Tests: `TestMarkdownReport`, `TestHTMLReport`
   - Files: `internal/perf/report/markdown.go`, `internal/perf/report/html.go`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Regression tracking** -- NDJSON history, stddev-aware threshold detection
   - Tests: `TestRegressionDetection`, `TestRegressionStddevAware`, `TestRegressionNoFalsePositive`
   - Files: `internal/perf/regression.go`, `internal/perf/report/trend.go`
   - Verify: tests fail -> implement -> tests pass

8. **Phase: CLI** -- subcommand dispatch, flag parsing, help handling
   - Files: `cmd/ze-perf/main.go`, `cmd/ze-perf/run.go`, `cmd/ze-perf/report.go`, `cmd/ze-perf/track.go`
   - Verify: `go build ./cmd/ze-perf/` succeeds, `ze-perf -h` prints usage

9. **Phase: DUT configs and runner** -- Docker test infrastructure
   - Files: `test/perf/configs/*.conf`, `test/perf/run.sh`, `Makefile`
   - Each config must include the route forwarding mechanism from the DUT table above
   - Verify: configs are valid for each DUT (manual review, or start DUT and check session)

10. **Phase: Documentation**
    - Files: `docs/features.md`, `docs/guide/command-reference.md`, `docs/guide/benchmarking.md`, `docs/comparison.md`
    - Verify: docs reference correct CLI flags, output format, and family modes

11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Latency math uses monotonic clock (time.Now uses monotonic), not wall clock |
| Correctness | Percentile calculation handles edge cases (0 routes, 1 route) |
| Correctness | Median/stddev calculation handles edge cases (1 iteration = stddev 0, 2 iterations) |
| Correctness | Warmup iterations are fully discarded, not leaked into aggregation |
| Correctness | Outlier removal uses convergence-ms as the discriminator, never discards below 3 remaining runs |
| Correctness | TCP_NODELAY set on both sender and receiver connections |
| Correctness | Receiver parses both inline NLRI and MP_REACH_NLRI correctly |
| Correctness | Sender uses correct encoding path: inline NLRI for ipv4/unicast (no --force-mp), MP_REACH_NLRI for ipv4/unicast with --force-mp and for ipv6/unicast |
| Naming | JSON keys use kebab-case per ze conventions |
| Naming | CLI flags use long-form `--kebab-case` |
| Data flow | Sender timestamps AFTER socket write returns |
| Data flow | Receiver timestamps on prefix extraction from read buffer |
| Rule: no-layering | No chaos package imports in internal/perf/ |
| Rule: cli-patterns | All subcommands handle -h/--help, use flag.NewFlagSet |
| Rule: tdd AC-Linked | For each AC, quote the expected behavior, name the test assertion. If a no-op passes the test, the test is invalid |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze-perf` binary builds | `go build ./cmd/ze-perf/` |
| `ze-perf run` produces JSON | Run against trivial forwarder, verify JSON structure |
| `ze-perf report --md` produces markdown | Feed sample JSON, verify table structure |
| `ze-perf report --html` produces HTML | Feed sample JSON, verify self-contained HTML |
| `ze-perf track --check` detects regression | Feed crafted NDJSON, verify exit code |
| DUT configs exist | `ls test/perf/configs/` |
| Makefile targets work | `make ze-perf-build` |
| All tests pass | `make ze-verify` |
| Help output works | `ze-perf -h`, `ze-perf run -h`, `ze-perf report -h`, `ze-perf track -h` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | DUT address/port validated before TCP connect |
| Input validation | JSON file paths validated (no path traversal in report/track) |
| Input validation | Family flag validated against allowed set (ipv4/unicast, ipv6/unicast) |
| Input validation | `--force-mp` only valid with `--family ipv4/unicast` |
| Resource limits | Connection timeout prevents hanging on unresponsive DUT |
| Resource limits | Route count capped to prevent OOM on large benchmarks |
| No secrets | DUT configs do not embed credentials (BGP has no auth in ze-perf v1) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Loopback test hangs | Check goroutine leak, add timeout to test |
| Trivial forwarder doesn't forward | Check that it copies UPDATE type messages, not just any BGP message |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf.md`
- [ ] Summary included in commit
