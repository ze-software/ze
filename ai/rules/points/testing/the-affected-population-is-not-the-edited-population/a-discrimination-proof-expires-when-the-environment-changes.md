---
kind: note
level:
stage:
---
A discrimination proof expires when the environment changes. `ai/rules/interop-and-goal-validation.md` requires you to revert a change once and watch the test go red, which proves that test could fail on the day it was proven, against the wiring of that day. This point is about the proof EXPIRING. A change to what reaches a component moves a green test onto another rail without touching one assertion, so the test still passes, no gate reddens, and the recorded proof now describes code the test no longer runs.
