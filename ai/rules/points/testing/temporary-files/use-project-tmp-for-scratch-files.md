---
kind: note
level:
stage:
---
Use project `tmp/` (gitignored) for scratch files, never `/tmp`.
A subfolder per debugging task (`tmp/watchdog-debug/`) isolates artifacts from each
other, but not from a sibling session: put it under your session's own directory
(below), unless the artifact must outlive the session.
