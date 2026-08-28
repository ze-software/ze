---
kind: note
level: MUST NOT
stage:
---
A compiled observer MUST NOT report an assertion failure only by printing a
line and then returning `nil`. `fixture.Observe` can still request a clean
daemon shutdown, so the daemon exit code does not prove the observer's
assertion. Return an error from the scenario.
