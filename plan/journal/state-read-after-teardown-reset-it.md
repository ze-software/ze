# A decision reads state that the teardown has already reset

Cleanup that branches on "how far did this get" reads the state after a deferred
stop has put it back to its initial value, so the branch always takes one side.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-02 | rfc-verdict-fix-ospf (c29) | `startSessionForAdj` onDone (`internal/plugins/ldp/register.go`) | runSession defers onDone then Stop, so onDone reads the state Stop reset: every end calls `recordSetupFailure`, and an operational session never clears its backoff. onDone also emits SessionDown for a session that never sent SessionUp, re-arming the OSPF RFC 5443 stuck timer | fixed: runSession hands onDone the on-operational fact; onDone then clears the backoff and emits SessionDown, else records a setup failure. Tests: `session_end_test.go` |
| 2026-10-11 | bgp-fsm-delayopen-and-automatic-start | `Peer.run` reconnect loop (`peer_run.go`) | The reset of `delay` and `prefixTeardownCount` "after stable session" runs only when `safeRunOnce` returns nil, which happens only on stop, so an Established session ended by NOTIFICATION or hold expiry never resets either backoff | fixed: after any error, `run` restarts both when the cycle reached Established and stayed there at least `reconnectMax` from `EstablishedAt()` (`cycleStayedEstablished`), before the ErrTeardown branch. Tests: `peer_run_backoff_test.go` |
