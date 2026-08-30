---
kind: directive
level: MUST NOT
stage:
---
**A recorded discrimination proof MUST NOT be relied on after a change to what
reaches the component under test.** `ai/rules/interop-and-goal-validation.md`
proves a test could fail on the day it was proven, against the wiring of that
day. A change to what reaches a component moves a green test onto another rail
without touching one assertion, so the test still passes, no gate reddens, and
the recorded proof now describes code the test no longer runs.
