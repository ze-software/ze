---
kind: directive
level: MUST
stage:
---
- Non-race first-party Go compilation MUST set `CGO_ENABLED=0` in the process environment.
- This covers binaries, tests, benchmarks, fuzzing, `go run`, nested helpers, and installed project tools.
- A test-only command that uses `-race` MAY set `CGO_ENABLED=1`.
- Race binaries MUST NOT ship or serve as release or build evidence.
- Inherited CGO defaults MUST NOT be used.
