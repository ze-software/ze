---
kind: note
level:
stage:
---
Test binaries live in a private `bin/` under the native functional action's
session scratch directory. `.ci` tests execute bare names and need an isolated
`etc/ze` (`internal/le/functional/binaries.go`, `internal/test/sessionpath`).
