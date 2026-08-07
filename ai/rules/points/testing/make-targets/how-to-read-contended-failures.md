---
kind: note
level:
stage:
---
How to read contended failures:
- `near_timeout` kind: the test consumed >80% of its timeout but the context
  deadline did not fire. This is CPU starvation, not a bug. Rerun on a quiet
  machine.
- `host-load` field in failure group JSON: load average, CPU count, and
  concurrent process counts at run start.
- Timing baseline updates are suppressed during contended runs to prevent
  slow-run pollution of the EMA.
- The project rejects retry-on-failure masking. Contended verdicts are for
  classification, not automatic retry.
<!-- source: internal/test/runner/hostload.go -- HostLoad, Contended, IsNearTimeout -->
