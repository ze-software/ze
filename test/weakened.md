| Test | Reason |
|------|--------|
| tmpfs_test | The file-level count of the row below: 86 assertions to 82, all four of them `TestCleanup`'s. No other test in the file changed. |
| TestCleanup | Retired with its subject, `Tmpfs.WriteToTemp`. That method picked its own temp directory, and the runner now creates one working directory per test and writes the declared tmpfs files into it (`Record.WorkDir`, `internal/test/runner/runner_exec.go`), so the method had one caller left, its own test. `WriteTo`, the half that remains, keeps its coverage in `TestWriteTo` and in the runner's `TestRecordWithoutTmpfsStillRunsOutsideTheCheckout`, which asks the child process which directory it ran in. |
