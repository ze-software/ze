---
kind: note
level:
stage:
---
The directory carries the id, so the file name does not. That is what keeps
argv[0] personality dispatch working (`cmd/ze/dispatch.go` `binarySuffixRoot`
reads the segment after the last `-`) and lets a `.ci` test exec `ze` by bare
name off one PATH entry. A binary's location also decides where `ze` resolves
its config and database (`internal/core/paths/paths.go` `ConfigDirFromBinary`),
so a session's `ze` reads `<session-dir>/etc/ze` and the repository's `etc/ze`
is the human's alone.

The directory is LOOKED UP, never recomputed: every consumer takes the single
directory matching `tmp/session/????-??-??-<id>`, and names a new one with
today's date only on a miss. Recomputing from today's date would move a
session's directory at midnight and orphan the binaries it is running.
