| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | time | `time.Since(estAt)` bypassed simulated clock | fixed: replaced with `clock.Now().Sub(estAt)` |
| 2026-03-21 | - | time | same bypass in operational-commands handlers | fixed: replaced all `time.Now()` with clock instance calls |
| 2026-10-27 | spec-add-path-limit-send-receive (verify changed triage at e6474aa200) | `internal/component/bgp/reactor/reactor_connection.go` (`b6d84e8104`) | `TestNoDirectTimeCalls` (`internal/core/clock`) refuses `conn.SetWriteDeadline(time.Now().Add(fwdWriteDeadlineDefault))`, added by the DelayOpen wiring | open: take the deadline from the reactor clock |
