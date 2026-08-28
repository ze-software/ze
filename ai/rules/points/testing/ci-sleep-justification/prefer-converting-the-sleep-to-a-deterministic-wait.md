---
kind: directive
level: MUST
stage:
---
The sleep MUST be converted to a deterministic wait with `fixture.Poll`,
`fixture.Dispatch`, or an engine-step predicate from the Compiled Observer API
whenever a condition exists to wait on. Only when no such condition exists
does the sleep stay, and then it MUST be justified. See "Try a sync primitive
before you write a sleep" below for what trying means and how the comment records it.
