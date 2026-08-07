---
kind: note
level:
stage:
---
This table and `DEFERRAL_TERMINAL_STATUSES` must not drift apart. They did once,
and it cost: the gate tested only `status == "open"` while this rule's own prose
taught the word `deferred`, so rows written correctly per the rule were never
looked at. 23 live rows without a home had accumulated behind that hole.
