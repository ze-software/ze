# 625 -- rs-fastpath-1-profile

## Context

Ze's route-server forwarding at 100k IPv4 prefixes runs at ~33k rps -- 16× behind bird (~780k rps) on the same Docker harness (`test/perf/run.py`). The umbrella `spec-rs-fastpath-0` is a 3-child effort (profile → adj-rib-in off hot path → zero-copy pass-through) to close that gap. Child 1 was tasked with turning the gap into a named, profile-backed bottleneck so children 2 and 3 could be scoped against concrete evidence rather than hypothesis, and with setting the umbrella's AC-1 numeric target based on what the structural fixes should realistically achieve.

## Decisions

- **Wired profile capture into `test/perf/run.py` (PPROF / GCTRACE env gates) rather than adding a new in-ze endpoint.** `cmd/ze/pprof.go` already exposes `--pprof <addr>` bound to container-local loopback. The harness now passes that flag, spawns a Python thread that `docker exec python3`s into the ze container to fetch CPU / heap / allocs / goroutine profiles, and archives `gctrace` via `docker logs`. Kept the localhost-only binding security constraint intact.
- **Dropped the originally-planned "T-ms flush timer" from scope.** `server_forward.go` + `worker.go` already flush partial batches via the `onDrained` callback when the per-source worker channel empties; under sustained load batches hit `maxBatchSize`; no state wedges a single UPDATE behind a timer. Added `TestBatchForwardSingleFlushOnDrain` to prevent the onDrained path from silently regressing.
- **Named `forwardCh` depth as `rsForwardChDepth = 16` with a senders-x-batch sizing comment** over tuning the value. The profile showed the forwardCh is *not* contended -- the engine-side RPC is the slow step, so enlarging the channel would only grow the in-flight buffer without accelerating drains.
- **Bumped `maxBatchSize` 50 → 500** because Phase 2 re-bench showed +9.1 % throughput, -9.4 % first-route, -8.5 % p99, no regression. Chose 500 over a wider sweep because the profile shape says engine-side per-UPDATE cost dominates the fixed-per-RPC cost, so further batch growth has sharply diminishing returns.
- **Declined to sweep `ze.rs.fwd.senders` and `forwardCh` depth** despite the spec skeleton listing them. Profile showed engine's `Dispatcher.Dispatch` serialises on a registry RLock, so more senders would add contention without raising throughput.
- **Set umbrella AC-1 target 400k rps / ≤ 50 ms first-route (aspirational, 200k rps floor)** over deferring the number indefinitely. The number is grounded in the profile: children 2 and 3 structurally remove the top two cost centres (adj-rib-in hot-path subscription, rs↔engine text-RPC round-trip), which together account for >60 % of allocation pressure; removing them should raise throughput by 10×+.
- **Wrote the RFC 7947 short summary** (`rfc/short/rfc7947.md`) to close a required-reading gap: the umbrella cited it but no summary existed.

## Consequences

- The benchmark harness is now self-instrumenting -- future sessions can run `PPROF=1 GCTRACE=1 DUT_ROUTES=100000 python3 test/perf/run.py --test ze` to reproduce a profile set without writing new plumbing.
- Child 2 and child 3 open with a concrete scoring table (Design Insights in the spec): top-10 CPU, top-10 allocations, before/after knob numbers. They do not need to re-profile to know which functions their changes must move.
- `maxBatchSize` is now 500. Memory per batch grew proportionally (worst case 500 × ~10 bytes per ID = 5 KB per batch), still bounded by `rsForwardChDepth × maxBatchSize ≈ 8000 in-flight IDs` per RS.
- `cmd/ze/pprof.go` localhost-only binding is confirmed load-bearing: the harness uses `docker exec` so the validation does not need to loosen.
- Three deferrals logged for future work: the batch-flush `.ci` was dropped as redundant with the Go unit test; an engine-side `CommandRegistry.All()` per-RPC allocation (~5 % of allocation pressure) is left for child 3 to subsume via the RPC bypass; the lower-size scaling sweep is deferred to child-3 verification.

## Gotchas

- The first pprof run surprised me: the #1 allocator is `plugin/server.tokenize` (19.4 % of 2.5 GB), not anything inside `plugins/rs`. The cost is structural -- every batch the rs plugin sends becomes a text command that the engine re-tokenizes. Allocating hotspots in `plugins/rs` itself (like `extractWireFamilies`'s 2-entry map) do not even register in the top-20. This reframed the umbrella: the rs plugin is nearly optimal; the cost lives in the *boundary* between rs and the engine.
- Profile captures ~137 % of wall-clock CPU (1.37 cores). The 43-47 % GC share is therefore absolute -- not "half of one core" but "half of 1.4 cores" -- which is an order of magnitude more GC pressure than a healthy Go daemon should show.
- Build-image timing: `test/perf/run.py --build --test ze` rebuilds the ze-interop image (Go build inside Docker) before each run, ~2 minutes on this machine. For iterative sweep work, use `--test ze` only and rebuild explicitly when code changes land.
- The Alpine ze-interop image has `python3` but not `wget`/`curl`. The profile-capture thread uses `python3 -c 'urllib.request.urlopen(...)'` for portability; `docker cp` of in-container files would be the fallback if python were removed.
- `gctrace.log` is ~98 MB for a 30-second capture at 100k routes. Do not grep the full file casually; use head/tail for pause-frequency spot-checks and `awk '/^gc /'` for structured parsing.

## Files

- `internal/component/bgp/plugins/rs/server.go` -- added `rsForwardChDepth` named constant with sizing comment; `maxBatchSize` 50 → 500 with evidence-linked comment.
- `internal/component/bgp/plugins/rs/server_test.go` -- added `TestForwardChDepthNamed` (AC-5) and `TestBatchForwardSingleFlushOnDrain` (AC-4).
- `test/perf/run.py` -- added `PPROF`, `PPROF_PORT`, `PPROF_CPU_SECONDS`, `PPROF_DIR`, `GCTRACE` env gates; `pprof_fetch` + `pprof_capture_thread` helpers; container stderr archival.
- `test/plugin/bgp-rs-perf-pprof.ci` -- functional test: `ze --pprof` endpoint reachable, index lists expected profiles.
- `rfc/short/rfc7947.md` -- new RFC summary for RS semantics.
- `plan/spec-rs-fastpath-0-umbrella.md` -- AC-1 numeric target set (400k rps / ≤ 50 ms first-route, 200k rps floor).
- `plan/deferrals.md` -- three new rows (batch-flush `.ci` cancelled; CommandRegistry.All allocation, lower-size sweep open for child 3 verification).
- Profile artefacts under `tmp/perf-run/pprof/100000/` and `tmp/perf-run/pprof-batch500/100000/` (not committed).
