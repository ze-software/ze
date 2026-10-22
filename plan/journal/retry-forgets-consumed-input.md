| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-22 | spec-bgp-explicit-final-policy (verification) | `internal/test/peer` message and linger loops | A timeout after consuming part of a BGP header or body discarded that prefix; retry parsed the remainder as a new header. Exact-frame and following-rejection regressions failed. | fixed: connection-owned reader retains offsets across timeouts. Targeted race20 and the peer package passed; actual final-reload smoke passed in 12.9s. Cause of the earlier intermittent fence miss remains unproved. |
