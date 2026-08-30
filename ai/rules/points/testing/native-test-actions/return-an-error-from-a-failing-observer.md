---
kind: directive
level: MUST
stage:
---
- **A compiled observer MUST report an assertion failure by RETURNING an error, and MUST NOT print a line and return `nil`.** `fixture.Observe` can still request a clean daemon shutdown, so `expect=exit:code=0` does not prove the observer's assertion and MUST NOT be relied on alone. `fixture.Run` passes the returned error to `fixture.ReportFailure`, which emits the `ZE-OBSERVER-FAIL` sentinel the runner detects.
- **An assertion on a production log line SHOULD be preferred over either**, because it verifies the production code path rather than the observer: `expect=stderr:pattern=<decision log>` plus `reject=stderr:pattern=<wrong outcome>`.
