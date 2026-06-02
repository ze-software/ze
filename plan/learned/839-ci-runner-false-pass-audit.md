# 839 -- ci-runner-false-pass-audit

## Context

An audit of the functional `.ci` test harness (`internal/test/runner/`) for the
single property "a failing test must never report PASS" found four ways the
runner could pass a test that was actually failing. The Make/unit/fuzz/functional
gates propagate exit codes correctly and `comparePluginJSON` is strict; the gaps
were all in how per-test assertions reach the pass/fail decision.

## Decisions

- Every assertion directive that the parser accepts MUST be consumed in the
  decision path. `reject=syslog:` was parsed, stored, and counted toward
  `hasOutputAssertion`, but `validateLogging` never checked it and the syslog
  capture server was only started for `expect=syslog`. A parsed-but-unevaluated
  assertion is the same class as "feature not wired": it silently passes.
- A successful sub-check is a self-validation signal, not a license to skip the
  rest. The HTTP path used an early `return true` after HTTP+file checks that
  skipped stderr/stdout/syslog assertions and the universal observer-fail
  sentinel. HTTP presence now feeds `selfValidated`; it no longer short-circuits.
- Assertion patterns must fail closed. An empty `expect=/reject=` regex compiles
  to a pattern that matches everything, so a typo passes vacuously. Empty
  patterns are rejected at parse time and guarded again in `validateLogging`.
  For reject paths, compile the regex explicitly rather than relying on
  `Server.Match`, which returns false on an uncompilable pattern (fail open).
- Content matchers that key on a subset of inputs must fail loudly when they
  cannot match, if nothing else validates the message. `validateJSON` matches by
  NLRI but `extractNLRIs` only understands unicast/flow; a json-only expectation
  (no wire `expect=bgp:hex=`) in another family was skipped. It now errors unless
  an authoritative wire-hex check backs the same message.

## Consequences

- When auditing a test runner, enumerate every assertion field and grep that each
  is read in the decision path, not just parsed. A 0-reference field is a
  false-pass candidate.
- Prefer guards that fire only on genuinely-unvalidatable cases (e.g. json-only +
  unsupported family, nil syslog server + syslog assertion) so hardening is
  zero-regression: verified against `encode` 51/51, `plugin` 400/400,
  `ui` 128/128, `web` 64/0 and the runner unit tests.
- Failure strings stay specific and greppable (`reject=syslog pattern found:`,
  `no syslog server was started`, `NLRI content the matcher cannot extract`) so
  both humans and agents can parse them.

## Files

- `internal/test/runner/runner_validate.go`
- `internal/test/runner/runner_exec.go`
- `internal/test/runner/record_parse.go`
- `internal/test/runner/runner_test.go`
- `internal/test/runner/record_test.go`
- `internal/test/runner/record_newformat_test.go`
