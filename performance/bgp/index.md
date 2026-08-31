# Performance Comparison

> **Benchmark numbers need context.**
>
> These numbers measure one specific scenario (route propagation latency through
> a single DUT with two peers) on one specific machine under artificial conditions.
> They do not predict real-world performance. Different hardware, different route
> counts, different address families, different policies, different network
> conditions will all produce different results.
>
> Use these numbers to understand *relative* differences between implementations,
> not as absolute performance claims. If performance matters to you, run ze-perf
> on your own hardware with your own workload.

## Methodology

Ze-perf establishes two BGP sessions with a device under test (DUT): a sender
and a receiver. The sender injects routes and records when each was sent. The
receiver parses incoming UPDATEs and records when each prefix arrived.
Propagation latency = time received minus time sent, matched by prefix.

Each benchmark runs multiple iterations. Results show the **median** across
iterations with standard deviation. Outlier iterations (beyond 2 stddev from
median convergence time) are automatically discarded.

## Environment

| Field | Value |
|-------|-------|
| Platform | darwin/arm64 |
| Virtualization | Docker (Colima VM) |
| Date | 2026-06-05 |
| Routes | 100,000 |
| Seed | 42 |
| Iterations | 3 measured, 1 warmup |

**These results were collected on a development laptop using Docker containers via Colima. A dedicated server with bare-metal networking would produce different (likely faster and more consistent) numbers.**

## DUT Setup

All DUTs run in Docker containers on the same host. Each DUT is
configured with two passive BGP peers (sender AS 65001, receiver AS 65002)
and AS 65000 as the local AS. The benchmark tool (ze-perf) establishes both
sessions, injects routes via the sender, and measures when they arrive at
the receiver.

- **Ze** -- Go BGP daemon, goroutine-based, kernel TCP stack.
  Config: passive peers, route-reflector plugin (bgp-rs), 1M prefix limit per family.
  Transport: kernel TCP (standard Docker networking).
- **BIRD** -- C BGP daemon, kernel TCP stack.
  Config: passive peers, import/export all (no filtering).
  Transport: kernel TCP (standard Docker networking).
- **FRR** (Free Range Routing) -- C BGP daemon, kernel TCP stack.
  Config: passive peers, PERMIT route-maps in/out (no filtering).
  Transport: kernel TCP (standard Docker networking).
- **GoBGP** -- Go BGP daemon, kernel TCP stack.
  Config: passive peers, default accept policy.
  Transport: kernel TCP (standard Docker networking).
- **RustyBGP** -- Rust BGP daemon, kernel TCP stack.
  Config: passive peers, default policy.
  Transport: kernel TCP (standard Docker networking).
- **freeRtr** -- Java BGP daemon with its own TCP/IP stack.
  Config: passive peers, 256KB buffer-size, extended-update enabled,
  advertisement-interval-tx 0, incremental bestpath (1M limit), no safe-ebgp.
  JVM: 2GB heap with ZGC (low-pause garbage collector).
  Transport: rawInt bridge (UDP encapsulation between Docker eth0 and freeRtr's
  virtual interface layer) -- adds latency vs kernel TCP used by other DUTs.

Config files: `test/perf/configs/`

## Results

### ipv4/unicast (2026-06-05, 4 GB VM)

Fixed RPKI validation gate (was adding 30s pending delay without cache servers)
and throughput stddev (now derived from convergence via error propagation).

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p99 | +/- | Withdrawal | +/- |
|-----|-------------|-----|------------------|-----|-----|-----|------------|-----|
| ze | 62ms | 10ms | 1,612,903 | 260,145 | 43ms | 13ms | 596ms | 15ms |
| bird | 65ms | 0ms | 1,538,461 | 0 | 28ms | 3ms | 518ms | 0ms |
| rustybgp | 327ms | 17ms | 305,810 | 15,898 | 266ms | 2ms | 683ms | 18ms |
| frr | 595ms | 11ms | 168,067 | 3,107 | 568ms | 11ms | 637ms | 12ms |
| gobgp | 1,198ms | 20ms | 83,472 | 1,393 | 1,145ms | 22ms | 1,319ms | 7ms |
| freertr | 2,218ms | 56ms | 45,085 | 1,138 | 2,209ms | 57ms | 1,145ms | 4,609ms |

