# 726 -- diag-crash-capture

## Context

Ze runs on gokrazy appliances where stderr goes nowhere. When a goroutine panics outside the existing component-level recovery points (BGP reactor, plugin delivery, API handlers), the Go runtime writes the stack trace to fd 2 and exits. The trace is lost, making post-mortem diagnosis impossible. The in-memory log ring buffer (512 entries of pre-crash context) is also lost.

## Decisions

- Chose stderr pipe redirect via dup2 over defer-only approach, because Go panics in non-main goroutines cannot be caught by a defer in main(). The only way to capture arbitrary goroutine panics is to intercept fd 2 itself.
- Chose dup2_unix.go build tag (`//go:build unix`) over linux-only, because syscall.Dup2 works on darwin too and crash capture must be testable during development.
- Chose panic-detection-based crash files over continuous current.log writing, to avoid needing a CleanShutdown mechanism (which conflicts with main()'s many os.Exit paths).
- Chose to reuse ze.log.destination for crash syslog over a separate ze.crash.syslog config, to avoid configuration divergence.
- Chose crash dir autodetection (probe /perm/ze/crash, config-dir/crash, /var/lib/ze/crash, /tmp/ze-crash) over a required env var, to ensure crash files are always written without operator configuration.
- Put HandlePanic + os.Exit(2) in cmd/ze/main.go rather than the library, because project hooks block panic() and os.Exit() in non-main.go files.

## Consequences

- All stderr output (panics, runtime warnings, stray fmt.Fprintf) is now forwarded to syslog when ze.log.destination is configured.
- Crash files include ring buffer context (last 64 log entries) for diagnosis.
- The pipe reader's crash file write for non-main goroutine panics is best-effort (races against process exit). Syslog forwarding is the reliable primary capture (line-by-line, real-time).
- `show crashes` / `show crashes latest` CLI commands are available for post-restart inspection.

## Gotchas

- Go's _exit(2) after panic terminates all goroutines instantly. An in-process pipe reader cannot reliably capture the full panic trace. Syslog line-by-line forwarding mitigates this.
- gocritic exitAfterDefer fires when a defer exists in a function with os.Exit calls. The defer must be structured carefully to avoid lint issues with existing os.Exit paths in main().
- env.Get uses a cache built from os.Environ() on first access. Tests must call env.ResetCache() after t.Setenv to ensure the cache sees the new value.

## Files

- `internal/core/crashlog/` (new package: crashlog.go, stderr.go, persist.go, list.go, dup2_unix.go, dup2_unsupported.go)
- `internal/plugins/crashes/cmd/show.go` (new: show crashes RPC handlers)
- `cmd/ze/main.go` (modified: crashlog.Init() wiring)
- `docs/guide/operations.md`, `docs/guide/command-reference.md`, `docs/features.md`, `docs/comparison.md`
