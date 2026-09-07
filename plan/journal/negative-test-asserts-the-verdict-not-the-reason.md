# Negative test asserts the verdict, not the reason

A test that proves something FAILS is only as strong as the failure it names. A
non-zero exit, a nil-error check, a "returns an error" assertion: each one is
satisfied by every other way the same code can break, so the test survives a
build in which the refusal it exists to prove no longer happens. The verdict is
free; the reason is the evidence. Assert the message, the code, or the state the
refusal produces, and prove the assertion discriminates by breaking the producer
and reading which line went red.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | fixit-ci-runner-cannot-test-stdin | `test/ui/config-history-stdin-refused.ci` over the `cliio.IsStdin` guard in `cmdHistory` (`internal/component/config/cli/cmd_history.go`) | The fixture asserts `expect=exit:code=2` and `expect=stderr:contains=history needs on-disk revision history`. Measured with the guard cut so history opens the substituted path instead: the run still exits 2, because `NewEditorWithStorage` cannot open the path either and `main.go` answers `exitError` for that too. The exit assertion PASSED under the cut. Only the stderr line discriminated, and had the fixture been written with the exit code alone it would have been green against a build where the refusal branch does not exist | the fixture keeps both assertions and its header records which one carries the proof, so a later author cannot drop the stderr line as redundant. The same walk is now stored for the runner itself: `internal/test/runner/testdata/mustfail/*.ci` each name their expected failure on a `# must-fail:` line and `TestCIMustFailFixturesAllFail` (`internal/test/runner/must_fail_test.go`) compares that text against `rec.Error`, so a fixture that goes red for a NEW reason fails the gate exactly as loudly as one that goes green |
