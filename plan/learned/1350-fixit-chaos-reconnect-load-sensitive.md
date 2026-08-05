# 1350 -- A Run That Ends On Virtual Time Cannot Wait For A Real Reconnect

## Context

`TestInProcessChaosReconnect` failed on a loaded host and passed on a quiet one, with
`"1" is not greater than "1"`: a disconnect fired and the peer never recorded a second
`EventEstablished`. The in-process chaos runner drives a VIRTUAL clock and burns 60
virtual seconds in about 0.6 real seconds, then cancels the simulators. The reconnect
that chaos provoked needs REAL time, so the run cut the handshake it had just started and
the assertion counted an event the run had made unreachable. `ai/rules/completion.md`
forbids recording that shape as a load excuse, so the goal was to make the run stop on
state rather than on elapsed time.

## Decisions

- Made `cfg.Duration` an EARLIEST bound for teardown, over lengthening the run or lowering
  the chaos rate: both of those buy green by removing the pressure the test exists to
  apply. `RunConfig.DisconnectAt` in the same package already had this shape.
- Gated the wait on `chaosProgress`, which counts DISPATCHED actions against APPLIED ones,
  over gating it on peer state alone: peer state cannot see an action queued on a channel,
  and `ActionConnectionCollision` holds the simulator for 500 real milliseconds while the
  peer still reads as healthy.
- Required ONE observable recovery per peer chaos knocked down, over requiring a healthy
  peer at the final instant: at chaos rate 1.0 the peer is usually down when the window
  closes, and an earlier gate demanding health held the full 90 second ctx in 4 of 80 runs.
- Excluded `ActionConnectionCollision` from `endsSession`, over treating every chaos action
  as session-ending: RFC 4271 Section 6.8 lets the reactor reject the second connection and
  keep the session, so a demanded fall hangs whenever the reactor is right.
- Hoisted the background clock advancer ABOVE the settle wait, over leaving it after: a
  handshake cannot finish while virtual time stands still, so the wait would never settle.

## Consequences

- A chaos run's end is now a condition, so adding a chaos action means asking whether
  `endsSession` should name it. Naming an action that does not certainly end the session
  makes `Owed` hang until ctx; omitting one that does makes the gate settle too early.
- The settle wait runs after the advance loop, which is the only feeder of `chaosTick`, so
  it can never extend the scenario or schedule more chaos. That is what keeps AC-4 true.
- A real wait exposes real races. This one found `DirectBridge.SendCallback` sending on
  `callbackCh` while `CloseCallbacks` closed it (`pkg/plugin/rpc/bridge.go`). The author
  had wrapped the send in `recover`, which stops the panic and does not stop the race.
  `sendMu` now serializes them. Expect the same when converting any other blind wait.

## Gotchas

- **A spec's Task section states the SYMPTOM, and a later session can read it as the
  DIAGNOSIS.** This spec's Task described chaos arriving every second with no window to
  recover. A session resuming this work concluded the 1 second `ChaosInterval` was the
  defect and raised it to 15 seconds. AC-3 forbade exactly that edit in writing, and the
  fix had already landed. Read the spec's own Acceptance Criteria and Evidence sections
  before editing anything the Task paragraph appears to blame.
- **Measure the hypothesis, not just the outcome.** The interval edit passed under load,
  which looked like proof. Running the OLD value under the SAME load was what refuted it:
  at loadavg 29.5 the shipped 1 second interval passed 3 of 3. A change that "fixes" a
  test it never had to fix is invisible unless the discriminator is run both ways.
- **`go test` caching makes a load experiment silently vacuous.** The first under-load run
  reported exit 0 with `(cached)`, so the binary never executed while the load was
  applied. Any load or timing measurement needs `-count=1` or greater.
- **`recover` around a channel send is not a fix for a send-close race.** It converts the
  panic and leaves the data race, and `-race` still fails.
- **The `-race` binary is the one that fails.** Isolated it takes 6.46 seconds against 4.13
  plain, which matched the 6.30 seconds in the original report and identified the failing
  run as the race pass.
- **`chaosProgress.WentDown` is gated on `perturbed`, which only `EventChaosExecuted` sets,
  and the simulator calls `OnSessionEnd` BEFORE emitting that event.** So the callback's
  `WentDown` is a no-op on a peer's first chaos-caused fall, and the latch is set moments
  later by `EventDisconnected` in the drain. The verdict is unaffected because `es.Set` in
  the same callback is immediate, but both writes in that callback do not take effect at
  the same time.

## Files

- `internal/chaos/inprocess/chaos.go` -- `waitSettled`, `chaosProgress`, `endsSession`
- `internal/chaos/inprocess/runner.go` -- call site, advancer hoisted, latches from the drain
- `internal/chaos/peer/simulator.go` -- `SimulatorConfig.OnSessionEnd`
- `pkg/plugin/rpc/bridge.go` -- `sendMu`, `beginSend`, `endSend`
- `internal/chaos/inprocess/runner_test.go` -- UNCHANGED, and AC-3 requires it stays so
