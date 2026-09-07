# Rewrite drops a test nobody misses

A surface is ported from one implementation to another and the port is judged
by whether the new implementation WORKS. Its tests are not part of that
comparison, so a test that existed only in the old implementation vanishes with
it. Nothing goes red, because the assertion that would have gone red is the
thing that was deleted. The behavior it pinned is then unguarded, and the next
change to it is silent.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | fixit-plugin-concurrency-is-pinned-to-a-ci-constant | the make-to-`le` port of the functional test runner, `eae282592` | `test_serial_suites_stay_serial` existed in the retired `scripts/le/functional_test.py` and has no counterpart in the Go runner that replaced it. It pinned that the derived `-p` must never reach the `reload`, `managed` and `vpp` suites, which must stay serial. For the window since the port, a change to the concurrency derivation could have made all three parallel with nothing going red, and those are precisely the suites whose tests contend for one daemon, one managed server and one dataplane. Found while implementing the spec that owns the derivation, not by any gate: the port's own review compared what the new runner DOES against the old one, and a test is not a behavior the new implementation has to reproduce to look correct | FIXED here in `24c773bc57`: `TestSerialSuitesStaySerial` (`internal/le/functional/functional_test.go`), recorded RED under a deliberate flip of `reload` to `Scaled` (`suite reload ends with [-p 32], want [-p 1]`) and green after reverting. The transferable point is about the PORT rather than these three suites: a rewrite's acceptance criterion is behavior parity, and test parity is a separate question nobody asks. What would end the class is asking it once per port -- enumerate the retired implementation's test names and account for each -- since the old file is still in git history and the enumeration is cheap. Whether the same port dropped OTHER assertions is unmeasured and is not this row's to answer |
