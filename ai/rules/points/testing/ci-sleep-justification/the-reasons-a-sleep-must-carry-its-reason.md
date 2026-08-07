---
kind: directive
level:
stage:
---
1. A blind sleep hides real races. A reader cannot tell whether it is safe (a
   bounded poll interval that already blocks on a real condition) or a guessed
   duration that will flake under load.
2. When a sleep is deliberately left un-converted, the reason (deliberate timer,
   a Linux-only effect verifiable only under QEMU, an effect with no queryable
   readiness signal) is knowledge that must live next to the code, not in a
   reviewer's head.
