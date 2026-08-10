---
kind: directive
level: MAY
stage:
---
**Beside the seven, `check_drain_floor` compares the derived sign-off count against the drain policy in `rfc/drain-budget.txt` (a start date and a rate, and nothing else).** It is a schedule rather than a ratchet, and it ships INERT at rate 0: only the owner MAY arm it, with a one-line commit.
