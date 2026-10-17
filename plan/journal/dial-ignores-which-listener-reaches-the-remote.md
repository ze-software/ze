# A dial ignores which listener reaches the remote

With several listeners configured, the dial path takes the first one whatever
the remote, so a listener that cannot reach the remote is used while another
that can sits idle.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - (L2TP review, walked into) | `Subsystem.PlaceOutgoingCall` (`internal/component/l2tp/outgoing_call.go`), the PPPoE relay sink (`relay_sink.go`) | Both dial from `s.reactors[0]`, so with several `environment l2tp server` blocks a loopback first server refuses a remote another listener reaches. Read, not run | open: pick the reactor whose listener can reach the remote, or refuse naming the server to use |
