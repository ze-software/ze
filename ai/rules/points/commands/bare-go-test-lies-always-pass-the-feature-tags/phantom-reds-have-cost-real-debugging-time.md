---
kind: directive
level:
stage:
---
**This has cost real time.** On 2026-07-15 two `plan/known-failures/` entries
(7 tests) were disproven as pure tags artifacts. Both had been logged with a
confident but wrong root cause (a "macOS socket-stack quirk"; a "broken
listener-conflict validator"), and one was "re-confirmed" six days later by
repeating the same flawed invocation. A phantom red is worse than a real one: it
sends the next session hunting a bug that was never there.
