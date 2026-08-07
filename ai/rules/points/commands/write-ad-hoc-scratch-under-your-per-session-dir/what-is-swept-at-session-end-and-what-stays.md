---
kind: note
level:
stage:
---
The whole directory is removed at session end (`.claude/hooks/session-end-scratch.sh`,
with a 24h backstop in `session-start.sh`), so your scratch is self-contained and
disposable. Do NOT relocate artifacts that are already session-keyed (commit
scripts `tmp/commit-<sid>.sh`, session state `tmp/session/*-<SID>*`) or shared by
design (`tmp/ze-verify.*`, the durable `cache/`) -- those stay put. `GOCACHE` is
`cache/go-cache` (`Makefile`), on the durable side.