### ipv4/unicast (2026-06-05, 4 GB VM, pre-fixes)

Throughput stddev inflated by reciprocal transform (raw per-iteration stddev).
Ze penalized by RPKI validation gate enabling without cache servers (30s fail-open).

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p99 | +/- | Withdrawal | +/- |
|-----|-------------|-----|------------------|-----|-----|-----|------------|-----|
| bird | 29ms | 0ms | 3,448,275 | 97,221 | 35ms | 0ms | 513ms | 0ms |
| ze | 53ms | 18ms | 1,886,792 | 939,982 | 57ms | 16ms | 571ms | 22ms |
| frr | 548ms | 7ms | 182,481 | 2,414 | 549ms | 6ms | 633ms | 5ms |

### ipv4/unicast (2026-05-24, post-optimization)

After pool dedup, buffer-first encoding, and forwarding fast-path work.

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p99 | +/- |
|-----|-------------|-----|------------------|-----|-----|-----|
| bird | 44ms | 1ms | 2,272,727 | 62,858 | 28ms | 5ms |
| ze | 71ms | 2ms | 1,408,450 | 44,964 | 54ms | 4ms |
| rustbgpd | 179ms | 5ms | 558,659 | 15,247 | 151ms | 12ms |
| rustybgp | 252ms | 14ms | 396,825 | 20,283 | 233ms | 13ms |
| openbgpd | 472ms | 0ms | 211,864 | 0 | 461ms | 0ms |
| frr | 537ms | 10ms | 186,219 | 3,764 | 532ms | 10ms |
| gobgp | 1,147ms | 13ms | 87,183 | 1,031 | 1,118ms | 14ms |
| freertr | 2,294ms | 146ms | 43,591 | 7,872 | 1,992ms | 619ms |

### ipv4/unicast (2026-04-22, initial)

First benchmark run, before any optimization work.

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p99 | +/- |
|-----|-------------|-----|------------------|-----|-----|-----|
| bird | 50ms | 0ms | 2,000,000 | 32,675 | 26ms | 0ms |
| ze | 91ms | 27ms | 1,098,901 | 461,693 | 81ms | 27ms |
| rustbgpd | 179ms | 5ms | 558,659 | 15,247 | 151ms | 12ms |
| rustybgp | 252ms | 14ms | 396,825 | 20,283 | 233ms | 13ms |
| frr | 537ms | 10ms | 186,219 | 3,764 | 532ms | 10ms |
| gobgp | 1,147ms | 13ms | 87,183 | 1,031 | 1,118ms | 14ms |
| freertr | 2,294ms | 146ms | 43,591 | 7,872 | 1,992ms | 619ms |

## Reading the Results

**Convergence** is the time from the first UPDATE sent to the last UPDATE
received. Lower is better. This is the primary metric: the time until all routes
are propagated.

**Throughput** is routes received per second, averaged over the convergence
window. Higher is better. Zero means all routes arrived in a single burst
(sub-second convergence with coalesced TCP delivery).

**p50/p99** are per-route latency percentiles. p50 is the median route's
latency; p99 is the slowest 1%. The gap between p50 and p99 shows how
consistent the DUT's forwarding is.

**+/-** columns show standard deviation across iterations. Small stddev means
consistent performance; large stddev means the measurement is noisy.

**Withdrawal** is the time from sending route withdrawals to the receiver
going idle. Lower is better. Measures how fast the DUT propagates route
removals.

**Lost** should always be zero. Any lost routes indicate the DUT failed to
forward some prefixes.

## Reproducing

This document is generated by `ze-perf report --doc`. To regenerate it with
fresh results:

```bash
# Build the Go benchmark binary and all DUT images, then run every benchmark.
go build -tags ze_perf -o bin/ze-perf ./cmd/ze
go run ./cmd/ze-perf-run --build --test

# Run selected DUTs.
go run ./cmd/ze-perf-run --build --test ze bird

# Render the document from the saved results.
bin/ze-perf report --doc test/perf/results/*.json > docs/performance.md
```

Requires Docker (Colima on macOS). See [Benchmarking Guide](../../guides/benchmarking/index.md)
for details on environment variables (`DUT_ROUTES`, `DUT_REPEAT`, `PPROF`, etc.).
