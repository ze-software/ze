# spec-fixit-redistribute-establishment-stall

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-migrate-sleeps-infra (P0 carve-out); spec-redistribute-late-join-replay |
| Phase | 0/1 (investigation) |
| Updated | 2026-07-15 |

## Task

Root-cause and fix a reproducible, config-specific BGP establishment stall: when an
external observer plugin issues ANY engine activity (dispatch / quiesce / show-poll,
or even a bare `wait_for_event` callback read) during BGP session establishment, a
**single-peer redistribute session never establishes** (`connections-established`
stays 0). Only the original blind `time.sleep` (observer fully idle, not even reading
its callback connection) lets it establish. This blocks deterministic-wait conversion
of the redistribute test bucket under spec-fixit-migrate-sleeps-infra:
`bgp-redistribute-{announce,burst,explicit-nhop,filtered-out,metrics,nexthop-self,withdraw}.ci`
(7 tests, ~16 sleeps), plus `api-raw.ci` / `api-route-refresh.ci` if they share the trigger.

## Investigation Findings (2026-07-15)

A long investigation this session. Net: **part fixed and committed; a deeper part
diagnosed but not fixed.** Two of my intermediate hypotheses were disproven -- both
recorded here so the next session does not repeat them.

### Fixed + committed
- **`fix(bgp): reconnect backoff floor 5s, not 120s connect-retry` (commit 44ad25d23).**
  `internal/component/bgp/reactor/peer.go` NewPeer(:294) set `reconnectMin :=
  settings.ConnectRetry` (default 120s, RFC 4271 ConnectRetryTimer, `peersettings.go:66`),
  while `reconnectMax = DefaultReconnectMax` (60s). So the backoff floor (120s) exceeded
  its ceiling (60s), contradicting the design in `peer_run.go:24` ("min 5s, max 60s") and
  `DefaultReconnectMin` (5s, peer.go:81). A failed first connect stranded the peer
  `connecting` for 2 minutes. Fix: `reconnectMin := DefaultReconnectMin`. ConnectRetry keeps
  its real role as a connect timeout (`reactor_dynamic.go:343`). Reconnect unit tests use
  `SetReconnectDelay` overrides, so unaffected. Verified: converted `announce` 25/25 (was
  ~80% flaky pre-fix); reactor package unit tests green.

### Regression caused by that fix (RESOLVED 2026-07-16 -- fixed in `runner.go`, see end of section)
44ad25d23 widened `internal/chaos/inprocess` `TestInProcessChaosReconnect`, already logged
flaky-under-`-race` in `plan/known-failures.md` since 2026-07-08, into a failure that also
reproduces WITHOUT `-race` when the test runs in isolation.

Measured with the Makefile's build tags (bare `go test` is not equivalent, see
`plan/known-failures.md` "BEFORE LOGGING ANYTHING HERE"), by patching `peer.go` in place and
restoring it:

| Build | Result |
|-------|--------|
| pre-44ad25d23 backoff (`reconnectMin := settings.ConnectRetry`) | PASS 2/2, ~4.3s |
| HEAD (`reconnectMin := DefaultReconnectMin`) | FAIL, 92.00s, `established==1` (`runner_test.go:688`) |

Order-dependent: a full-package `-race` run of `./internal/chaos/inprocess/` PASSED once, so
`make ze-chaos-unit-test` (`mk/test-chaos.mk:27`, `go test -race ./internal/chaos/...`) may
still be green. The isolation failure is the reliable reproducer.

This is chaos-harness work, NOT a BGP defect: the harness itself documents the intended 5s
backoff (`runner_test.go:246`, `runner.go:518-522` "DefaultReconnectMin = 5s virtual"). The
test was green only because the 120s bug parked the retry loop outside the window.

**RESOLVED 2026-07-16. The open question ("where does the real time go inside `vc.Advance`")
rested on a false premise: none of it is spent there, and the advance loop is not slow.**

A goroutine dump taken 30s into the freeze answered it in one run:

| Goroutine | Where |
|---|---|
| runner | `simWg.Wait()` (`runner.go:594`) -- the advance loop had ALREADY finished |
| ze session | `VirtualClock.Sleep` (`virtualclock.go:49`) from `session.go:767` |

