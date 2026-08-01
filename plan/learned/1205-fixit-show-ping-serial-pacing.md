# Learned: fixit-show-ping-serial-pacing

Spec: `plan/spec-fixit-show-ping-serial-pacing.md` (decouple `show ping`'s batch
ICMP engine so a lossy batch is bounded by ~timeout, not count*timeout).

NOTE (numbering): chose 1205 to dodge heavy contention around 1198-1200; run
`python3 scripts/dev/learned_numbers.py --fix` at drain to reconcile with `.counter`.

## What the change does
Rewrites `doPingCtx` (`internal/component/ping/cmd/ping.go`): it now opens the raw
ICMP socket and hands ownership to a new `runPingBatch`, which drives the streaming
sibling's `runPingSession` (stream.go) in a goroutine, drains its per-probe `out`
channel, sorts the replies by sequence, and aggregates them via
`summarizePingReplies` into the exact same result map the serial engine produced.
Probe sends are paced by an internal `defaultPingBatchInterval` (10ms) while a
seq-keyed receiver matches replies, so a lost probe no longer blocks the next send.
A black-holed `count 100 timeout 30s` that used to take ~50 minutes
(serialized to count*timeout) now finishes in ~(count*interval)+timeout.

## Non-obvious findings / traps
- **The bug was invisible because the engine was untestable.** `doPingCtx` opened a
  real raw socket (CAP_NET_RAW), so no unit test could drive it — the exact reason
  the serial blow-up survived. The fix piggybacks on the seam the monitor-cadence
  sibling already built (`pingConn` interface + injectable `clock.Clock`), so the
  batch is now tested with a fake conn + fake clock, no root, no `time.Sleep`.
- **Reuse the sibling receiver, do NOT write a second one (spec R-4).** `runPingBatch`
  calls `runPingSession` rather than duplicating the sender/receiver/reaper machinery.
  `runPingSession` already emits one map per probe with EXACTLY the batch's per-reply
  shape (`{seq int, status, rtt-ms}`) and closes `out` exactly once on teardown, so
  the batch just collects, sorts, and aggregates. One matcher, no drift.
- **`count == 0` is a landmine when reusing `runPingSession`.** In the streaming
  contract, `count == 0` means "stream until the context is canceled" (unbounded).
  A non-positive count reaching a *bounded* batch would hang forever. `runPingBatch`
  guards `count <= 0`: it closes the handed-in conn and returns the empty well-formed
  result. `parsePingArgs` enforces 1-100 today, but the guard is fail-closed insurance.
  Locked by `TestRunPingBatchCountZeroDoesNotHang`.
- **Replies must be re-sorted by seq.** `runPingSession` emits in *resolution* order
  (a late/lost probe resolves after later ones), but `printPingResults` (offline.go)
  and `show-ping.ci` render `replies[]` in array order and the old serial engine
  produced 0..N-1. `runPingBatch` sorts by `seq` before summarizing to keep output
  deterministic.
- **`replies` MUST stay `[]map[string]any`.** `printPingResults` does
  `results["replies"].([]map[string]any)` — a bare `[]any` would silently drop all
  per-reply lines. `emptyPingResult` returns a non-nil `[]map[string]any` for the
  same reason.
- **Behavior deviation (documented in the spec):** a *mid*-batch `WriteTo`
  error (some probes already on the wire) yields *partial* results instead of the old
  `return nil, error`; pacing makes ENOBUFS/EAGAIN rare (R-3) and partial diagnostic
  output beats an opaque error. `sent = len(replies)` because every collected reply is
  one successful write.
- **Fail-closed on TOTAL send failure (review ISSUE, fixed).** The first draft let a
  *total* send failure (first `WriteTo` fails: ENETUNREACH with no route, EPERM without
  CAP_NET_RAW) fall through to `sent=0/received=0/loss-percent=0.0` with `StatusDone` --
  a transport failure rendered as a healthy 0%-loss answer, the exact
  `ai/rules/fail-closed-guards.md` anti-pattern. `runPingSession` *swallows* the WriteTo
  error (it only emits per-probe maps), so `runPingBatch` cannot see it directly; it
  reconstructs the failure from the empty result: `count > 0 && len(replies) == 0` means
  nothing reached the wire, so it returns `errPingNoProbesSent` (or a ctx-cancel error
  if `ctx.Err() != nil`). `runPingBatch` therefore returns `(map, error)`, not just a
  map. Locked by `TestRunPingBatchSendErrorFailsClosed` (fakePingConn.setWriteErr).
  count<=0 stays `(emptyPingResult, nil)` -- "nothing requested" is not a failure.
- **Fake-clock test trap: RTT timestamp races a post-inject clock advance.** The
  receiver captures `at: clk.Now()` when it forwards a reply; if the test advances the
  clock (to fire timeouts) right after `injectReply`, the reply's RTT is computed
  against the advanced clock, AND a probe's reaper can steal it. Robust pattern for a
  mixed answer/timeout batch test: answer only the *latest-deadline* probes (whose
  reapers cannot fire during the advance) and assert status/counts there, not exact
  RTT. Assert exact RTT only where the clock is stable (all-answered, no post-inject
  advance) or in a pure `summarizePingReplies` unit test. This bit under `-race`
  (min/max off by the advance) on the first draft.

## What proves it
- Unit (fake conn + fake clock, deterministic, `-race` clean x5):
  `TestDoPingBatchHealthyShape` (AC-4/AC-5), `TestDoPingBatchBoundedUnderLoss`
  (AC-1/AC-2/AC-5), `TestDoPingMatchesLateReply` (AC-3), `TestDoPingAllLostBounded`
  (AC-2 worst case), `TestRunPingBatchCountZeroDoesNotHang` (count<=0 guard),
  `TestRunPingBatchSendErrorFailsClosed` (total-send-failure guard),
  `TestSummarizePingReplies` (aggregation math / shape).
- Functional: `test/plugin/show-ping.ci` extended with `check_multiprobe_batch`
  (count=3 batch shape; the old test only did count 1, which never spaced two sends).
  Needs CAP_NET_RAW; validated statically, run under privileged/QEMU CI.

## Scope decisions (spec autonomous defaults)
- A-5: NO `show ping` `interval` CLI arg. Internal pacing only. So AC-7, the
  `ze-ping-cmd.yang` interval leaf, and the `command-reference.md` update are N/A.
- Fork resolved to in-flight seq-keyed map with paced sends (not send-all-then-collect)
  to bound socket burst at `count 100 size 65507` (R-3).

## Files

None recorded.
