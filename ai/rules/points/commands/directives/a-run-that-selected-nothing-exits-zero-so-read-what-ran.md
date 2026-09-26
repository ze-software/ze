---
kind: directive
level: MUST
stage:
rationale: ai/rationale/commands.md
---
**A test run's exit code MUST NOT be read as evidence that the test ran: the log MUST be read for the test's own name.** A runner that does not recognize its suite, its verb or its selector prints usage and exits 0, so a mistyped invocation is indistinguishable from a pass. The tell is the usage text where a per-test line belongs, and the check is a `VERIFY STEP` or a `--- PASS` line naming the test.
**A functional test whose fixture is compiled into `le test` MUST have `le test` rebuilt before the run, not `ze` alone.** `le test fixture <name>` resolves the name in the binary's own registry (`internal/test/fixture`, `Register`), so a fixture added in this session is absent from a stale binary while the daemon under test is current, and the run fails for a reason that has nothing to do with the change.
