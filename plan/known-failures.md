# Known Test Failures

Pre-existing failures that need fixing. Each entry includes failure output and root-cause hypothesis.
Sessions should attempt to fix entries here before logging new ones.

Resolved flake-investigation knowledge distilled into
`plan/learned/608-concurrent-test-patterns.md` (2026-04-17 tidy). Read that
summary before investigating a new concurrency or test-isolation failure --
the recurring shapes are catalogued there.

## prefix-maximum-enforce parallel-load flake -- LOGGED 2026-04-17

**File:** `test/plugin/prefix-maximum-enforce.ci`
**Symptom:** Fails intermittently under `make ze-verify` (GOMAXPROCS=29,
parallel functional suite). Standalone retries pass cleanly:
`bin/ze-test bgp plugin prefix-maximum-enforce` -> pass 1.0s. The recorded
parallel-load shape was a message-order mismatch around maximum-prefix
enforcement.
**Hypothesis:** Port/time contention or message-order timing under
high-parallel runner load. The current port-reservation mitigation and local
stress repeats are promising, but keep this row open until a full release gate
shows the flake shape is gone.
**Unrelated to spec-l2tp-7** -- no L2TP code touched by this test; same
behaviour before the spec landed.

2026-05-03 update: the paired `bfd-auth-meticulous-persist.ci` flake is no
longer tracked here. Its Python driver now waits for the persisted `.seq` file
to appear and advance instead of relying on a fixed 2.5s sleep before shutdown.
Focused stress verification passed locally:

```bash
bin/ze-test bgp plugin -c 3 bfd-auth-meticulous-persist
bin/ze-test bgp plugin -c 3 prefix-maximum-enforce
```

2026-05-03 follow-up: `bin/ze-test bgp plugin -c 10 prefix-maximum-enforce`
passed 10/10 locally. Keep this row open until a full release gate proves the
parallel-load flake shape is gone.

## TestPeerInfoPopulatesStats (uptime == 0) -- RESOLVED 2026-04-29

**File:** `internal/component/bgp/reactor/reactor_api_test.go:47`
**Symptom:** `assert.True(t, p.Uptime > 0, "uptime should be non-zero for established peer")` fails because `SetEstablishedNow()` stamps the current time and `adapter.Peers()` runs immediately after, often computing `time.Since(established)` == 0 on fast CPUs (nanosecond clock resolution can return the same value twice in back-to-back calls). Reproduced with `go test -run TestPeerInfoPopulatesStats -count=3` (3/3 fail).
**Hypothesis:** The test was introduced in commit `0801fe949 feat: replace generic message counters with per-type BGP statistics`; uses `SetEstablishedNow()` + immediate `Peers()` call with no delay in between. Pre-existing; not caused by any fmt-0-append or peer_initial_sync migration.
**Resolution:** `TestPeerInfoPopulatesStats` now sets `establishedAt` explicitly in the past instead of calling `SetEstablishedNow()` immediately before reading `Peers()`.

Verification: `go test -count=3 ./internal/component/bgp/reactor -run TestPeerInfoPopulatesStats` passed locally on 2026-04-29.

## TestFwdPool_StopUnblocksDispatch (residual flake) -- RESOLVED 2026-05-03

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
**Resolution:** The test no longer waits for post-`Dispatch` goroutine code to
write a result channel after `Stop()` returns. It now asserts the production
contract directly: `Stop()` must return after unblocking the blocked dispatch.

Verification:

```bash
go test -race ./internal/component/bgp/reactor -run TestFwdPool_StopUnblocksDispatch -count=500
```

Passed locally on 2026-05-03.

## watchdog plugin test (flake under parallel load) -- RESOLVED 2026-05-03

