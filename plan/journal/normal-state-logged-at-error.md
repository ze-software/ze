# A normal state logged at Error

A caller that classifies an RPC failure by its shape (shutting down,
connection lost, everything else) logs the "everything else" arm at Error.
A refusal that only reports a legitimate state of the network, such as no peer
being up to receive a forward, then reaches the operator as an Error with no
fix to apply, and buries the Errors that need one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-22 | - (handover 19) | bgp-rr `updateRouteWithMeta` (`rr/rr.go`) | With every other client down, `update-route failed` logs at ERROR for `no established peers to forward to`: `isConnectionError` does not match `errNoEstablishedPeersToForwardTo` (`reactor_api_forward.go`). Seen in `bgp-rr-rpki-revalidate-relay.ci` stderr | open: predates that test |
