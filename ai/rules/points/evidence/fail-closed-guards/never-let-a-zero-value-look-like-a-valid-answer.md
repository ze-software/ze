---
kind: note
level:
stage:
---
A zero value must never be a valid-looking answer. Where a lookup's miss returns
a zero that downstream reads as a legitimate outcome (allow, match-nothing,
success, count-of-1), the miss is invisible at every later layer.
