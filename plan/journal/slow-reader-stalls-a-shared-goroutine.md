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
| 2026-11-11 | single-owner-lifecycle-5-session-end-locals | L2TP reactor `run` (`internal/component/l2tp/reactor.go`) publishing to `radiusAcct.onSessionDown` (`plugins/authradius/acct.go`) | One reactor goroutine serves packets, ticks, kernel results and PPP events, and publishes SessionDown itself. In-process readers run on it, and Accounting-Stop is sent there with a 10 s bound (Start and Interim use the session's own task). While a RADIUS server is slow no tunnel's packet is handled. Read, not run | open: the Accounting-Stop wait leaves the publishing goroutine |
