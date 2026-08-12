---
kind: note
level:
stage:
---
The reason is attribution, not speed and not memory. Suites share the build
cache, the ports and the `bin/ze` processes. A concurrent run therefore makes a
red that belongs to nobody. A killed process and a real defect read the same in
a log.

The repo-wide verify lock says this for one target. This says it for every
suite, and it names who holds the right to run one.
