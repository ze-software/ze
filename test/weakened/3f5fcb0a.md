| Test | Reason |
|---|---|
| TestSessionDownFollowsBufferedDeliveries | Nothing left the suite. The top-level body now runs ten subtests. Its eight assertions moved, unchanged, into the shared helpers `leftWithoutDown`, `finish` and `oneDown` of `peer_run_test.go`. Every subtest, the original remote-close case among them, runs all eight, and adds the delivery-worker exit, exactly one leave arm, and (where the case names another drainer) the arm running off the read goroutine. |
