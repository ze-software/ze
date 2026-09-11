# Learned: measure the instrument before you trust the green

The end-to-end proof for this spec needed a socket held in a persistently
failing state while the daemon's own CPU use was read off `/metrics`. The only
mechanism that sustains such a failure is ptrace syscall fault injection, and it
is also what destroyed the reading.

Go's runtime sends itself a continuous SIGURG stream to preempt goroutines.
ptrace traps on every signal delivery, and `--seccomp-bpf` does not help: that
flag elides the trap for syscalls outside the trace set, never for signals. Each
trap is a round trip into the tracer, and time spent ptrace-stopped counts as
wall clock rather than as the tracee's CPU (`getrusage`). The tracer therefore
throttles the traced daemon to the same order of magnitude the fix itself
produces.

The numbers are the point. The SAME pre-fix build, driven BY HAND under the same
strace wrapper, reached `ze_l2tp_listener_read_errors_total 36640` and
`process_cpu_seconds_total 0.99` in under four seconds: a real, reproduced busy
loop. Driven through the `.ci` harness it reached 19 errors and no measurable CPU
delta, and passed every assertion in the file. No threshold separates the two
builds, because the instrument's own cost dominates over exactly the range where
they would differ.

**The test that catches this: run the instrument against a build you have
deliberately broken, and compare the ratio of the instrument's own cost to the
size of the effect.** A green from an instrument that has never been shown to go
red carries no information, and `ai/rules/interop-and-goal-validation.md` already
requires that walk. What this spec adds is that the walk can FAIL on the
instrument rather than on the test, and that outcome looks exactly like a passing
test until you run the reverted build through it.

## A vacuous scenario is worth keeping, where it cannot be mistaken for coverage

The scenario was kept rather than deleted, because the mechanism, the three
mechanical problems solved along the way, and the fourth one that cannot be
solved are what the next attempt needs, and a spec section cannot carry a
runnable file.

It is kept where no gate reads it. `test/draft/` already had that property and
already had one tracked exception, the `gr-vacuity-*.ci` exhibit, so this is the
second use of a route that exists: a `.gitignore` negation puts the file in git
while all four recursive `.ci` readers keep skipping the directory.

**The alternative was the trap.** A tracked `.ci` under `test/l2tp/` would be an
accidental orphan, because `netnsSelections` (`internal/le/qemu/netns_linux.go`)
is an explicit name list and `validateNetnsSelection` refuses a named test with
no file while never noticing a file nobody named
(`plan/journal/gate-excludes-part-of-its-population.md`). Worse than not running,
it would have been a file that passes against the BROKEN build sitting in a
gated directory.

**A deliberate orphan and an accidental one differ only in what the file SAYS.**
All three files state, in their own headers, that no gate reads them, why, and
what would have to change for them to be gated. That last clause is what makes
it a decision rather than rot: the mechanism is named, and
spec-failing-socket-proof-needs-a-non-ptrace-injection-point owned it.

**It answered on 2026-09-11, and the orphan stopped being one.** `ze-test
fail-syscall` (`internal/test/failsyscall`) fails a named syscall through a
classic seccomp filter and then execs the daemon, so no tracer is in the path.
The failing-socket scenario moved to `test/l2tp/subscriber-reader-failing-socket.ci`
and the l2tp suite now gates it, leaving `gr-vacuity-*.ci` as the one tracked
exception under `test/draft/`. What that build cost is
`plan/learned/018-the-replacement-instrument-brings-its-own-defect.md`.

## A cross-compiled test is not a test that ran

Three `//go:build linux` tests were written, cross-compiled, type-checked and
handed on as evidence for AC-1 on two of the four loops. The handoff said plainly
that they had never executed, and the reason given was that the host is darwin.

The reason was wrong, and the closure found it by trying. `./le qemu run` boots a
Linux guest on this darwin host and runs whatever you hand it; the "qemu guest
evidence requires Linux" refusal belongs to the guest-only `pppoe-test` and
`netns-test` verbs. One cross-compiled test binary and one guest run turned three
unproven tests into three passing ones on kernel 7.2, and the discrimination walk
in the guest produced the red that made them evidence:
`ze_ppp_reader_errors_total{loop="dhcpv6"} 335243` in a 200ms window with the
pacing removed, against a bound of 50 with it.

**Before writing "cannot be run on this host", run the thing that would refuse
and read what it refuses.** A refusal that belongs to one verb reads, in a
handoff, as a property of the machine.

## Where the pacer landed, and why it is one package

Four receiver goroutines reached the same wrong default at four different times,
which is what a missing shared helper looks like from outside. Three private
backoff implementations already existed (`ddos/flowspec/probe.go`,
`flowexport/conntrack_worker.go`, `exabgp/bridgerun/respawn.go`) and none was
reusable: one is domain policy, one is a constant `time.Sleep` that ignores every
stop signal, and one says in its own header that it has no backoff.

The import direction decided the home before the design started.
`internal/component/l2tp` imports `l2tp/ppp` and `l2tp/pppoe`, so no
component-level package reaches all four call sites and `internal/core/pacer` is
the only home. That is a constraint to read, not a decision to take, and reading
it first is what kept the answer to one package and one type.

`Wait` takes `<-chan struct{}` rather than a `context.Context`, which is why the
same call serves a listener holding a plain stop channel and three loops holding
a context: `ctx.Done()` already has that type. The shape that would have forced a
context on the listener, or an adapter on the other three, was avoided by taking
the narrower type.
