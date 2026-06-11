# 879 -- L2TP Control Message Priority and Data P-Bit

## Context

L2TP's reliable engine send queue was pure FIFO. Under heavy session churn (many ICRQs/ICRPs),
tunnel-level control messages (StopCCN for teardown, HELLO for keepalive) would sit behind
session-level messages, delaying tunnel teardown and liveness detection. Separately, PPP LCP
Echo messages used for CQM monitoring flow as L2TP data messages and could benefit from the
RFC 2661 P (Priority) bit, but the kernel's l2tp_ppp module does not expose P-bit control.

## Decisions

- Chose prepend-to-sendQueue over a separate priority queue or bypass path, because at most
  1 StopCCN + 1 HELLO per tunnel lifetime are priority. A separate queue adds complexity
  for near-zero benefit; bypassing would break Ns ordering.
- Chose to exempt priority messages from MaxSendQueueDepth over rejecting them, because a
  StopCCN that cannot be enqueued delays tunnel teardown indefinitely.
- Kept the existing Outstanding()==0 guard for HELLO over removing it. Outstanding messages
  ARE implicit keepalive signals; priority is defense-in-depth.
- CDN (session teardown) is deliberately non-priority to avoid starvation during mass disconnect.
- Documented kernel P-bit limitation over implementing a userspace workaround, because
  intercepting LCP Echo in userspace would duplicate the data path and conflict with kernel state.

## Consequences

- Ns assignment in send() (not at queue insertion) is load-bearing for priority correctness.
  Any refactor that moves Ns assignment earlier would break priority prepend safety.
- Future Enqueue callers are forced to choose priority by the bool parameter; the compiler
  enforces the decision.
- Kernel P-bit support remains a known limitation. A future kernel patch adding an
  L2TP_ATTR_PRIORITY generic netlink attribute would enable ze to set P=1 for CQM echo
  packets without userspace changes.

## Gotchas

- RFC 2661 P bit is data-only; control messages MUST have P=0. The priority mechanism for
  control messages is queue ordering, not a wire flag.
- The prepend uses `append([]pendingSend{ps}, e.sendQueue...)` which allocates a new backing
  array. Acceptable because priority enqueues are rare (once per tunnel lifetime for StopCCN).

## Files

- `internal/component/l2tp/reliable.go` -- Enqueue priority parameter + prepend logic
- `internal/component/l2tp/tunnel_fsm.go` -- StopCCN + HELLO pass priority=true
- `internal/component/l2tp/session_fsm.go` -- session messages pass priority=false
- `internal/component/l2tp/reliable_test.go` -- 4 new priority tests
- `docs/architecture/wire/l2tp.md` -- priority and P-bit documentation
- `rfc/short/rfc2661.md` -- priority queue note
