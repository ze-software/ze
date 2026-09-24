# Crash Capture

Ze runs on gokrazy appliances where stderr goes nowhere. When a goroutine panics
outside a component-level recovery point, the Go runtime writes the stack trace
to fd 2 and exits. The trace is lost, and so is the in-memory log ring that
holds the pre-crash context.

<!-- source: internal/core/crashlog/crashlog.go -- crash capture entry point -->
<!-- source: internal/core/crashlog/stderr.go -- stderr redirect and syslog forwarding -->
<!-- source: internal/core/crashlog/persist.go -- crash file persistence and rotation -->
<!-- source: internal/core/crashlog/list.go -- crash file listing for the CLI -->
<!-- source: internal/core/crashlog/crashout.go -- the runtime's own crash file, and the harvest on the next start -->
<!-- source: internal/core/crashlog/stderr_queue.go -- the relay queue that never blocks a writer -->

## The decisions

**Descriptor 2 stays the real stderr, and the runtime gets a crash file of its
own through `debug.SetCrashOutput`.** The Go runtime prints an unrecovered
panic, a fatal error and a SIGQUIT goroutine dump only after it has frozen every
goroutine, and it writes them with a raw write to descriptor 2. No goroutine of
Ze's runs at that moment, so no in-process reader can capture them. Until
2026-09-24, Init dup2'd a pipe onto descriptor 2. Its only reader was a goroutine,
so the reader was frozen with the rest. A panic trace went into the pipe and
nowhere else, and a dump larger than the 64 KiB pipe buffer blocked for ever:
`kill -QUIT` hung the daemon it was sent to diagnose. Now the runtime writes the
dump to the real stderr and writes a copy to the crash output file.
<!-- source: internal/core/crashlog/stderr.go -- redirectStderr -->

**The crash output file is armed at start and harvested by the next start.**
The file cannot be created when the crash happens, so Init creates it in
`<crash dir>/.pending/`, locks it with `flock`, writes a header, and gives it to
the runtime. The kernel drops the lock when the process exits, however it exits.
The next Init takes each pending file whose lock is free. A file with output
after the `=== Runtime Output ===` marker becomes a `crash-*.log` and joins the
rotation. A file with only the header came from a clean exit and is removed. A
locked file belongs to a running process and is left alone. The header cannot
carry the release version, because Init runs before the version is stamped. It
carries the start time, the commit, the Go version, the PID and the command.
<!-- source: internal/core/crashlog/crashout.go -- armCrashOutput, harvestPendingCrashes -->
<!-- source: internal/core/crashlog/lock_unix.go -- flock on unix -->
<!-- source: internal/core/crashlog/lock_unsupported.go -- no lock, so nothing is armed or harvested -->

**`os.Stderr` goes through a relay that never blocks a writer.** Go code that
writes `os.Stderr` writes into a pipe. A pump goroutine moves the pipe into a
queue of at most 4 MiB, and a relay goroutine copies the queue to the real
stderr and to syslog at the pace they allow. A stalled syslog socket or a slow
serial console therefore never stops a writer. Past 4 MiB the queue drops bytes
and writes `crashlog: N bytes of stderr dropped` into the stream where they went
missing.
<!-- source: internal/core/crashlog/stderr_queue.go -- stderrQueue -->

**The build tag is `unix`, not `linux`.** `flock` works on darwin too, and crash
capture must be testable during development.

**An execve goes through `crashlog.Exec`, which drains the relay before it
replaces the image.** The new image keeps the real descriptor 2, but the relay
goroutines do not survive an execve, so the lines still queued would be lost.
`Exec` calls `Flush` first. A raw `syscall.Exec` or `unix.Exec` outside the
package is refused by the `crashlogExec` rule in `.golangci/ruleguard/modern.go`.
<!-- source: internal/core/crashlog/exec_unix.go -- the leave that drains the pipe -->
<!-- source: .golangci/ruleguard/modern.go -- crashlogExec, the rule that refuses a raw execve -->

**A panic trace that Go code writes to `os.Stderr` is still detected by the
relay**, for a framework that recovers a panic and prints it, and becomes a crash
file when the relay reaches the end of its input. **Crash files are written on
panic detection, not continuously.** A continuous
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

**The runtime's fatal output does not reach syslog.** It goes to the real
stderr and to the crash output file, because nothing in the process runs while it
is written. A crash file from the runtime carries no log ring for the same
reason. `HandlePanic`, which runs in a recover, still writes both.

**Everything Go code writes to `os.Stderr` is forwarded** to syslog whenever
`ze.log.destination` is configured, a stray `fmt.Fprintf` included. A raw write
to descriptor 2 is not, because descriptor 2 is the real stderr.

**A crash file carries the last 64 log ring entries** for context.

`show crashes` and `show crashes latest` read them after the restart.
<!-- source: internal/plugins/crashes/crashes.go -- offline crash file viewer -->

**`env.Get` caches `os.Environ()` on first access.** A test that calls
`t.Setenv` must call `env.ResetCache()`, or the cache never sees the new value.

