# 608 -- Concurrent test flake patterns (2026-04 distilled)

## Context

Between 2026-04-07 and 2026-04-16, `plan/known-failures.md` accumulated a
cluster of investigations into race-detector hits, timeouts and flaky
assertions. Most were closed by the same handful of patterns recurring across
subsystems (BGP reactor, forward pool, SSE broker, CLI editor, BFD transport,
chaos runner, plugin SDK). When those entries were tidied, the concrete
production/test snippets were dropped -- this file preserves the transferable
knowledge so the next flake investigation recognises the shape fast instead of
re-deriving it. This is a cross-cutting distillation, not a single-spec
summary.

## Patterns

- **Locked write + unlocked read is a race even when the detector is
  silent.** `s.bufReader` / `s.bufWriter` / `s.conn` were assigned under
  `s.mu.Lock()` inside `connectionEstablished` but read by the Run loop
  without any lock. The Go memory model calls that a race regardless of
  scheduling; the detector may stay quiet for months. Fix: capture all three
  under `s.mu.RLock()` at the top of the loop and pass them as parameters.
  When two locks exist (`s.mu` + `s.writeMu`), nest in the established
  ordering (`s.mu -> s.writeMu`). Reference implementation:
  `internal/component/bgp/reactor/session.go:Run`,
  `internal/component/bgp/reactor/session_connection.go:connectionEstablished`.

- **Subscribe-before-broadcast in pub/sub tests.** `http.Client.Do` returns
  as soon as the server flushes response headers, which happens BEFORE the
  handler reaches `broker.Subscribe(...)`. A test that does
  `Do -> Broadcast -> Body.Read` can broadcast into zero subscribers and
  block forever. Fix: poll `broker.ClientCount()` (or equivalent
  subscriber-count accessor) until it is >= 1 before broadcasting.
  Reference: `waitForClient` helper used by SSE tests in
  `internal/chaos/web/sse_test.go`.

- **Gate handlers to force deterministic queue state.** Tests that assert
  "backpressure triggered" or "overflow observed" cannot assume the worker
  will not drain items in flight. Park the handler on the first item with a
  `gate` channel, dispatch all N items (channel fills, overflow fills),
  assert the intermediate state, then close the gate. Paired-gate pattern:
  one gate parks the filling phase, a second gate parks the draining phase.
  Reference: `TestBackpressureNoResumeAbove10Percent` in
  `internal/component/bgp/plugins/rs/worker_test.go` and the matching
  supersede test in `forward_pool_supersede_test.go`.

- **Barrier sentinels must respect FIFO against queued items.** A
  "flush-all-prior-work" sentinel cannot use a fast path that bypasses the
  queue. Before `TryDispatch(sentinel)`, check whether the overflow queue is
  non-empty; if so, dispatch via the slow path that preserves order. The
  drain path must never process leftover items in place -- requeue them to
  the front of overflow so the next cycle picks them up after the channel
  drains. Reference: `Barrier` and `drainOverflow` in
  `internal/component/bgp/reactor/forward_pool_barrier.go` /
  `forward_pool.go`.

- **Cleanup goroutines must complete work, not just exit.** `Loop.Stop()`
  tore down the BFD transport and its express-loop goroutine but never
  iterated `l.sessions` to call `CloseAuth`, so any auth persister pinned by
  a third party never flushed its sequence number. Coordinators that own
  auxiliary goroutines must drain all their work on stop, not only tear
  down their own loop. Reference: `Loop.Stop` in
  `internal/plugins/bfd/engine/engine.go` and regression test
  `TestLoopStopFlushesPinnedPersister`.

- **Duration budget must accommodate CPU starvation under parallel test
  binaries.** `TestInProcessBasicRoute` passed in ~3s isolated and needed
  ~46s under `make ze-verify` load. Treat the scenario `Duration`
  independently from the outer `ctx` timeout: give it ~8x the isolated
  wall-clock, keep the ctx as the absolute ceiling. Don't confuse the two.

- **Test-fake pool handles with zero-value IDs corrupt real pool slots.**
  Tests that constructed `BufHandle{Buf: make([]byte, 4096)}` to call
  production code got a handle whose zero-value `ID`/`idx` collided with
  the first real slot in the BufMux pool. Eviction paths then double-freed
  or silently marked a live slot as available. Fix: add a sentinel ID
  (`noPoolBufID = ^uint32(0)`) that `ReturnReadBuffer` skips, and give
  tests a helper (`testPoolBuf(t)`) that tags its handles with it. Any pool
  exposing handle structs to tests needs the same sentinel discipline.
  Reference: `internal/component/bgp/reactor/bufmux.go`,
  `session.go:ReturnReadBuffer`, `reactor_test.go:testPoolBuf`.

