---
kind: directive
level:
stage:
---
**Not detected (by design, to avoid false positives):** changing an expected
value in place while the assertion structure stays (e.g. `Equal(t, 1, x)` ->
`Equal(t, 2, x)`). This is the one weakening the hook cannot see; treat it with
the same discipline manually. Adjusting an expected value to match broken code is
the same violation as removing the assertion.
