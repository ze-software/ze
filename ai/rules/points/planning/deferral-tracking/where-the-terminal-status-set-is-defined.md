---
kind: note
level:
stage:
---
`deferral_unassigned_problems` (`internal/le/commit`) checks the
Destination of every row whose Status is NOT terminal. The terminal set is
`DEFERRAL_TERMINAL_STATUSES` in that file:
