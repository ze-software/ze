---
kind: directive
level:
stage:
---
**Beside the seven, `check_drain_floor` compares the derived sign-off count against the drain policy in `rfc/drain-budget.txt` (a start date and a rate, and nothing else).** It is a schedule rather than a ratchet, and it ships INERT at rate 0: arming it is a one-line commit only the owner takes.