**File:** `test/plugin/watchdog.ci`
**Symptom:** Output `mismatch` under `make ze-verify` (parallel mode); passes in isolation (`bin/ze-test bgp plugin 272`, 3.8s).
**Reproduction:**
- FAILS: `make ze-verify` occasionally on first run; retries pass.
- PASSES: `bin/ze-test bgp plugin 272` in isolation, every run.
**Hypothesis:** Peer-tool timing against scripted BGP sessions under
parallel CPU load. The watchdog test asserts a specific session outcome
that may arrive out-of-order when the tester's event loop is preempted.
Related: "Full-suite flaky plugin+encode tests" entry below.
**Resolution:** `test/plugin/watchdog.ci` no longer loops against parent
process lifetime with fixed sleeps. The script now waits for the plugin
post-startup callback, drains startup route delivery with `peer-flush`, and
sends the exact three announce/withdraw pairs asserted by the peer. The sibling
`test/plugin/watchdog-med-override.ci` was hardened with the same pattern.

Verification:

```bash
bin/ze-test bgp plugin -c 3 watchdog
bin/ze-test bgp plugin -c 3 watchdog-med-override
```

Both passed locally on 2026-05-03.

## nexthop plugin test (flake under parallel load) -- RESOLVED 2026-05-03

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
**Resolution:** `test/plugin/nexthop.ci` keeps the intentional first-update
before initial EOR ordering, but replaces fixed `time.sleep(0.2)` gaps with
`ze-bgp:peer-flush` via `wait_for_ack()` after each injected route. This keeps
the wire assertions unchanged while waiting for delivery before the next route.

Verification:

```bash
bin/ze-test bgp plugin -c 3 nexthop
```

Passed locally on 2026-05-03.

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

2026-05-03 mitigation: `ze-test` BGP and VPP runners now call
`runner.ReservePorts`, which holds advisory per-port locks for the suite
lifetime. This prevents concurrent `ze-test` processes from selecting the
same free range between probe and later bind. It does not protect against
arbitrary external processes binding the selected ports, so keep this row
open until a full release gate shows the flake shape is gone.

Verification:

```bash
go test ./internal/test/runner -run 'TestFindFreePortRange|TestReservePorts|TestAllocatePorts|TestCheckPortAvailable|TestPortRangeString' -count=1
go test ./cmd/ze-test ./internal/test/runner -count=1
go test -race ./internal/test/runner -count=1
```

All passed locally on 2026-05-03.

## Egress-filter tests need forwarding-plugin redesign -- RESOLVED 2026-04-29

The eight tests previously tracked here now use destination-peer wire
assertions (`expect=bgp`) for the release evidence instead of observer-side
smoke checks. The Python observers remain only to keep Ze alive and provide
diagnostics while the destination peer verifies the post-filter wire behavior.

Covered tests:

- `community-strip.ci` verifies egress community stripping on the forwarded UPDATE.
- `forward-overflow-two-tier.ci` verifies all 50 burst routes arrive in order.
- `forward-two-tier-under-load.ci` verifies all 80 burst routes arrive in order.
- `role-otc-egress-filter.ci` verifies provider-to-provider suppression by EOR-only output.
- `role-otc-egress-stamp.ci` verifies OTC is stamped on the forwarded UPDATE.
- `role-otc-export-unknown.ci` verifies no-role peers still forward unchanged.
- `role-otc-ingress-reject.ci` verifies leaked customer routes are not forwarded.
- `role-otc-unicast-scope.ci` verifies multicast bypasses OTC processing and forwards unchanged.

Verification:

```bash
go run ./cmd/ze-test bgp plugin 91 128 129 250 251 252 253 254
```

Result: `pass 8/8 100.0%` locally on 2026-04-29.

## test/plugin/show-errors-received (rare flake) -- RESOLVED 2026-05-03

