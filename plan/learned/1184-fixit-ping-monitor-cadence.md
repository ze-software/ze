# 1184 - fixit: monitor ping cadence holds under loss

Spec: `plan/spec-fixit-ping-monitor-cadence.md`
Scope: `internal/component/ping/cmd/stream.go` (+ new `stream_test.go`)

## Problem

`monitor ping <dest> interval 1s timeout 5s` did not probe every second on a
lossy path: it probed every ~`max(interval, timeout)`. The old `streamPing` was
strictly serial per probe: send, then **block in `ReadFrom` until this probe's
reply or its `start+timeout` deadline**, then sleep `interval - elapsed`. A lost
packet therefore cost the full `timeout` before the next probe, and the trailing
sleep could only subtract what the probe already consumed, never claw back the
deficit. Second defect: the loop only ever matched the CURRENT `seq`, so a reply
for seq N arriving after probe N+1 was sent failed the match and was discarded,
inflating reported loss exactly on a recovering path.

## Fix

Rewrote the session as a sender/receiver split behind a testable seam:

- `pingConn` interface (`WriteTo`/`ReadFrom`/`Close`, satisfied by
  `net.PacketConn`) + injected `clock.Clock`. `streamPing` opens the raw socket
  and calls `runPingSession(ctx, conn, clock.RealClock{}, ...)`; tests call
  `runPingSession` with a fake conn + `sim.FakeClock`.
- Sender: probes paced by `clk.NewTicker(interval)`; first probe sent
  immediately. Send is decoupled from reply latency, so cadence holds under loss.
- Receiver goroutine: a **pure reader**. Loops `ReadFrom`, applies the same
  length/type/id/source checks, forwards `{seq, arrivalTime}` on an internal
  channel. Never touches `out` or the in-flight map.
- Matching: main goroutine owns an in-flight map keyed by wire seq
  (`seq -> {num, sentAt, timer}`); a reply is attributed to its own seq
  regardless of what else is in flight. A seq not in the map (duplicate, expired,
  forged) is ignored — cannot resurrect or double-emit.
- Timeouts: per-probe `clk.AfterFunc(timeout, ...)` reaper that only sends the
  wire seq on an `expire` channel. Each probe times out at ITS deadline.
- Completion (`count`): ends when all sent AND in-flight empty — no trailing
  idle after the last reply, no early exit before it.
- Teardown: `Close` unblocks the receiver's `ReadFrom`; a `done` channel unblocks
  a pending reply-send; main joins the receiver, then closes `out` exactly once.

## Key design decision (diverges from spec hypothesis)

The spec floated a "mutex-guarded shared in-flight map". Instead the map is
**single-owner** (main select loop only); the receiver and the AfterFunc reaper
only *signal* over channels and never touch the map. Result: no shared mutable
state, so the race AC holds **by construction**, not by lock discipline. This is
the cleaner answer whenever a "shared + mutex" design can be refactored to
single-owner + channel signalling.

## Testability was the actual deliverable

The bug survived because `streamPing` opened a raw ICMP socket (`CAP_NET_RAW`),
so no unit test could reach it. The seam (`pingConn` + `clock.Clock`) is what
made loss, delay, reordering, timeout and teardown deterministic with no root
and no wall-clock sleeps. `internal/test/sim.FakeClock` (ticker via
`FireTickers`, timers via `Add`) is the right tool: component-tier test files
already import it freely.

## Gotchas

- Arm the AfterFunc reaper and record `sentAt` BEFORE `WriteTo`, then unwind on
  write error. Otherwise a test that observes the write (via the fake conn's
  blocking `wrote` handoff) and then advances the clock can race the timer
  registration and hang.
- On `ListenPacket` error, `streamPing` must still `close(out)` itself (the
  success path hands the close to `runPingSession`) — consumers detect session
  end only by the channel closing; exactly one close on every path.

## Not done here (honest)

- `test/ping/monitor-ping-cadence.ci` (privileged/QEMU end-to-end): out of this
  task's file scope AND unrunnable without `CAP_NET_RAW`/QEMU. Assumption A-1
  ("cadence really degrades") remains proven only by reading the old source +
  the deterministic unit tests; a privileged run is the remaining evidence.
- `doPingCtx` (`show ping`) shares the serial structure but has no interval and
  a bounded worst case — deliberately out of scope (separate spec).
