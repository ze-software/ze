---
kind: note
level:
stage:
---
This is the invariant the gate is built on: it re-checks every live row's
destination on every commit, so "outstanding work names a real spec" is surfaced
continuously (as a warning), for as long as the work is outstanding. Closing a row
early to quiet the warning hides the work from the only thing watching it.
