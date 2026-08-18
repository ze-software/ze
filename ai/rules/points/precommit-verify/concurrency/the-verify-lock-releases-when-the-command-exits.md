---
kind: note
level:
stage:
---
Lock releases when the command exits. `flock` is fd-backed, not
PID-backed -- no cleanup after a crash.
