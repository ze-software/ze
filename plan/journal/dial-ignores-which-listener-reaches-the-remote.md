# A dial ignores which listener reaches the remote

With several listeners configured, the dial path takes the first one whatever
the remote, so a listener that cannot reach the remote is used while another
that can sits idle.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - (L2TP review, walked into) | `Subsystem.PlaceOutgoingCall` (`l2tp/outgoing_call.go`), PPPoE relay sink (`relay_sink.go`) | Both dialed from `s.reactors[0]`, so a loopback first server refused a remote another listener reached | fixed: `reactorForRemoteLocked` (`outgoing_call.go`) picks a listener matching IP family and loopback reachability for `PlaceOutgoingCall` and `relay_sink.go`; `validateDialSources`/`reloadDialSources` aligned; `outgoing_call_test.go`, `relay_sink_test.go`, `config_test.go` |
