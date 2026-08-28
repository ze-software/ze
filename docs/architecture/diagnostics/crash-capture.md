# Crash Capture

Ze runs on gokrazy appliances where stderr goes nowhere. When a goroutine panics
outside a component-level recovery point, the Go runtime writes the stack trace
to fd 2 and exits. The trace is lost, and so is the in-memory log ring that
holds the pre-crash context.

<!-- source: internal/core/crashlog/crashlog.go -- crash capture entry point -->
<!-- source: internal/core/crashlog/stderr.go -- stderr redirect and syslog forwarding -->
<!-- source: internal/core/crashlog/persist.go -- crash file persistence and rotation -->
<!-- source: internal/core/crashlog/list.go -- crash file listing for the CLI -->

## The decisions

**Redirect fd 2 with dup2. A defer in `main()` cannot do this job.** A panic in
a non-main goroutine is not catchable by any defer in main, so the only way to
capture an arbitrary goroutine panic is to intercept the file descriptor itself.
<!-- source: internal/core/crashlog/dup2_unix.go -- fd 2 redirect on unix -->
<!-- source: internal/core/crashlog/dup2_unsupported.go -- non-unix stub -->

**The build tag is `unix`, not `linux`.** `syscall.Dup2` works on darwin too,
and crash capture must be testable during development.

**Crash files are written on panic detection, not continuously.** A continuous
`current.log` needs a clean-shutdown marker, which conflicts with the many
`os.Exit` paths in `main()`.

**Crash syslog reuses `ze.log.destination`.** A separate `ze.crash.syslog`
setting would let the two diverge.

**The crash directory is autodetected. `ze.crash.dir` is an override, not a
requirement.** The probe order is the explicit override, then the config
directory's `crash`, then `/perm/ze/crash`, `/var/lib/ze/crash` and
`/tmp/ze-crash`. Each candidate must be creatable and writable. A REQUIRED
setting would mean a crash file goes missing whenever nobody set one.

**`HandlePanic` and `os.Exit(2)` live in `cmd/ze/main.go`, not in the library**,
because the native write hook blocks `panic()` and `os.Exit()` outside a main
file.
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

## Constraints

**The in-process pipe reader is best effort.** Go's `_exit(2)` after a panic
terminates every goroutine at once, so the reader can lose the tail of the
trace. Line-by-line syslog forwarding is the reliable capture and runs in real
time.

**All stderr output is now forwarded**, including runtime warnings and a stray
`fmt.Fprintf`, whenever `ze.log.destination` is configured.

**A crash file carries the last 64 log ring entries** for context.

`show crashes` and `show crashes latest` read them after the restart.
<!-- source: internal/plugins/crashes/crashes.go -- offline crash file viewer -->

**`env.Get` caches `os.Environ()` on first access.** A test that calls
`t.Setenv` must call `env.ResetCache()`, or the cache never sees the new value.
