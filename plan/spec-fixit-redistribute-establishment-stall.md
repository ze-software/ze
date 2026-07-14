# spec-fixit-redistribute-establishment-stall

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-migrate-sleeps-infra (P0 carve-out); spec-redistribute-late-join-replay |
| Phase | 0/1 (investigation) |
| Updated | 2026-07-14 |

## Task

Root-cause and fix a reproducible, config-specific BGP establishment stall: when an
external observer plugin issues ANY engine activity (dispatch / quiesce / show-poll,
or even a bare `wait_for_event` callback read) during BGP session establishment, a
**single-peer redistribute session never establishes** (`connections-established`
stays 0). Only the original blind `time.sleep` (observer fully idle, not even reading
its callback connection) lets it establish. This blocks deterministic-wait conversion
of the redistribute test bucket under spec-migrate-sleeps-infra:
`bgp-redistribute-{announce,burst,explicit-nhop,filtered-out,metrics,nexthop-self,withdraw}.ci`
(7 tests, ~16 sleeps), plus `api-raw.ci` / `api-route-refresh.ci` if they share the trigger.

## Required Reading

- `internal/component/bgp/plugins/redistribute_egress/register.go` (peer-up state subscription + replay trigger)
- `internal/component/bgp/plugins/redistribute_egress/replay.go` (replay-on-request flow + coordinator)
- `internal/component/bgp/reactor/session_read.go` (`processMessage`, establishment path)
- `internal/component/bgp/reactor/reactor_notify.go` (message counters, `notifyMessageReceiver`)
- `plan/spec-migrate-sleeps-infra.md` (Mistake Log / Failed Approaches: the bisection)
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
