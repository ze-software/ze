# Performance Comparison

> **All benchmarks are lies.**
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
| Date | 2026-04-22 |
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
- **rustbgpd** -- Rust BGP daemon, kernel TCP stack.
  Config: passive peers, route_server_client enabled.
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
- **OpenBGPd** -- C BGP daemon (OpenBSD portable), kernel TCP stack.
  Config: passive peers, allow from/to any (no filtering).
  Transport: kernel TCP (standard Docker networking).

Config files: `test/perf/configs/`

## Results

### ipv4/unicast

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p50 | p99 | +/- | Max | Lost |
|-----|-------------|-----|------------------|-----|-----|-----|-----|-----|------|
| bird | 50ms | 0ms | 2,000,000 | 32,675 | 18ms | 26ms | 0ms | 26ms | 0 |
| ze | 91ms | 27ms | 1,098,901 | 461,693 | 20ms | 81ms | 27ms | 81ms | 0 |
| rustbgpd | 179ms | 5ms | 558,659 | 15,247 | 90ms | 151ms | 12ms | 152ms | 0 |
| rustybgp | 252ms | 14ms | 396,825 | 20,283 | 120ms | 233ms | 13ms | 235ms | 0 |
| openbgpd | 472ms | 0ms | 211,864 | 0 | 217ms | 461ms | 0ms | 466ms | 0 |
| frr | 537ms | 10ms | 186,219 | 3,764 | 412ms | 532ms | 10ms | 532ms | 0 |
| gobgp | 1,147ms | 13ms | 87,183 | 1,031 | 585ms | 1,118ms | 14ms | 1,125ms | 0 |
| freertr | 2,294ms | 146ms | 43,591 | 7,872 | 727ms | 1,992ms | 619ms | 2,294ms | 0 |

## Reading the Results

**Convergence** is the time from the first UPDATE sent to the last UPDATE
received. Lower is better. This is the primary metric -- it answers "how long
until all routes are propagated?"

**Throughput** is routes received per second, averaged over the convergence
window. Higher is better. Zero means all routes arrived in a single burst
(sub-second convergence with coalesced TCP delivery).

**p50/p99** are per-route latency percentiles. p50 is the median route's
latency; p99 is the slowest 1%. The gap between p50 and p99 shows how
consistent the DUT's forwarding is.

**+/-** columns show standard deviation across iterations. Small stddev means
consistent performance; large stddev means the measurement is noisy.

**Lost** should always be zero. Any lost routes indicate the DUT failed to
forward some prefixes.

## Reproducing

This document is auto-generated by `ze-perf report --doc`. To regenerate
with fresh results:

```bash
# Build ze-perf and all DUT Docker images, then run benchmarks
python3 test/perf/run.py --build --test

# Or test specific DUTs (available: ze, bird, frr, gobgp, rustbgpd, rustybgp, freertr, openbgpd)
python3 test/perf/run.py --build --test ze bird

# Regenerate this document from existing results
bin/ze-perf report --doc test/perf/results/*.json > docs/performance.md
```

Requires Docker (Colima on macOS). See [Benchmarking Guide](guide/benchmarking.md)
for details on environment variables (`DUT_ROUTES`, `DUT_REPEAT`, `PPROF`, etc.).
