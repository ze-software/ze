# failure report names a command that does not exist

A failure report earns its place by telling the reader what to run next. When
that command does not exist, the reader pays twice: once for the failure, and
once for finding out that the advice was wrong. The advice is never executed by
the code that prints it, so nothing goes red when the target it names is renamed
or was never created. Only a test that holds the printed name against the build
system makes that drift visible.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | verify-scope-4-suite-budget-and-ci | mk/test-functional.mk and scripts/status/verify_run.go | A failed functional suite is told to rerun with `make ze-<suite>-test`, by the FAIL block in `_ze-functional-test-impl` and by `functionalSuiteRerun`. No such target exists for any of the 24 gating suites. The real name is `make ze-functional-<suite>-test`, and even that is absent for ldp, rsvpte and install, which have no individual target at all | fixed on both surfaces. The FAIL block prints `make ze-functional-%s-test`, `functionalSuiteRerun` returns the same name, the cap-expiry group carries it as its `rerun` field, and ldp, rsvpte and install gained the individual targets they lacked. Three tests hold it: `TestFunctionalSuiteRerunNamesARealMakeTarget` (scripts/status/verify_run_test.go) checks the Go producer against the declared make targets for every all_suites member, and `TestEverySuiteCanBeRerun` and `TestCapExpiryTellsTheReaderWhatToRun` (scripts/dev/functional_suite_test.py) do the same for the two makefile producers |
| 2026-08-19 | none (owner-commissioned encode expectation fix) | internal/test/runner/report.go, internal/component/bgp/cli/main.go | The DEBUG block of a functional failure prints `ze bgp decode update <hex>`. Run from a shell it exits 1 with `invalid hex: encoding/hex: invalid byte: U+0055 'U'`, because `decode` takes the message kind as an option and `update` is parsed as the payload. The working form is `ze bgp decode --update <hex>`. The `Examples` list on the `ze bgp` help carries the same wrong form | not fixed |
