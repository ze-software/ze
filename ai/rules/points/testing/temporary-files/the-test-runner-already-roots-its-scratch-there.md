---
kind: note
level:
stage:
---
The functional-test runner already writes there: its per-run and per-test working
directories (configs, sockets, daemon pid/ready files) root at
`sessionpath.DefaultScratchRoot()` / `EnsureScratchRoot(baseDir)` when a session is
active, instead of the unowned `$TMPDIR/ze-functional-*` they used before
(`internal/test/sessionpath`, `internal/test/runner/runner.go`). Off-session the
runner still uses the system temp dir, unchanged.