**File:** `test/plugin/show-errors-received.ci` (test index 250 in `bin/ze-test bgp plugin`)
**Symptom:** Observer reports `ZE-OBSERVER-FAIL: unexpected error: no response for ze-plugin-engine:dispatch-command`. First `make ze-verify` run failed here; immediate retry passed with 37/37.
**Reproduction:** Not reliable -- occurred once during phase-2 l2tp-reliable implementation session, not on retry. Passes cleanly in isolation (verified 2026-04-17).
**Hypothesis:** The observer dispatches a `dispatch-command` event to `ze-plugin-engine` and awaits a response within a timeout. Under load (full `ze-verify` runs many other suites concurrently), the dispatch response may not arrive in time. The observer protocol likely needs a longer per-call timeout, or the test needs to gate on a readiness signal before dispatching.
**Resolution:** `test/plugin/show-errors-received.ci` now waits for the plugin post-startup callback before polling `show errors`. The dispatch path is therefore exercised only after the command registry is frozen.

Verification:

```bash
bin/ze-test bgp plugin show-errors-received -v
bin/ze-test bgp plugin -c 3 show-errors-received
```

Both passed locally on 2026-05-03.

## addpath + fib-vpp-* failures under make ze-verify -- RESOLVED 2026-05-03

**Files:**
- `test/encode/addpath.ci` (index `0` in `bin/ze-test bgp encode`)
- `test/plugin/fib-vpp-coexist-with-fib-kernel.ci` (index `103`)
- `test/plugin/fib-vpp-plugin-load.ci` (index `104`)
- Also `exabgp-test`: `conf-addpath` retry failure

**Symptom (observed during spec-l2tp-6b-auth Phase 9 closeout):**
- `TEST FAILURE: 0 addpath` -- `mismatch`, raw peer output shows only
  "listening on 127.0.0.1:1790 / new connection from ..." (no wire data
  captured in the 3000-char snippet).
- `TEST FAILURE: 103 / 104 fib-vpp-*` -- `timeout` with
  `expected messages: 0 / received messages: 0` (neither side received
  anything within the window).
- `FAIL  3 suite(s) failed: encode plugin parse` followed by
  `FAIL: exabgp-test` retry of `conf-addpath`.

**Why unrelated to Phase 9:** Phase 9 touches only
`internal/component/{ppp,l2tp,config}`. The addpath encode path is
`internal/plugins/bgp-*` and `internal/component/bgp/message`; the
fib-vpp tests are `internal/component/fib/*` + VPP subsystems. None of
those files were edited in the Phase 9 work, and the isolated suites
`go test ./internal/component/ppp/... ./internal/component/l2tp/...`
all pass green.

**Hypothesis:**
- addpath / conf-addpath: recent BGP peer YANG refactor (`be93b950`,
  `edc8bd55`) reshaped `connect-mode -> connect/accept`, and the exabgp
  migrate path + encode fixture may still reference the old shape.
- fib-vpp-*: VPP plugin tests likely require a VPP binary or kernel
  modules not present in this dev environment, or there's a plugin-
  load race with the recent `6b7f5db9` auto-load wiring.

**Next steps to try:**
- Run `bin/ze-test bgp encode addpath -v` in isolation and diff the
  captured vs expected hex.
- Check `test/plugin/fib-vpp-*.ci` timeout directives and verify VPP
  is runnable in the test harness.
- If fib-vpp tests require real VPP, gate them behind a build tag or
  a skip-when-missing check.

**Resolution:** Focused reruns of the previously failing encode/plugin tests
now pass, and the full ExaBGP compatibility suite passes 37/37 including the
previously mentioned `conf-addpath` case. No code change was needed in this
pass; the row is kept as resolved historical context.

Verification:

```bash
bin/ze-test bgp encode addpath -v
bin/ze-test bgp plugin fib-vpp-coexist-with-fib-kernel -v
bin/ze-test bgp plugin fib-vpp-plugin-load -v
make ze-exabgp-test
```

All passed locally on 2026-05-03.

## PPP LCP handleLCPPacket re-enters afterLCPOpen on Echo frames -- RESOLVED 2026-05-03

