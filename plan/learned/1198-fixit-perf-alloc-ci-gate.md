# Learned: fixit-perf-alloc-ci-gate

Spec: `plan/spec-fixit-perf-alloc-ci-gate.md` (allocs/op ceiling gate in `ze-verify`
+ scheduled Docker-free perf `--check`).

## What the change does
Adds an always-run, Docker-free CI gate (`make ze-alloc-gate`, registered as a
`ze-verify` stage) that runs the reactor hot-path `ReportAllocs` benchmarks with
`-benchmem` and asserts a per-benchmark integer `allocs/op` ceiling. Promotes the
machine-dependent timing regression check (`bin/ze-perf track --check`) into a
scheduled-only `.woodpecker/perf-nightly.yml` (cron event). The deterministic alloc
gate blocks merges; the noisy timing check never does.

## Non-obvious findings / traps
- **`go test` runs the enforce check with cwd = the package source dir, NOT the
  repo root.** The gate captures benchmark output to a file, then runs
  `TestAllocGateEnforce` (in `internal/perf`) which reads it via `ZE_ALLOC_GATE_BENCH`.
  A repo-relative path resolves under `internal/perf/` and fails. `mk/alloc-gate.mk`
  passes `$(CURDIR)/tmp/verify/alloc-gate-bench.txt` (absolute). This bit once (the
  first end-to-end run failed with "no such file or directory").
- **Registration is the Go map, not the make regex.** The target runs `-bench='.'`
  over the whole reactor package; `perf.CheckAllocCeilings` enforces ONLY the names in
  `perf.AllocCeilings` and ignores the rest. So AC-4 ("add one list entry") is a
  one-line map edit in `internal/perf/allocgate.go` — no Makefile change to gate a new
  benchmark.
- **Fail-closed on a missing benchmark.** A masked build/run failure emits no
  `Benchmark...` lines. `CheckAllocCeilings` treats every registered-but-absent
  benchmark as a violation, so a broken build cannot pass the gate silently
  (`| tee` masks the bench pipeline's exit code, so this guard is load-bearing, not
  belt-and-suspenders). `TestAllocGateMissingFailsClosed` locks it.
- **allocs/op is the stable column; B/op is not.** At low benchtime the `B/op`
  amortization is noisy (e.g. BufMux showed 1051 -> 175 B/op across runs) but
  `allocs/op` stayed integer-stable. The gate keys on allocs/op only — the whole
  reason it is machine-independent and belongs in the always-run gate (ns/op and
  throughput do not).
- **Measured baselines (2x count, `-benchtime=3000x`):** ForwardDirect=5 (ceiling 6,
  +1 headroom), BufMuxGetReturn / FwdPoolTryDispatch / EBGPWireCacheHitParallel=0
  (ceiling 0, strict — these are zero-alloc doctrine hot paths).
- **Only the full `ze-verify` gets the stage, not `ze-verify-changed`.** CI runs full
  `ze-verify` (`.woodpecker/verify.yml`), so AC-3 is met by the `default` branch of
  `stagesForMode` alone; keeping it out of the `-changed` list honors "do not slow the
  inline dev loop." `TestStagesIncludeAllocGate` asserts BOTH directions.
- **`make ze-perf` is broken; the nightly uses `make perf`.** `mk/perf.mk:16` builds
  `./cmd/ze-perf`, which does not exist (ze-perf is `cmd/ze` + tag `ze_perf`,
  `Makefile:164`). Pre-existing bug, out of this spec's scope; the nightly builds via
  `make perf`.
- **No `fmt.Sprintf` in the checker.** Violation messages use `textbuf.Buffer`
  (no-sprintf-alloc hook), even though this is not a hot path — the hook is mechanical.

## Verification boundary (parked background job)
AC-1 proven end-to-end (`make ze-alloc-gate` exits 0, all 4 benchmarks within
ceilings). AC-2 proven end-to-end (a crafted over-ceiling log trips
`TestAllocGateEnforce`, exit 1, naming `BenchmarkBufMuxGetReturn`). AC-3 by
`TestStagesIncludeAllocGate`. AC-4/AC-5 structural (map registration; cron-only
Docker-free yaml, valid YAML). Full `make ze-verify` NOT run (would kill live
servers). See `tmp/drain-fixit-perf-alloc-ci-gate.md` for the shared-file overlaps
(Makefile/docs) and the learned-number `--fix`.

## Files

None recorded.
