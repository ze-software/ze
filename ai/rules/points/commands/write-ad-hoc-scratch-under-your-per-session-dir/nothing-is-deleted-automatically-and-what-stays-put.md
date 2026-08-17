---
kind: note
level:
stage:
---
**Nothing under `tmp/session/` is ever deleted automatically**: not at session
end, not on an age timer, not by a hook. Your directory outlives your session,
so a log you wrote is still there tomorrow. Cleanup is the operator's:
`make ze-session-clean BEFORE=<YYYY-MM-DD>` removes the session directories
dated strictly before that date, and `make clean` removes your own. Do NOT
relocate artifacts that are already session-keyed (commit scripts
`tmp/commit-<sid>.sh`) or shared by design (`tmp/ze-verify.*`, the durable
`cache/`) -- those stay put. `GOCACHE` is `cache/go-cache` (`Makefile`), on the
durable side.
