---
kind: note
level:
stage:
---
Some failures only surface under the scheduling and GC pressure of the full
~22-suite run (many concurrent `ze` daemons on all cores). Rerunning the single
suite never triggers them, and looping the full suite to hunt the bug is
impractical (minutes per run, low hit rate). The verify aggregator also
truncates the crashing daemon's goroutine stack to ~2 lines, so the crash site
is usually lost.
