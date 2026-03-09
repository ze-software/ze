# 376 — Read Pause for TCP Backpressure

## Objective

Add proactive backpressure to the BGP engine: pause reading from peers under stress so kernel recv buffers fill, TCP window shrinks, and senders throttle naturally. KEEPALIVE sending continues independently; hold timer expiry is the safety valve (RFC 4271 Section 6.5).

## Decisions

- **Pause gate mechanism:** `atomic.Bool` fast-path check + channel-based blocking. O(0) overhead on unpaused path (just an atomic load). `resumeCh` is created by `Pause()` and closed by `Resume()` to unblock the select.
- **Cancel goroutine calls Resume():** ensures pause gate never blocks shutdown — the existing close-on-cancel goroutine calls `s.Resume()` before closing the connection.
- **Three-level API:** Session (Pause/Resume/IsPaused), Peer (PauseReading/ResumeReading/IsReadPaused), Reactor (PausePeer/ResumePeer/PauseAllReads/ResumeAllReads). Each level delegates downward.
- **Worker pool thresholds:** 100% high-water (channel full) / 10% low-water. Spec originally proposed 90%/50% but implementation used wider band to reduce oscillation. `bpReminderInterval` for periodic WARN during sustained backpressure.
- **Tier 2 (SO_RCVBUF syscall shrinking):** documented in spec only, not implemented. Deferred indefinitely.

## Patterns

- **Pause gate in read loop:** check `s.paused.Load()` before `readAndProcessMessage()`. If true, `waitForResume()` selects on `resumeCh`, `ctx.Done()`. The cancel goroutine handles `errChan` and calls `Resume()` so `waitForResume` doesn't need to watch `errChan` directly.
- **Idempotent Pause/Resume:** multiple calls are safe. `pauseMu` protects channel create/close. Atomic bool checked under lock for state transitions.
- **Peer delegates to session:** `PauseReading()` locks peer mutex, gets session, calls `session.Pause()`. Nil session is a no-op.

## Gotchas

- **Spec had stale paths:** `internal/plugins/bgp/reactor/` should be `internal/component/bgp/reactor/` — the package was relocated during arch-0 restructuring.
- **Threshold mismatch:** spec said 90%/50%, implementation uses 100%/10%. The wider band (90% of capacity between high and low) significantly reduces pause/resume oscillation. The tests validate the actual thresholds.
- **waitForResume must re-check closeReason:** after `resumeCh` is closed, it could be a real resume or a shutdown-triggered unblock. Must check `closeReason` to distinguish.

## Files

- `internal/component/bgp/reactor/session_flow.go` — Pause(), Resume(), IsPaused(), waitForResume()
- `internal/component/bgp/reactor/session.go` — pause fields (paused, pauseMu, resumeCh), Run() gate, cancel goroutine Resume() call
- `internal/component/bgp/reactor/peer_send.go` — PauseReading(), ResumeReading(), IsReadPaused()
- `internal/component/bgp/reactor/reactor_connection.go` — PausePeer(), ResumePeer(), PauseAllReads(), ResumeAllReads()
- `internal/component/bgp/plugins/rs/worker.go` — onLowWater callback, backpressure detection
- `internal/component/bgp/plugins/rs/server.go` — wires onLowWater to call ResumeAllReads