The advance loop costs exactly what `runner.go:427-430` implies (~0.6s real for 60s virtual)
and then EXITS -- after which nothing advances the clock. `session.Run()` polls for its
connection with `s.clock.Sleep(10ms)` (`session.go:762-768`) and `VirtualClock.Sleep` is a
bare `<-ch` (`virtualclock.go:47-50`). `clock.Clock.Sleep` takes no ctx, so `simCancel()`
cannot reach a goroutine parked there -- only `Advance` can. ze's session was stranded
mid-sleep, never completed the handshake, the simulator blocked forever on the reply that
never came (`executeReconnectStorm` -> `readMsg`, `simulator_actions.go:233`), and
`simWg.Wait()` hung until the 90s context tore the sockets down. That is the 92.00s, and
`established==1` because the peer was asleep -- not because reconnect was broken.

44ad25d23 is exonerated as a cause and stays: it only changed WHEN ze lands in that sleep.
Because the advance loop finishes in ~0.6s real, a chaos action firing late in the virtual
window is still mid-handshake when time stops; the 120s floor parked the retry outside the
window and hid a defect that predates it.

Fix (`runner.go`, Run's teardown): keep advancing the virtual clock until both the simulators
and the reactor are down. Real time does not stop while a system shuts down, and neither may
virtual time. Verified 3/3 PASS in 3.70s, matching the 3.69s measured at `8f5f2ff4b`
(pre-regression) vs 92.00s broken; `./internal/chaos/...` green; target test 2/2 green under
`-race`; `make ze-lint-changed` 0 issues. `plan/known-failures.md` entry closed.

Do NOT "fix" this by reverting 44ad25d23: the 120s floor exceeded the 60s ceiling and
contradicts `peer_run.go:19-25`, which documents this loop as deliberately replacing the RFC
4271 ConnectRetryTimer with "min 5s, max 60s".

### CONFIRMED (evidence)
- NOT a mutex deadlock: a 122-goroutine dump has zero `[semacquire]`. Refutes H1-H3 as
  *deadlock* hypotheses.
- `announce` (auto-loads the orchestrator) converts to a deterministic
  `wait_until(state=='established')` + `quiesce()` recipe and passes 25/25.
- The 6 *explicit* `internal redistribute-orchestrator` tests (explicit-nhop, filtered-out,
  withdraw, burst, metrics, nexthop-self) DO NOT pass when converted: the peer stays
  `connecting`; the reactor.peer log shows every dial `connection refused` with a doubling
  backoff (5->10->20->40s), so it never establishes inside the test window.
- The committed **blind-sleep** versions of those 6 PASS with a proper build; my observer-only
  conversion (any dispatch during establishment) is what breaks them. Confirmed via HEAD vs
  converted A/B with the proper `zetest` build.
- Full untruncated startup timeline (manual harness): `StartPeers` fires FAST (~0.26s after
  startup begins), and with a LONG-LIVED peer the session establishes on the 2nd dial (5s
  backoff). So the failure is a TIMING interaction with the ze-test peer's lifetime, not a
  slow StartPeers.

### DISPROVEN (do not repeat)
- "Committed plugin-startup regression (closed pipe)": FALSE. Artifact of running with
  `ZE_TEST_NO_BUILD=1 ZE_BIN=bin/ze` where `bin/ze` was a `make ze` **core** build lacking the
  `zetest` fake plugins (fakeredist/fakefib). With the default `zetest` build (buildZe,
  `cmd_bgp.go:458` uses `runner.TestBuildTags()`), `redistribute-as112-announce` passes.
  Always let ze-test build (do not pin ZE_BIN to a core binary) for the `bgp plugin` suite.
- "~20s StartPeers delay from observer polling": FALSE. That number conflated the ze-test
  `go build` time with startup. The manual full-log capture shows StartPeers is fast.
- "The reactor drops or rejects an inbound connection while the peer's outbound retry loop is
  cycling, so a faster backoff starves establishment": FALSE, and it is NOT a missing feature.
  The accept-while-cycling path exists and is wired: `peer_run.go:126-137` selects the backoff
  on `p.inboundNotify` alongside `p.clock.After(delay)` and restarts `runOnce` immediately
  WITHOUT doubling the delay; `reactor_connection.go:151-163` (`acceptOrReject`) buffers the
  connection via `peer.SetInboundConnection` rather than closing it on `ErrNotConnected` /
  `ErrSessionTearingDown` / `ErrAlreadyConnected` for a passive peer, commented as handling
  "the race where the remote reconnects faster than our session teardown";
  `peer_connection.go:67-80` stores the conn and signals the size-1 notify channel. Do not go
  looking for a missing inbound-accept mechanism in the reactor: read these three first.

### REMAINING (the real open question)
Why does the ze-test check-mode peer never accept a connection when the observer polls during
establishment, while a long-lived sink peer does? Every dial is `connection refused`, so the
ze-test peer is not listening at dial time -- pin whether the observer's dispatch shifts ze's
first-dial timing relative to ze-test peer bind/lifetime, or whether the ze-test peer exits
early. Then decide: (a) event-driven establishment wait in the observer (subscribe to the
peer-up `state` event, never dispatch during the connect window), or (b) an engine change so a
plugin's dispatch during establishment cannot perturb peer connect timing. Redistribute tests
remain blind-sleep (annotated, passing) until this is resolved.

## Required Reading

- `internal/component/bgp/plugins/redistribute_egress/register.go` (peer-up state subscription + replay trigger)
- `internal/component/bgp/plugins/redistribute_egress/replay.go` (replay-on-request flow + coordinator)
- `internal/component/bgp/reactor/session_read.go` (`processMessage`, establishment path)
- `internal/component/bgp/reactor/reactor_notify.go` (message counters, `notifyMessageReceiver`)
- `plan/spec-fixit-migrate-sleeps-infra.md` (Mistake Log / Failed Approaches: the bisection)
- `plan/spec-redistribute-late-join-replay.md` (the behavior the fix must not regress)
- `ai/rules/diagnosis-before-fix.md`, `ai/rules/no-fabrication.md`

## Current Behavior

Source files read during investigation:
- [ ] `internal/component/bgp/plugins/redistribute_egress/register.go`
- [ ] `internal/component/bgp/plugins/redistribute_egress/replay.go`
- [ ] `internal/component/bgp/reactor/session_read.go`
- [ ] `internal/component/bgp/reactor/reactor_notify.go`

Behavior to preserve: late-join replay (spec-redistribute-late-join-replay) must keep
working: a peer that establishes AFTER a redistribute injection must still receive the
current redistribute route set via the peer-up->ReplayRequest->targeted-inject path.
`redistribute-as112-announce.ci` (2-peer) must keep passing. Plain-BGP polling tests
(`nexthop-self`) must keep passing. The fix changes only the pathological single-peer +
active-observer stall, not the replay contract.

## Data Flow

### Entry Point
A BGP peer reaches Established; the reactor emits a `state` (down->up) event. In parallel,
an external observer plugin (the `.ci` test's `.run`) may call the plugin engine
(`dispatch-command`, `quiesce`) or read its callback connection (`wait_for_event`).

### Transformation Path
`register.go:83` subscribes the redistribute plugin to `["state"]`. On the down->up edge
(`register.go:92-93`, `OnStructuredEvent`) it calls `coord.onPeerUp(bus, peerAddr)`, which
allocates a monotonic replayID and emits `redistevents.ReplayRequest{replayID}`
(`replay.go:6-11`). Producers re-emit `RouteChangeBatch{ReplayID}`; the coordinator looks
up replayID->peer and injects the current redistribute route set to that ONE peer
(`replay.go:11-14`). So establishment synchronously drives a plugin-facing state dispatch
plus a replay injection back toward the reactor.

### Boundaries Crossed

| From | To | Shared point |
|------|----|--------------|
| Reactor session goroutine (establishment) | plugin engine RPC serialization | plugin-engine command/dispatch lock |
| Redistribute plugin state-event callback | replay coordinator inject | reactor forward pool |
| External observer engine RPC (`dispatch`/`quiesce`/`wait_for_event`) | same plugin-engine serialization | the establishing window |

### Integration Points
- `redistribute_egress` state subscription (`register.go:83`).
- Reactor establishment / forward-pool drain that `quiesce()` waits on.
- The plugin-engine command/dispatch serialization shared by observer and plugins.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| Single-peer redistribute peer reaches Established while the observer polls the engine (`wait_for_event`/dispatch) during establishment | -> reactor establishment path + redistribute peer-up replay (`register.go:92`, `replay.go` coordinator) | new `.ci` / reactor test asserting the peer establishes; FAILS (stall, `connections-established: 0`) before the fix, PASSES after |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Reactor establishment completes while a plugin issues an engine RPC during the establishing window | `internal/component/bgp/reactor/*_test.go` (new) | no lock-order / re-entrancy stall on the single-peer establishment path |
| Replay-on-peer-up still fires exactly once after establishment | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | late-join replay contract preserved (AC-5) |

### Functional Tests

| Test | Validates |
|------|-----------|
| `bgp-redistribute-announce.ci` converted to deterministic waits | establishment + eor-sent + updates-sent poll replaces `time.sleep`; passes 3x + concurrently |

## Files to Modify

- `internal/component/bgp/reactor/` (establishment path) OR
  `internal/component/bgp/plugins/redistribute_egress/replay.go` / `register.go`
  (make the peer-up replay dispatch non-re-entrant w.r.t. establishment) — exact file
  determined by AC-2 root-cause.
- `test/plugin/bgp-redistribute-*.ci` (7 files) + `api-raw.ci` / `api-route-refresh.ci`
  (convert off `time.sleep`).
- `test/.ci-sleep-baseline` (ratchet down).

## Implementation Steps

1. Reproduce deterministically (single-peer redistribute + observer calling
   `wait_for_event` once during establishment). Capture goroutine dumps at the stall.
2. Confirm/refute H1-H3 (below) from the dumps; cite the producing lock/queue `file:line`.
3. Fix at the owning layer per `ai/rules/diagnosis-before-fix.md` (likely: async /
   non-re-entrant peer-up replay dispatch, or decouple `quiesce` drain from
   peer-established state). Never weaken the test.
4. Convert the 7 redistribute tests (+ api-raw/route-refresh) to the proven
   `established -> eor-sent (show bgp summary) -> updates-sent (show bgp peer detail,
   reactor_notify.go:268)` recipe; verify each 3x + concurrently; ratchet the baseline.
5. Confirm no regression in `redistribute-as112-announce.ci` and the replay tests.

### Hypotheses (confirm/refute in step 2)
- H1: observer RPC + replay-on-establish `state` dispatch contend on the plugin-engine
  serialization point; the replay inject issued from inside the peer-up callback re-enters
  a path the reactor still holds during establishment (single-peer timing specific).
- H2: circular wait: `quiesce`/forward-pool drain gated on peer-established, while
  establishment is indirectly gated on the replay dispatch completing.
- H3: `state` delivery to observer vs. redistribute plugin races; the observer reading its
  callback reorders/drops the peer-up edge the coordinator needs.

## Acceptance Criteria

- AC-1: a regression test reproduces the stall and fails before the fix.
- AC-2: root cause cited to `file:line`, with H1-H3 confirmed/refuted from goroutine evidence.
- AC-3: a single-peer redistribute session establishes while the observer polls the engine.
- AC-4: the 7 `bgp-redistribute-*` tests (+ api-raw/route-refresh if affected) converted off
  `time.sleep`, verified 3x + concurrently, baseline ratcheted.
- AC-5: no regression in `redistribute-as112-announce.ci` or the replay tests.

## Risks & Assumptions

- A-1 (unvalidated): the stall is a re-entrancy/lock-order bug, not a protocol requirement.
  Validate via goroutine dump before any fix.
- R-1: reordering establishment vs. replay dispatch could regress late-join replay; guard
  with spec-redistribute-late-join-replay tests.
- R-2: as of 2026-07-14 a concurrent session's `internal/component/iface` break prevents
  `make ze`; this spec cannot be implemented until the tree builds.

## Checklist

- [ ] Stall reproduced with a failing regression test (AC-1)
- [ ] Root cause cited `file:line`, H1-H3 resolved (AC-2)
- [ ] Fix applied at owning layer; single-peer establishes with active observer (AC-3)
- [ ] 7 redistribute tests + api-raw/route-refresh converted, verified, baseline ratcheted (AC-4)
- [ ] as112-announce + replay tests still green (AC-5)
- [ ] Tests written (stall regression test + converted functional tests)
- [ ] Tests FAIL before the fix (stall reproduced, `connections-established: 0`)
- [ ] Tests PASS after the fix
- [ ] `make ze-test` green
- [ ] Review Gate: `/ze-review` clean (0 BLOCKER, 0 ISSUE)
