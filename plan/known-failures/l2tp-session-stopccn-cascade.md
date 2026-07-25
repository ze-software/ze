### `l2tp` functional suite `session-stopccn-cascade` -- RESOLVED 2026-07-25

**This shard is resolved and should be deleted in the commit that lands the
fix.** It is kept here only so the correction to its original diagnosis is
reviewable.

#### The original root cause was wrong

The earlier entry claimed: "ze's receive window does not advance past the second
session's rapid-fire ICRQ, so the peer's later StopCCN (a higher Ns) is never
delivered to `handleStopCCN`". That is refuted by the producing code:

- `OnReceive` classifies purely on `hdr.Ns == e.nextRecvSeq` and advances
  `nextRecvSeq` on every in-order delivery (`reliable.go:487-499`). There is no
  window gating on the receive path at all -- `e.win` governs only the SEND path
  (`Enqueue` `reliable.go:365`, `drainSendQueue` `reliable.go:429`). The only
  receive-side bound is the reorder queue's capacity (`reliable_reorder.go:55`),
  consulted solely for out-of-order Ns.
- The `.ci` peer sends strictly increasing Ns 0..6, so every message takes the
  in-order branch and the reorder queue is never used.

#### The actual cause: no CAP_NET_ADMIN on Linux

The asserted log is emitted only when the tunnel still holds sessions --
`if len(cleared) > 0` at `tunnel_fsm.go:596`. On a Linux host where
`resolveGenlFamily` succeeds (`kernel_linux.go:498-505`) but the process lacks
CAP_NET_ADMIN, the sessions are gone before StopCCN arrives:

`handleICCN` sets `kernelSetupNeeded` (`session_fsm.go:180`) ->
`collectKernelEventsLocked` emits a setup event (`reactor_kernel.go:33-61`) ->
the genl create fails EPERM -> `handleKernelError` (`reactor_kernel.go:388-411`)
-> `teardownSession` (`session_fsm.go:434-455`) -> `removeSession` deletes the
session (`session.go:249`). `clearSessions` then returns nil and the log is
suppressed.

So `option=needs-linux:caps=net-admin` (added in `6c27ebd37`) is a correct
marking, and the test is no longer an unexplained red.

#### Evidence the control path itself is sound

On darwin -- and on any Linux where the genl resolve fails -- the kernel worker
is nil (`kernel_other.go:37-39`, `reactor_kernel.go:26-28`), no kernel error is
ever raised, the sessions survive, and the cascade works. Measured at HEAD:

| Harness | Result |
|---|---|
| Daemon + the `.ci`'s own python peer, 5 runs | 5/5 pass, `StopCCN clearing sessions` present |
| Same, peer launched with no readiness fence, 3 runs | 3/3 pass |
| The real `ze-test` runner, `needs-linux` line removed from a scratch copy | 1/1 pass |
| Same runner, `.ci` body as of `fe6aa242f` | 1/1 pass |

#### Coverage after the fix

`TestSession_StopCCNCascadeThroughEngine`
(`internal/component/l2tp/session_stopccn_cascade_test.go`) proves AC-9 on every
platform in the always-run unit suite under `-race`: it drives the same
SCCRQ/SCCCN/ICRQ/ICCN x2/StopCCN sequence over real UDP through the reactor and
reliable engine and asserts `count=2` plus zero surviving sessions. That is a
stronger assertion than the `.ci`'s stderr substring, so gating the `.ci` to
QEMU costs no coverage.