**File:** `internal/component/ppp/session_run.go` around line 696-726,
combined with `internal/component/ppp/lcp_fsm.go:392-393`.

**Symptom:** When an LCP Echo-Request / Echo-Reply arrives in the
Opened state, `LCPDoTransition(Opened, RXR)` returns
`{NewState: Opened, Actions: [SER]}`. `handleLCPPacket`'s
`if tr.NewState == LCPStateOpened { ... afterLCPOpen() }` branch has
NO guard for `cur != LCPStateOpened`, so it re-enters afterLCPOpen on
every Echo-Request / Echo-Reply in Opened. That re-emits EventLCPUp
and re-runs `runAuthPhase` inline.

**Discovery:** Surfaced while writing a Phase 9 regression test for
/ze-review ISSUE 1 (LCP-during-reauth). The test panicked on nil
`s.ops.setMRU` because afterLCPOpen was being called recursively
from `handleLCPPacket` via `waitCHAPLike -> handleFrame -> handleLCPPacket`
when the test injected an LCP Echo-Request. Stack trace:

```
panic: runtime error: invalid memory address or nil pointer dereference
 session_run.go:423 in afterLCPOpen
 session_run.go:718 in handleLCPPacket  <-- Opened-branch re-entry
 session_run.go:589 in handleFrame
 auth.go:300 in waitCHAPLike
 chap.go:262 in runCHAPAuthPhase
```

**Why production seems to work:** afterLCPOpen's side effects
(setMRU, SetMTU, SetAdminUp) are idempotent; runAuthPhase emits
EventAuthRequest but the always-accept responder in the l2tp-auth
plugin would just accept again. So operationally a "burst" of
re-auth events fires every Echo but the session limps on. Likely
produces stray EventAuthRequest / EventSessionUp events visible to
plugins and in logs.

**Resolution:** `handleLCPPacket` now runs the post-Opened side effects
only when the FSM actually transitions into `LCPStateOpened`. The RXR
action path now sends Echo-Reply only for Echo-Request; Echo-Reply and
Discard-Request are consumed without a reply.

The FSM still maps `(Opened, RXR)` to `SER` to match the existing RFC
1661 transition table model. The action layer now gates `SER` by packet
code, so the shared RXR event no longer replies to Echo-Reply or
Discard-Request.

Verification:

```bash
go test ./internal/component/ppp -run 'TestHandleLCPPacketOpenedRXRDoesNotReenterOpened|TestLCPEcho|TestLCPFSMRXRInOpened' -count=1
go test -race ./internal/component/ppp -count=1
```

Both passed locally on 2026-05-03.

## vpp-config-invalid-poll-interval / -invalid-hugepage parse tests -- RESOLVED 2026-04-29

**Files:**
- `test/parse/vpp-config-invalid-poll-interval.ci` (parse test 314)
- `test/parse/vpp-config-invalid-hugepage.ci` (parse test 315)

**Symptom under `make ze-verify`:**
```
✗ vpp-config-invalid-poll-interval: expected failure but validation succeeded
✗ vpp-config-invalid-hugepage: expected failure but validation succeeded
```
Both tests declare `expect=exit:code=1` (negative tests). `ze config
validate` returns 0 for the bad input, so the parse runner flags them.

**Why unrelated to spec-l2tp-6b-auth Phase 9 .ci work:** the .ci files
added in this session only touch `test/ui/cli-env-reauth-interval.ci`,
`test/l2tp/reauth-interval-clamp.ci`, and
`internal/component/l2tp/subsystem.go`. The failing tests load VPP
config; they fail because sibling uncommitted work in
`internal/component/vpp/{config,vpp,schema}.go` has loosened the
validator without updating the negative-test fixtures. `git status`
at session start shows these files modified but not committed.

**Hypothesis:** VPP validator changes made `poll-interval` /
`hugepage` accept the previously-invalid values; the invalid-case
fixtures now look valid to the validator.

