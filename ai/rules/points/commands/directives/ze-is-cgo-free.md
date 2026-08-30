---
kind: directive
level: MUST
stage:
---
**First-party Go compilation MUST set `CGO_ENABLED=0` in the process environment, covering binaries, tests, benchmarks, fuzzing, `go run`, nested helpers and installed project tools; an inherited CGO default MUST NOT be relied on.** A test-only command that uses `-race` MAY set `CGO_ENABLED=1`, and a race binary MUST NOT ship or serve as build evidence.
