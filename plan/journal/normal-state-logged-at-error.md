# A normal state logged at Error

A caller that classifies an RPC failure by its shape (shutting down,
connection lost, everything else) logs the "everything else" arm at Error.
A refusal that only reports a legitimate state of the network, such as no peer
being up to receive a forward, then reaches the operator as an Error with no
fix to apply, and buries the Errors that need one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-22 | - (handover 19) | bgp-rr `updateRouteWithMeta` (`rr/rr.go`) | With every other client down, `update-route failed` logs at ERROR for `no established peers to forward to`: `isConnectionError` does not match `errNoEstablishedPeersToForwardTo` (`reactor_api_forward.go`). Seen in `bgp-rr-rpki-revalidate-relay.ci` stderr | open: predates that test |
| 2026-10-23 | spec-exabgp-6-api-compatibility (OD-2 part 2) | bgp-adj-rib-in `applyDecision` (`adj_rib_in/rib_validation.go`) | A verdict for a route withdrawn while pending is buffered by `storeEarlyDecision` as if it preceded its UPDATE, and expires a minute later with a WARN its comment calls a bug. The wire is right; only the log is wrong | open: the store must tell a verdict for a generation it already withdrew from one not yet received |
