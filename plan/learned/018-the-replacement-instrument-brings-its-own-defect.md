# Learned: the replacement instrument brings its own defect

`plan/learned/017-measure-the-instrument-before-you-trust-the-green.md` ends with
a mechanism named but not built: a way to hold one socket in a failing state
without ptrace, because ptrace's own cost destroyed the CPU reading the test
exists to make. This is what happened when that mechanism was built.

## The mechanism

A classic seccomp BPF filter answering `SECCOMP_RET_ERRNO | ENETDOWN` for one
named syscall, installed by a launcher that then `execve`s the daemon
(`ze-test fail-syscall`, `internal/test/failsyscall`). The kernel refuses the
call at its entry, there is no tracer, and the refused call is charged to the
daemon's own CPU. Measured in the QEMU guest on ze's runtime kernel, both builds
in one boot over the same 3s window:

| Build | Read errors | CPU |
|---|---|---|
| paced | 12 | 0.0133 of one core |
| pacer reverted | 1,053,776 | 2.1067 of one core |

ptrace had put the same two builds within one order of magnitude of each other.

Two facts decided the shape, and both had to be read out of the kernel rather
than assumed. `CONFIG_FAULT_INJECTION` and `CONFIG_KPROBES` are off in ze's
runtime kernel, so `fail_function` and `/proc/<pid>/make-it-fail` do not exist
there. `CONFIG_SECCOMP` and `CONFIG_SECCOMP_FILTER` are on, and they arrive from
the base defconfig rather than from a `runtime.config` request, so they are now
pinned in `gokrazy/kernel/runtime.require`.

## The lesson: the new instrument owes the same walk as the old one

The seccomp launcher fixed the CPU reading and broke the LOG reading, in a way
that produced no error anywhere.

Every `cmd/ze` binary calls `crashlog.Init` (`internal/core/crashlog/crashlog.go`),
which dup2s a pipe over fd 2 and drains it from a goroutine. An `execve` replaces
the image that held the goroutine while fd 2 survives into the new program, so the
launched daemon writes its whole log into a pipe nobody will ever read, and blocks
once 64 KiB have accumulated in it. Measured: the daemon served `/metrics`, counted
its read errors, answered every poll, and wrote not one line.

Both failures share one shape. The instrument sits between the daemon and the
observation, and it is the observation it degrades, never the daemon. Nothing goes
red at the instrument. The test simply reads a smaller number, or no number, and a
threshold written for the real effect cannot tell that apart from a fix.

**So the discrimination walk in `ai/rules/interop-and-goal-validation.md` is owed
by the REPLACEMENT instrument as much as by the test.** Running the reverted build
through it is the only step that distinguishes "the fix works" from "the instrument
is reading nothing".

## The assertion that looks redundant is the one that caught it

The scenario asserts three things: the CPU fraction, the error counter, and one
log line. The first two come off `/metrics`; the third comes off the daemon's
stderr. On the day it mattered, the two `/metrics` assertions passed and the log
assertion failed.

Had the scenario asserted only what the fixture measures, the launcher would have
shipped handing every future caller a daemon whose log vanishes and whose stderr
deadlocks past 64 KiB. The log line is not a third copy of the same claim: it
reads a different channel, and a channel is what broke.

The `.ci` header now says so in the file, because the next reader's instinct is
to delete the line that duplicates nothing.

## The unfixed siblings

Four other `execve` sites carry the same defect, one of them shipped product
code: `defaultRestart` (`internal/component/config/system/selfupdate.go`) re-execs
`ze` after a self-update, so a self-updated daemon loses its log and deadlocks on
stderr. They are recorded in
`plan/journal/output-lost-to-an-exit-past-the-flush.md`, which already held a row
for the same class through `os.Exit` rather than `execve`. The source repair both
rows name is one decision: `crashlog` owning the leave, so that every exit AND
every `execve` drains the pipe.

## A gate that cannot see an untracked package

`./le verify lint run` selects its packages from TRACKED files, so a brand-new
package is linted by nothing until its first commit. Running `golangci-lint`
directly over `./internal/test/failsyscall/...` reported five findings the gate
had not seen, including one `unused` const that only appears off Linux. Lint a
new package by hand before it is first committed; the gate will pick it up
afterwards.
