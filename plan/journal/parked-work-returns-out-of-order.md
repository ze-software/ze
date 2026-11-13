# Parked work returns out of order

Work that was admitted in order is set aside (parked, held, moved to an
overflow) and comes back in a different order than it went in.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | deployment-user-journeys | BGP reactor forward pool, policy replay hold | `TestPolicyFinalReplayPreservesPreAdmittedBatchOrder` failed 1 of 20 under `-race`, no race report: handler order was withdraw 9201 then announce 9202, the wire had the announce first, so the peer loses the route. | open: `endPolicyReplay` lowers the hold by bare atomic while `takeOverflowReleased` re-reads `forwardOrderHold` per parked item, so a drop between 9201 and 9202 releases 9202 first. Fix: judge the hold once per scan, or hold each live item after a held one. Repro: `-race -count=200`. |
