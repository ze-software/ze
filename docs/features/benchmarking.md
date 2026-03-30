# Performance Benchmarking

Ze includes `ze-perf`, a standalone BGP propagation latency benchmark tool. It
measures route forwarding performance through a device under test (DUT) by
establishing sender and receiver BGP sessions, injecting routes, and timing
their propagation.

<!-- source: cmd/ze-perf/main.go -- ze-perf CLI entry point -->

| Feature | Description |
|---------|-------------|
| Cross-implementation comparison | Docker runner for Ze, FRR, BIRD, GoBGP, rustbgpd (or any BGP speaker) |
| Multi-iteration with statistics | Median/stddev from repeated runs, outlier removal |
| Three encoding modes | IPv4 inline NLRI, IPv4 force-MP (MP_REACH_NLRI), IPv6 MP |
| Comparison reports | Markdown and HTML side-by-side reports from result files |
| History tracking | NDJSON history files with stddev-aware regression detection |
| CI integration | `--check` flag exits non-zero on performance regression |

<!-- source: internal/perf/metrics.go -- aggregation and outlier removal -->
<!-- source: internal/perf/regression.go -- regression detection -->

See [Benchmarking Guide](guide/benchmarking.md) for usage instructions.
