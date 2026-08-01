# 800 -- BGP Chaos Integration Tests

## Context

The chaos testing tool (ze-chaos) had extensive unit tests for its components (scenario generation, validation model, event log, properties, shrinking) but no automated test that ran the tool against a real Ze instance. The spec called for `--managed` and `--config-only` flags, `.ci` smoke tests, and Makefile targets to close this gap. By the time this spec was implemented, all the feature code already existed under different names: fork mode (default) replaced `--managed`, and `--config-only`, port auto-allocation, `.ci` files, and test runner registration were all pre-existing. The remaining gap was unit test coverage for these entry points.

## Decisions

- Kept fork mode as the default behavior over adding a separate `--managed` flag, because fork mode (pipe config via stdin to child Ze process) is simpler and avoids temp files entirely
- Tested via the `run([]string)` function over spawning subprocesses, because `run()` returns an exit code and is directly testable without binary compilation
- Tested utility functions (`allocatePort`, `waitForZe`, `checkPortFree`) directly over only testing through `.ci` integration, because unit tests run faster and isolate failure modes

## Consequences

- All chaos integration test infrastructure is now in place: unit tests in `main_test.go`, functional tests in `test/chaos/*.ci`, Makefile targets in `mk/test-chaos.mk`
- Future phases of chaos testing (if any) can add `.ci` files and they will be auto-discovered by `ze-test bgp chaos --all`
- The spec's naming diverges from the implementation: spec says `--managed`/`managed.go`, code has fork mode/`fork.go`

## Gotchas

- The spec was written before fork mode existed and its audit showed 0/33 items implemented, when in fact 33/33 of the feature code was pre-existing. Always audit against the actual codebase before starting implementation.
- Ze config uses `peer` blocks, not `neighbor`. Tests asserting config content need to match the actual Ze config syntax.
- Pre-existing lint warnings in `orchestrator_run.go` (hugeParam gocritic) are unrelated to this work.

## Files

- `cmd/ze/ze_chaos_main_test.go` (created: 15 unit tests for config-only, port allocation, input validation)
- `plan/spec-chaos-4-bgp-integration.md` (updated: audit, checklists, status)