- **Plugin stderr classifier must force ERROR for `panic:` and `fatal
  error:`.** An SDK constructor that returned a `*Plugin` with a nil
  callbacks map caused external plugins to panic on the first event. The
  panic was swallowed because stderr-relay defaulted to WARN and Go panic
  lines parse at Info level. Fix: classify stderr lines starting with
  `panic:` / `fatal error:` (and the goroutine stack that follows) as
  ERROR so they always relay. Covered by
  `TestClassifyStderrLine*` in
  `internal/component/plugin/process/stderr_relay_test.go`.

- **Fixed-port protocols in tests need SO_REUSEPORT behind an env-var
  gate.** BFD binds RFC 5881/5883 ports 3784/4784 as a hard protocol
  requirement; the test runner defaults to 20-wide parallelism, so a second
  test's bind instantly fails. Solution: the transport layer enables
  `SO_REUSEPORT` only when `ze.bfd.test-parallel=true`; every BFD `.ci`
  sets that env var via
  `option=env:var=ze.bfd.test-parallel:value=true`. Production leaves the
  flag unset and keeps its fail-fast single-binder behaviour. Same pattern
  applies to any future fixed-port protocol (L2TP, TACACS+, etc.).

- **Shallow map merge drops YANG augmentations.** `findModuleEntry` and
  `mergedRoot` used `maps.Copy` on module Dir maps, overwriting colliding
  keys. When multiple `-conf` modules augment the same container (e.g.
  `environment`), only the first match survived -- completion showed one
  module's children, none of the others. Fix: collect every module with a
  matching top-level entry and recursively merge their `Dir` maps via a
  dedicated `mergeAugmentedEntries` helper. Any code that walks the YANG
  tree across modules must do the same.

## Gotchas

- A `go test -race -count=N` that does not reproduce a race does not prove
  the race does not exist. `TestInProcessSpeed` had a genuine field race
  that failed to trigger across 230+ counted runs. Fix the code when the
  memory-model analysis says "race", even without a detector hit.
- A test that passes after the production logic is broken is invalid
  (`rules/testing.md` "Observer-Exit Antipattern"). The egress-filter
  conversion surfaced multiple tests where `sys.exit(1)` from an observer
  was hiding real rejection logic.
- Gate-channel tests are only deterministic if EVERY sibling test with the
  same pool uses the same gate discipline. Leaving a no-op handler in one
  test caused `TestFwdPool_SupersedingDifferentKeys` to race with its own
  worker goroutine.
- `drainOverflow`'s "just run leftovers directly" shortcut is a FIFO
  violation dressed as an optimisation. If you see `processDirect`-style
  code in a queue drain, grep for every caller that passes a sentinel item
  through the same queue -- one of them is almost certainly wrong.
- `tmp/*.go` is real Go source to `go test ./...`. Research subagents that
  drop vendored third-party source in `tmp/<topic>/` for inspection MUST
  rename to `.txt` or use a non-`.go` extension. Covered in repo memory.

## Files

- `internal/component/bgp/reactor/session.go`,
  `session_connection.go`,
  `session_read.go` -- lock-capture discipline for `conn`/`bufReader`/`bufWriter`
- `internal/component/bgp/reactor/forward_pool.go`,
  `forward_pool_barrier.go` -- barrier FIFO discipline, overflow requeue
- `internal/component/bgp/reactor/bufmux.go`,
  `reactor_test.go` -- `noPoolBufID` sentinel + `testPoolBuf` helper
- `internal/component/bgp/plugins/rs/worker_test.go` -- paired-gate
  backpressure test pattern
- `internal/chaos/web/sse_test.go` -- `waitForClient` subscribe-wait helper
- `internal/plugins/bfd/engine/engine.go`,
  `engine_test.go:TestLoopStopFlushesPinnedPersister` -- cleanup-drains-work
- `internal/plugins/bfd/transport/udp_linux.go` -- SO_REUSEPORT env-var gate
- `internal/component/plugin/process/process.go:classifyStderrLine`,
  `stderr_relay_test.go` -- panic/fatal ERROR classification
- `internal/component/cli/completer.go:findModuleEntry`,
  `mergedRoot`,
  `mergeAugmentedEntries` -- YANG augmentation-aware merge
- `internal/component/cli/testing/headless.go`,
  `internal/component/config/tree.go`,
  `meta.go` -- per-node RWMutex on Tree/MetaTree for concurrent dispatch
