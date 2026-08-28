---
kind: note
level:
stage:
---
`internal/test/fixture` owns the failure boundary. Its package tests MUST prove
that an unknown driver is refused and a returned error reaches
`fixture.ReportFailure`.
