# An interop scenario's checker waits on a peer the scenario never starts

The checker names a peer container the scenario directory carries no
configuration for, so the lab never starts it and the first assertion fails
before Ze's behaviour is read. The scenario proves nothing, whatever Ze does.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | route-policy-0-umbrella (found, not caused) | `ospf-multiaf-v4-frr` checker (`checkers.go`) | No FRR config in the scenario, yet the checker waits on `peerFRR`: "unknown peer frr" at c356b97d17^ and 697dc6a618. Ze also logs "no usable link-local source" on eth0 | open: give the scenario the FRR it checks, trace the warning, run green and red |
