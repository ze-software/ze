---
kind: directive
level: SHOULD
stage:
---
**`fixture.Poll` around `fixture.Dispatch` is the compiled payload-predicate
wait, and it SHOULD be used in place of a `time.Sleep` followed by a one-shot
assertion.** The test then blocks until the observed payload matches, within a
bounded attempt count. The API is `docs/architecture/testing/ci-format.md`.
