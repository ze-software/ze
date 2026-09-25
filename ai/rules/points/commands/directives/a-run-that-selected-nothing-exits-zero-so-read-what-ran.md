---
kind: directive
level: MUST
stage:
---
**A test run's EXIT CODE MUST NOT be read as evidence that the test ran: the log MUST be read for the test's own name.** A runner that does not recognise its suite, its verb or its selector prints usage and exits 0, so a mistyped invocation is indistinguishable from a pass. Measured on 2026-09-08: `./le test stress-repro run suite "bgp ui --draft" test bgp-update-delay-command` answered "not reproduced", which reads as green, and had run nothing at all, because the ui suite is `./le test harness ui` and takes no `bgp` verb (`internal/le/test/functional/suites.go`, `Suites`). The tell is the usage text where a per-test line belongs, and the check is a `VERIFY STEP` or a `--- PASS` line naming the test.
**A functional test whose fixture is COMPILED into `le-test` MUST have `le-test` rebuilt before the run, not `ze` alone.** `le-test fixture <name>` resolves the name in the binary's own registry (`internal/test/fixture`, `Register`), so a fixture added in this session is absent from a stale binary while the daemon under test is current, and the run fails for a reason that has nothing to do with the change.
