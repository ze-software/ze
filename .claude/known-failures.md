# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Resolved flake-investigation knowledge distilled into
`plan/learned/608-concurrent-test-patterns.md` (2026-04-17 tidy). Read that
summary before investigating a new concurrency or test-isolation failure --
the recurring shapes are catalogued there.

## TestFwdPool_StopUnblocksDispatch (residual flake) -- LOGGED 2026-04-11

**File:** `internal/component/bgp/reactor/forward_pool_stop_test.go`
**Symptom:** Flaky at ~0.6% (3 failures in 500 iterations) under
`go test -race -count=500`. `require.Eventually` on the `result` channel
times out after 5 seconds despite `pool.Stop()` having returned.
**Background:** Surfaced while fixing `TestFwdPool_Barrier_WithOverflow`
(see `plan/learned/608-concurrent-test-patterns.md` for the barrier FIFO
fix). Loosening an overly-strict `assert.False(t, stopOk)` cut the flake
rate from ~20% to this residual scheduling issue.
**Hypothesis:** `Stop` only returns after `dispatchWG.Wait()` confirms the
blocked goroutine has exited `Dispatch`; the subsequent `result <- ok`
write to a buffered channel should be sub-microsecond. The deferred
`dispatchWG.Done()` may be observed by `Wait()` BEFORE the goroutine is
scheduled onto its post-defer code (the `result <- ok` line).
**Next steps to try:**
- Add `t.Cleanup` + goroutine stack dump when `Eventually` fails, to see
  where the goroutine parks.