**Next steps to try (for the VPP-work author):**
- Diff `internal/component/vpp/config.go` and
  `internal/component/vpp/schema/ze-vpp-conf.yang` against HEAD and
  decide whether the new validator is intentionally more permissive.
- Either tighten the validator back up, or update the two .ci
  fixtures with a value that is still invalid under the new rules.

**Correction (spec-vpp-7 session, 2026-04-17 10:56Z):** the hypothesis
above is wrong. I A/B-tested by temporarily reverting
`internal/component/vpp/{config.go,vpp.go,schema/ze-vpp-conf.yang}`
to `origin/main` (HEAD `f5895422`), rebuilt `bin/ze`, and re-ran
`ze config validate` against the same `hugepage-size=4M` / `poll-interval=0`
inputs both tests use -- `ze config validate` returns exit 0 and
`configuration valid` in BOTH cases on pristine origin. My VPP changes
(add `external bool` leaf + `runOnce` external branch) did not touch
the hugepage enum nor the poll-interval range; diff of the three files
against origin confirms. The root cause is that `ze config validate`
does not enforce YANG enum or range constraints at all -- it only
enforces `unknownKeys` (so `nonsense-leaf` IS rejected, which is why
`vpp-config-unknown-key.ci` passes). The two invalid-input tests have
been failing since commit `6b7f5db9` (the one that added them).

**Resolution:** `internal/component/config/yang_schema.go` now carries YANG
leaf `enum` and numeric `range` restrictions into `LeafNode`, and the
hierarchical parser enforces them through `ValidateLeafValue`.

Verification:

```bash
go test -count=1 ./internal/component/config
go run ./cmd/ze-test bgp parse 178 179 183 184
```

Both passed locally on 2026-04-29.

## TestRaiseHookNetdevDisambiguation (nft readback) -- RESOLVED 2026-05-03

**File:** `internal/plugins/firewall/nft/readback_linux_test.go:77`
**Symptom:** `inet 0 = prerouting (ok=true), want HookInput` and `inet 1 = input (ok=true), want HookOutput`. The test expects nftables hook IDs 0/1 to map to Input/Output for the inet family, but the kernel returns Prerouting/Input instead. This is a kernel version mismatch: on newer kernels (6.8+), the inet hook numbering follows the netdev convention where 0=Prerouting, not the legacy inet convention where 0=Input.
**Hypothesis:** The test's expected mapping was written against an older kernel. The readback code or the test expectations need to account for the kernel's actual hook ID assignment.
**Resolution:** The Linux readback test now expects `raiseHook(inet, 0)` to map to `HookPrerouting` and `raiseHook(inet, 1)` to map to `HookInput`, while netdev family still disambiguates the same raw values as ingress and egress.

Verification:

```bash
make ze-linux-test ZE_LINUX_TEST_PACKAGES="./internal/plugins/firewall/nft"
```

Passed locally on 2026-05-03.

Remove entries once fixed.

## 2026-04-18 -- BGP config: `remote: accept` direction-placement broke functional `.ci` suite -- RESOLVED 2026-04-29

After commit `7991bc294` ("config(bgp): direction-based placement of connect/accept") moved the `accept` leaf between container scopes, many existing `.ci` test configs still write `remote { accept ... }` and the parser rejects with "unknown field in remote: accept (line 6)". This makes the daemon refuse to load, so every `.ci` test that relies on the daemon starting reports `FAIL: SSH server did not start (no address in daemon.log)`.

**Affected suites (5 of 8 in `make ze-verify`):** encode, plugin, parse, reload, ui. Plus exabgp-test downstream.

**Representative failure line (`tmp/ze-verify-func.log`):**

    error: load config: parse config: line 6: unknown field in remote: accept (line 6)

**Resolution:** No stale `remote { accept ... }` placement remains under `test/`.

Verification: `rg -n -U 'remote\s*\{[^}]*accept\s+(true|false)' test` returned no matches locally on 2026-04-29.
