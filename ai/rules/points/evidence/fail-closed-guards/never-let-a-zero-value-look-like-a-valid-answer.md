---
kind: directive
level: MUST NOT
stage:
---
**A zero value MUST NOT be a valid-looking answer.** Where a lookup's miss returns a zero that a downstream layer reads as a legitimate outcome (allow, match-nothing, success, count-of-1), the miss is invisible at every later layer.
**Beware the guard that works where you spot-check it and not where it matters.** A constraint that visibly rejects on one node shape can be structurally unable to reject on another, and the spot check passes either way.
