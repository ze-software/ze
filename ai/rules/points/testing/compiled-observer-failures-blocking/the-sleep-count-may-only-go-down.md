---
kind: directive
level: MUST
stage:
---
**Sleep ratchet: the total `time.sleep(` count across `test/**/*.ci` MUST NOT
increase.** The committed baseline lives in `test/.ci-sleep-baseline`, and
`./le doc wiring` fails when the count exceeds it. **A payload-predicate wait
MUST be used instead of a sleep**, because a sleep hides a real race:
`fixture.Poll` around `fixture.Dispatch` in a compiled observer, or a
`wait_until` / `dispatch_until` engine step. **A change that removes sleeps MUST
lower the baseline in the same change.**
