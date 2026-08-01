# 1144: RPKI validation uses store-and-reconcile, not retry-poll

## Context

When Ze receives a BGP UPDATE, both the RPKI plugin and adj-rib-in plugin
subscribe to the same event via the plugin event bus. There is no delivery
ordering guarantee between subscribers.

RPKI completes its VRP lookup and dispatches a command to adj-rib-in
(e.g. `adj-rib-in accept-routes ...`). If adj-rib-in hasn't stored the
route yet, the command used to fail.

## Previous approach (replaced)

`dispatchValidation` retried up to 20 times at 50ms intervals (up to 1s total).
This was increased from 3 retries with exponential backoff (30ms total) because
Docker interop testing showed the original window was insufficient. Retry-polling
is the wrong synchronization primitive: it wastes time, has an arbitrary ceiling,
and obscures the actual dependency.

## Implemented fix

adj-rib-in buffers early RPKI decisions in an `earlyDecisions` map. When
`accept-routes` or `reject-routes` arrives before the route exists as pending,
the decision is stored. When the route later arrives and enters the pending path,
`applyEarlyDecision` checks the buffer and immediately promotes or drops it.

RPKI dispatches a single command with no retry loop. Stale early decisions are
swept every 5s and warn on expiry (1 minute timeout, indicates a bug).

Peer-down cleanup clears both pending routes and early decisions.

## Key files

- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` (EarlyDecision type, buffer, sweep)
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` (accept/reject buffer on miss)
- `internal/component/bgp/plugins/adj_rib_in/rib.go` (applyEarlyDecision call site, struct field)
- `internal/component/bgp/plugins/rpki/rpki.go` (retry loop removed)

## Found during

spec-interop-gap-coverage, scenario 43-rpki-frr.

## Files

None recorded.