- Replace `result` with a `sync.WaitGroup` waiting on the GOROUTINE to
  complete. A prior attempt failed differently ("wg.Done but result
  empty") -- suggests the goroutine IS exiting without running
  `result <- ok`, which is only possible if Dispatch silently panics.
- Add `defer recover()` in the test goroutine to catch silent panics.

## plugin test 272 watchdog (flake under parallel load) -- LOGGED 2026-04-16

**File:** `test/plugin/watchdog.ci`
**Symptom:** Output `mismatch` under `make ze-verify-fast` (parallel mode); passes in isolation (`bin/ze-test bgp plugin 272`, 3.8s).
**Reproduction:**
- FAILS: `make ze-verify-fast` occasionally on first run; retries pass.
- PASSES: `bin/ze-test bgp plugin 272` in isolation, every run.
**Hypothesis:** Peer-tool timing against scripted BGP sessions under
parallel CPU load. The watchdog test asserts a specific session outcome
that may arrive out-of-order when the tester's event loop is preempted.
Related: "Full-suite flaky plugin+encode tests" entry below.
**Fix pattern candidate:** the EoR gating pattern documented for plugin
tests Y/147/150-152/244-245 (now resolved) -- keeps the peer session open
past observer startup. Apply if the same symptom shape is confirmed.

## plugin test 153 nexthop (flake under parallel load) -- LOGGED 2026-04-16

**File:** `test/plugin/nexthop.ci`
**Symptom:** Under `make ze-verify-changed` the test fails with message
mismatch: received MP_REACH_NLRI ends in `0001` while expected ends in
`0002`. Suggests the peer replayed only the first UPDATE and cut the
session before the second was delivered.
**Reproduction:**
- FAILS: `make ze-verify-changed` (parallel load across categories).
- PASSES: `bin/ze-test bgp plugin 153` in isolation (1.9s).
**Hypothesis:** Same family as plugin test 272 above -- peer-tool closes
TCP before the session-establishment handshake completes, observer sees
truncated flow.
**Fix needed:** Add `expect=bgp:...EoR` gating to the peer stdin, matching
the EoR pattern used by `api-peer-show.ci`.

## Full-suite flaky plugin+encode tests (test isolation) -- LOGGED 2026-04-11

**Files / test IDs observed:**
- `test/encode/ebgp.ci` (Test 9, encode category)
- `test/plugin/attributes.ci` (Test Q, plugin category) -- timeout, 0/5 messages received
- `test/plugin/mup4.ci` (Test 121) and `test/plugin/mup6.ci` (Test 122) -- message mismatch
- `test/plugin/rpki-decorator-autoload.ci` (Test 196) -- timeout waiting for second message
- `test/plugin/check.ci` (Test V) -- message mismatch, received UPDATE with default-route payload when the test expected an empty body

**Symptom:** Each test passes in isolation (`bin/ze-test bgp plugin <id> -v` or `bin/ze-test bgp encode ebgp -v` -> exit 0). Each also passes when the plugin suite is re-run right after a failure (`bin/ze-test bgp plugin --all` -> 230/230). They fail only when the whole sequence `make ze-verify` runs all categories back-to-back, and the failing set changes run to run.

**Reproduction conditions:**
- FAILS: `make ze-verify` clean run (encode + plugin + reload + exabgp + editor + etc., all sequential). Flake hits 1-5 tests each time.
- PASSES: `bin/ze-test bgp plugin --all` alone (47s, 230/230).
- PASSES: `bin/ze-test bgp encode --all` alone.
- PASSES: individual test reruns.

**Root-cause hypothesis:** resource contention across test binaries when
`make ze-functional-test` runs its child categories sequentially without
sufficient isolation. Candidates:
- Ephemeral port reuse race between `encode` and `plugin` suites.
- Loopback alias `127.0.0.2` setup failing (seen in WARN logs:
  `ensureLoopbackAlias: ioctl SIOCAIFADDR 127.0.0.2 on lo0: operation not permitted`).
  On macOS dev machines this is expected -- tests fall back to ephemeral
  ports, but that may expose ordering assumptions.
- Prior test's BGP peer still holding the listening socket when the next test
  binds. The ebgp/check failures (UPDATE arrives with unexpected default-route
  body) look like a prior peer session bleeding into the new session.

**Proposed next step:** investigate `ze-test` runner for cross-category port
reservation or a barrier between categories.

**Interim workaround:** re-run `make ze-verify` or the specific category.
Single-category runs (`bin/ze-test bgp plugin --all`) are stable and safe as
a gate when the full suite flakes.

## Egress-filter tests need forwarding-plugin redesign -- LOGGED 2026-04-11

The 16-file observer-exit conversion (spec-ci-observer-per-test-audit)
finished all 16 runtime_fail swaps but 8 of them are in "framework wired,
AC verification TODO" state because of a single architectural issue
discovered during phase 1: ze does not auto-forward UPDATEs between
configured peers. Forwarding is plugin-driven (bgp-rs, bgp-cache, etc.).

The following 8 tests load only `bgp-adj-rib-in` and no forwarding
plugin, so their egress-side AC verification cannot succeed in the
current test shape:

- `community-strip.ci` (AC-7 egress strip)
- `forward-overflow-two-tier.ci` (AC-10/11/12 overflow pool)
- `forward-two-tier-under-load.ci` (AC-10/11/12 two-tier dispatch)
- `role-otc-egress-filter.ci` (OTC ingress stamp)
- `role-otc-egress-stamp.ci` (OTC egress stamp on forward)
- `role-otc-export-unknown.ci` (no-role passthrough)
- `role-otc-ingress-reject.ci` (OTC ingress reject)
- `role-otc-unicast-scope.ci` (OTC unicast scoping)

Each carries an inline STATUS comment pointing at this entry.

**Compounding issue:** even if these tests loaded a forwarding plugin,
the `bgp-rpki` plugin auto-loads via `ConfigRoots: ["bgp"]` and enables
the adj-rib-in validation gate via `OnAllPluginsReady`. Routes then
wait in `r.pending` for either RPKI validation or a 30s fail-open
timeout. All 8 tests use `tcp_connections=1` peers that disconnect
before the 30s timeout, so `clearPeerPending` wipes the route before
the python observer can see it in `adj-rib-in status`. Any future
redesign must address the forwarding-plugin gap AND the validation
gate interaction.

**Redesign paths:**

- **Path A (minimal ze change):** Add `--plugin ze.bgp-rs` (or
  `bgp-cache`) to each .ci so the reactor actually broadcasts received
  UPDATEs. Then add one info-level log in `bgp/plugins/filter_community`
  or `bgp/plugins/role` on the successful-apply path, and assert on it
  via `expect=stderr:pattern=`. This is what `bgp-filter-prefix`
  already does (cmd-4 `prefix-list accept`) and what phase 2 did for
  `community ingress applied`.

- **Path B (wire-level):** Switch dest peers from `--mode sink` to
  default check mode with an `expect=bgp:hex=` directive matching the
  post-rewrite wire bytes. Requires computing the exact post-rewrite
  hex (AS-PATH prepend, NEXT_HOP rewrite, attribute insertion/removal)
  for each test. More invasive but does not touch production code.

Until the redesign, the 8 tests above are protected by `runtime_fail`
sentinel + weakened `total < 0` assertions + negative regression
checks (`panic recovered`, `fatal error`, `treat-as-withdraw`,
`ZE-OBSERVER-FAIL`). The framework catches crashes and bad wire bytes;
it does not catch logic errors in the egress-filter code paths.

## test/plugin/show-errors-received (rare flake) -- LOGGED 2026-04-14

**File:** `test/plugin/show-errors-received.ci` (test index 250 in `bin/ze-test bgp plugin`)
**Symptom:** Observer reports `ZE-OBSERVER-FAIL: unexpected error: no response for ze-plugin-engine:dispatch-command`. First `make ze-verify` run failed here; immediate retry passed with 37/37.
**Reproduction:** Not reliable -- occurred once during phase-2 l2tp-reliable implementation session, not on retry. Passes cleanly in isolation (verified 2026-04-17).
**Hypothesis:** The observer dispatches a `dispatch-command` event to `ze-plugin-engine` and awaits a response within a timeout. Under load (full `ze-verify` runs many other suites concurrently), the dispatch response may not arrive in time. The observer protocol likely needs a longer per-call timeout, or the test needs to gate on a readiness signal before dispatching.
**Parked.** Estimated 15-30 min investigation: grep `dispatch-command` handling in `ze-plugin-engine` and the observer's timeout config.

Remove entries once fixed.
