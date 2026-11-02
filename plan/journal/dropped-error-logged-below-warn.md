# A caught and dropped error is logged below WARN

Code catches an error, does not return it, and goes on as if the operation had
happened. The only record of the failure is a debug or info line, which no
running system shows, so the missing behavior has no trace.

Captured: 2026-11-02 rule go-standards/directives/log-a-dropped-error-at-warn-or-above

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | - | MOBIKE source lookup, `routeSourceTo` (`internal/component/ike`) | Under the XFRM catch-all discard the UDP connect that found the source address failed with EPERM, logged at debug, so UPDATE_SA_ADDRESSES was never sent and `mobike-initiator` failed every nightly with nothing in the log | fixed: d6ef7093a8 asks the routing table with RTM_GETROUTE, which never crosses the SPD |
| 2026-11-02 | spec-dropped-error-0-umbrella | tree-wide, `./le arch refuse-by-default report` | b2448d44e7 refuses the shape on changed lines; the report lists 328 existing instances (bgp 120, ike 39, iface plugins 20, plugin 15, l2tp 15, tftpserver 10, ospf 10, sysctl 9, exabgp bridge 8, bfd 8), `serviceMobike` among them, each a failure whose only record the default level hides | open: each instance returned, counted, raised to WARN, or annotated with its guard |