## The kernel half

Everything above captures a Go panic, which is a fault Ze survives long enough to
write about. A KERNEL panic is the uncovered case: the running kernel is gone, no
process of Ze's is left, root is read-only SquashFS, and the image holds no
busybox to run a post-mortem from.

<!-- source: internal/core/crashlog/kernel.go -- harvest, readiness, the artifact name -->
<!-- source: internal/core/crashlog/kernel_linux.go -- the pstore reader and the machine probes -->
<!-- source: internal/core/crashlog/kernel_other.go -- the non-Linux stubs -->
<!-- source: internal/plugins/crashes/readiness.go -- the one readiness answer every surface reads -->
<!-- source: internal/plugins/crashes/doctor.go -- the three registered checks -->

### The decisions

**The kernel writes the record, and Ze only reads it.** The expensive half of a
crash-dump feature is not the dump, it is the code that runs after the machine
has already failed. With pstore's ramoops backend there is none: the kernel puts
the oops or panic log into a reserved memory region that a warm reboot does not
clear, and Ze reads it on the NEXT healthy boot. No Ze code runs at the moment of
the fault, so no Ze defect can cost the reboot as well as the record.

**A kernel record is a second ARTIFACT KIND, not a second system.** It resolves
through the same directory probe, rotates under the same `ze.crash.keep` count,
is listed by the same `show crashes`, and travels in the same `crashes` support
module. `show kdump` and `show crash` were not added beside it: one concept
carries one name.

**The kind is derived from the artifact NAME, and the name is a suffix.** A
kernel artifact is `crash-<timestamp>-<record>-kernel.log`. A prefix would have
been easier to read and would have broken the retention pass, which sorts by name
and relies on that order being chronological: every Go panic report would be
deleted before the first kernel one.

**The artifact name is derived from the record, so a harvest is idempotent.** The
record's own time and identity produce the file name, so a record the source
refused to clear is rewritten to the same file on the next boot rather than
appearing twice under two timestamps.

**The source is cleared only after the write succeeds.** Clearing on read loses
the only copy of what the kernel said whenever the crash directory is not yet
writable, which is exactly the early-boot case the harvest runs in. A failed
write leaves the record for the next boot.

**Configured and armed are computed from different sources, and neither is
derived from the other.** Configured is the committed leaf. Armed is what the
RUNNING kernel booted with, read back off `/proc/cmdline` and checked against a
readable pstore. A reservation is a boot argument, so a commit looks like it took
effect and changes nothing: reporting one field would hide the gap this feature
exists to close.

**Both cmdline tokens are required, and they must name one region.**
`reserve_mem=<N>M:4096:zecrash` carves the region and `ramoops.mem_name=zecrash`
binds ramoops to it. A region nothing binds to is RAM taken from the operator for
no record; a binding with no region has nowhere to write. The region name is
declared once, as `crashlog.ReserveRegionName`, and the build and the readiness
probe both read it from there.

**The region is named, never addressed.** Size-named reservation has been in the
kernel since 6.12, so the per-machine physical address that used to make ramoops
awkward on x86 is not needed, and the tokens read like the hugepage tokens beside
them at the same assembly seam. `enforceReserveMemKernelFloor` fails the runtime
kernel build below that version rather than reserving nothing in silence.

**pstore is mounted by the reader, not assumed.** gokrazy mounts sysfs and
nothing else, so the mount point exists and the filesystem does not.
`pstoreSource.Available` mounts it, and reports what is missing when it cannot,
rather than answering with an empty record set a caller cannot tell from a
machine that never faulted.

**Every entry in the store is harvested, not only the dmesg one.** A record left
behind holds part of the reserved region, so the next fault has less room to write
into.

### Constraints

**The record is kernel-authored, and its length is not trusted.** The reader
copies at most 1 MiB per record through an `io.LimitReader` and at most 64
records per harvest, and it never sizes an allocation from a field in the file.
The record's file name is data too: it is checked for a separator or a parent
reference before it reaches a path.

**The context block is the HARVEST boot's log ring, and the heading says so.**
The ring of the crashed boot died with the kernel, so a heading claiming
pre-crash context would be false.

**A watchdog reboot produces no artifact.** A wedged box that the watchdog
reboots wrote no kernel record, so the harvest finds an empty store and reports
no reason. If a watchdog reboot produced an artifact, every wedge would read as a
kernel fault.

**Three runtime dependencies, three doctor checks, three diagnostic codes.** The
reservation on the running kernel, a readable record store, and a writable crash
directory each fail silently, and the appliance has no shell to diagnose any of
them with: `doctor-crash-capture-unarmed`, `doctor-crash-capture-pstore` and
`doctor-crash-directory-unwritable`.

**The full memory image is a separate, opt-in half and is amd64-only.** gokrazy's
reboot path kexecs on amd64 alone, so `system crash-dump memory-image enabled
true` is refused at commit on any other architecture rather than committing and
capturing nothing. Where it is allowed, readiness reports it unarmed with the
shortfall in bytes until the target has room for an image sized to its RAM.
