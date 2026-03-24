# 417 -- ze-perf

## Context

Operators comparing BGP implementations (Ze, GoBGP, FRR, BIRD) lacked a standardized tool for measuring route propagation latency. Each implementation's benchmarking approach was ad-hoc and non-comparable. The goal was a standalone binary that establishes sender and receiver BGP sessions through a device under test, measures propagation timing with statistical rigor (multiple iterations, outlier removal, percentile latencies), and produces structured output for cross-implementation comparison and regression tracking.

## Decisions

- TCP-only benchmarking over in-process testing because fair cross-implementation comparison requires real network.
- Duplication of BGP session code over importing ze-chaos, because the purposes differ (timing precision vs correctness fuzzing) and the overlap is small (~50 lines for session setup).
- Prefix-only matching for route correlation over full attribute matching, because DUTs may modify AS_PATH and NEXT_HOP during forwarding.
- Aggregated results (median + stddev of N iterations) over individual run data, because single runs are noise.
- Stddev-aware regression detection over percentage-only thresholds, because normal variation within noise should not trigger CI failures.
- Dual-strategy connection (dial + listen) over dial-only, because some DUTs (e.g., rustbgpd) only respond to peers they can actively connect to.
- Three encoding modes (inline NLRI, force-MP, IPv6) over IPv4-only, to isolate MP_REACH_NLRI attribute overhead.
- Python runner script over shell, for Docker orchestration of multi-DUT benchmarks.

## Consequences

- `ze-perf run` measures propagation latency for any BGP implementation reachable over TCP.
- `ze-perf report` generates cross-implementation comparison tables (Markdown and self-contained HTML with SVG bar charts).
- `ze-perf track --check` enables CI regression detection with configurable thresholds.
- NDJSON history format allows append-only tracking committed to the repo.
- DUT configs exist for Ze, FRR, BIRD, GoBGP, and rustbgpd.
- The forwarder test helper pattern (accept two connections, forward UPDATEs) is reusable for future BGP integration tests.

## Gotchas

- Single-iteration benchmarks with loopback forwarders can receive zero routes if the warmup period is too short (100ms insufficient, 500ms reliable). The forwarder needs time to start its forwarding goroutines after both handshakes complete.
- CLI tests that capture os.Stderr cannot run in parallel due to the global variable race. Use sequential execution for stderr-capturing tests.
- `connectBGP` dual-strategy means tests on machines where port 179 is available (root or capability) may take a different code path than expected. The ConnectTimeout prevents hangs in either case.
- `flag.ContinueOnError` makes `-h` on subcommands return exit code 1 (ErrHelp treated as error). Top-level help is handled by the dispatch switch and returns 0.

## Files

- `cmd/ze-perf/main.go`, `run.go`, `report.go`, `track.go` -- CLI binary
- `cmd/ze-perf/main_test.go` -- CLI exit code and help tests
- `internal/perf/benchmark.go` -- RunBenchmark orchestrator
- `internal/perf/sender.go`, `receiver.go`, `session.go` -- BGP peer simulation
- `internal/perf/routes.go` -- deterministic route generation
- `internal/perf/metrics.go` -- latency percentiles, throughput, aggregation, outlier removal
- `internal/perf/result.go` -- Result struct, JSON/NDJSON serialization
- `internal/perf/regression.go` -- stddev-aware regression detection
- `internal/perf/report/` -- Markdown, HTML, trend report generators
- `internal/perf/forwarder_test.go` -- test forwarder and sink forwarder helpers
- `internal/perf/perf_test.go`, `benchmark_e2e_test.go` -- integration tests
- `test/perf/configs/` -- DUT configs (ze, frr, bird, gobgp, rustybgp)
- `test/perf/run.py` -- Docker orchestration runner
- `docs/guide/benchmarking.md` -- user guide
