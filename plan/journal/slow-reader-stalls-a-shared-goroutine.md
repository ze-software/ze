# A slow reader stalls the goroutine everything shares

One goroutine serves many independent units (tunnels, sessions, peers), and it
also delivers events to readers that run on it. One reader does bounded but
slow work there, such as a network request that waits for a server. While it
waits, the goroutine serves no other unit, so units that have nothing to do
with the slow one miss their timers and time out.

Nothing fails and nothing is logged: every call returns, only late. The cost
appears on other units, far from the reader that caused it, and only while the
remote party is slow.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-11 | single-owner-lifecycle-5-session-end-locals | L2TP reactor `run` (`internal/component/l2tp/reactor.go`) publishing to `radiusAcct.onSessionDown` (`plugins/authradius/acct.go`) | One reactor goroutine serves packets, ticks, kernel results and PPP events, and publishes SessionDown itself. In-process readers run on it, and Accounting-Stop is sent there with a 10 s bound (Start and Interim use the session's own task). While a RADIUS server is slow no tunnel's packet is handled | fixed: 93ff938cdb; the session's own task sends the Stop (test/l2tp/slow-accounting-control.ci) |
| 2026-11-14 | bgp-session-write-wedge (Phase 4, found) | route-server fast path `tryDirectWriteNoFlush` (`bgp/reactor/forward_rs_direct.go`) | It writes to a destination on the SOURCE peer's read goroutine. A destination that stops reading blocks that write for up to the write budget (30 s by default), so the source reads nothing meanwhile, and a source whose hold time is under the budget can lose its own session to its hold timer. Read, not run | open: the fast path never blocks the source's read goroutine on a destination's socket |
