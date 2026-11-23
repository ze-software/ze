---
spec: finish-l2tp
topic: L2TP
feature: l2tpv2-bng
found-by: interop
---

## Before

When an L2TP control ACK opened the transmit window and released an earlier
queued control message while the same received message also generated a fresh
reply, Ze sent the fresh reply with the higher sequence number ahead of the
earlier queued message. Multi-session peers whose receive queue inspects only
the next expected sequence number stalled waiting for the predecessor.

Under concurrent L2TP subscriber bring-up on Linux, transient kernel lock
contention (`EBUSY`) while binding a fresh `/dev/ppp` channel to a PPPoX socket
failed channel setup on the first attempt and dropped subscriber sessions. A
closed PPPoX socket could also surface an internal Go error instead of a closed
file error.

## After

Ze now transmits earlier queued L2TP control messages released by an incoming
ACK before fresh replies produced by the same received message, preserving
assigned sequence order on the wire.

Linux PPP channel capture now retries pre-bind `EBUSY` contention up to four
times on the original socket and fresh `/dev/ppp` file without falling back to
legacy ioctls, and wraps closed-socket lease failures with `os.ErrClosed`. Both
fixes are proven in a 48-session multi-LAC L2TP interop scenario alongside
full-session `FF 03` PPP framing verification.

## Corrects

none

## Exposure

L2TP LNS deployments terminating many concurrent sessions per tunnel, where
send-window queueing or simultaneous PPP channel creation occurs.

## Upstream

none

## Asked by

none
