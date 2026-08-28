---
kind: note
level:
stage:
---
`fixture.Poll` around `fixture.Dispatch` is the compiled payload-predicate wait.
Prefer it over `time.Sleep` plus a one-shot assertion so the test blocks until
the observed payload matches, within a bounded attempt count.
