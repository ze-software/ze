---
kind: note
level:
stage:
---
`deferral_unassigned_problems` (`scripts/dev/commit_helper.py`) checks the
Destination of every row whose Status is NOT terminal. The terminal set is
`DEFERRAL_TERMINAL_STATUSES` in that file:
