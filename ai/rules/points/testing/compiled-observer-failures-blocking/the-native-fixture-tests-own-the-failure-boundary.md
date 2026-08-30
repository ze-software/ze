---
kind: directive
level: MUST
stage:
---
**`internal/test/fixture` owns the failure boundary, and its package tests MUST
prove that an unknown driver is refused and that a returned error reaches
`fixture.ReportFailure`.**
