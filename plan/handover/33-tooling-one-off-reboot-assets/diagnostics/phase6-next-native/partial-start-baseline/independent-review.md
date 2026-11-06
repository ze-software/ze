# Controlled partial-start baseline: independent safety review

Verdict: **REFUSE execution** until unprotected startup has a joined abort path.
This is source review only. No formatting, compiler, test, native process,
approval command, or source edit ran.

## P1 — The unprotected abort can return while startup still runs

`proveNativeCatalogPartialHandoff` closes the safety connection/listener on an
unprotected failure, waits twenty seconds for the surrogate, then sets
`controller.native = nil` when that wait misses. That removes the direct-child
join from later cleanup. `TestNativeCatalogPartialParent` likewise declines to
force the helper before the ownership mark, without establishing its join.

The helper still performs its input hashing before `Start` and child-safety
publication. Its test allows forty seconds, while the controller's safety
accept allows ten. Those are distinct allowed intervals. The helper neither
observes safety EOF during those preparation steps nor checks cancellation
before starting its child. Therefore an ordinary late preparation can outlive
the controller's ten-second accept plus twenty-second abort wait. The helper
can then start the resistant child after the safety deadline/EOF, discover the
failed transfer, and only afterward run its deferred child cleanup. Meanwhile
the supervisor has already discarded its surrogate join and can exit.

This source path needs no PID-reuse attack or unkillable kernel task. It leaves
the gate/surrogate and potentially the just-started child executing after an
early failed precondition. Isolating the supervisor as a subreaper does not
join children when that supervisor returns or its test timeout exits it.

The baseline must keep an abort owner throughout preparation and the
fork-to-transfer interval. After abort/EOF it must not create a new child; an
already started child must be settled by its retained owner. Keep the
surrogate's sole Wait attached until that owner has joined, including every
precondition error. Do not clear `controller.native` as an alternative to
settlement, lengthen product bounds, signal a guessed PID/group, or move safety
kills before the oracle. A revised test-only startup guard may repair this
baseline envelope while leaving the old broken `nativeCatalogProbe.finish`
and its intended RED unchanged.

## What is sound in the protected path

- The ordinary refused observer remains unchanged except for the explicitly
  selected test seam and its two-pidfd safety role. The actual broken `finish`
  still runs before all survivor/socket/join-acknowledgement assertions.
- After both safety handles are bound, the independent child observer does
  not grant the child to the actual cleanup controller. Its final safety kill
  occurs after the oracle and cannot turn the old cleanup failure into PASS.
- The supervisor becomes a subreaper before starting fixture processes. Its
  sequential adopted waits use the independently verified process handles,
  after the surrogate's direct Wait, with no wildcard wait or numeric sweep.
- The selected Go 1.27.3 `os/pidfd_linux.go` uses `waitid(P_PIDFD)` and retains
  the handle when it returns `ECHILD`. Thus checking that same handle for
  `ErrProcessDone` before accepting an already-reaped child is meaningful.
  The adopted reaper owns final release; the generic controller does not
  release those same handle objects concurrently.
- The explicit ABI values match retained primary definitions:
  `vendor/golang.org/x/sys/unix/zsysnum_linux_amd64.go` has `SYS_PRCTL=157`,
  `zsysnum_linux_arm64.go` has `167`, and `zerrors_linux.go` has
  `PR_SET_CHILD_SUBREAPER=0x24`. Unsupported architectures fail before children.
- Endpoint resource cleanup covers constructor failures. The separate foreign
  sentinel has its own direct Start/Wait owner. Canonical source still matches
  `before/`, and the ordinary `after/` still matches `refused-v1/`.

No existing test body or RFC-tagged assertion is weakened, so no approval row
is owed. This verdict covers only execution safety of
`TestNativeCatalogPartialHandoffCleanup`. It grants no approval to either old
native root and establishes no P1/P2 repair or runtime discrimination result.

After correcting the abort envelope, review the actual delta again. MAIN may
then run the isolated baseline and require the intended old semantic failure
plus independently completed safety reaping; setup or transfer failures do
not qualify as the RED.
