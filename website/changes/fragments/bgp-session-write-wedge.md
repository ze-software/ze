---
spec: bgp-session-write-wedge
topic: BGP
found-by: review
---

## Before

A BGP peer that stopped reading while it kept its TCP connection open could
hold its session with Ze for good. This is what a router does when its BGP
process hangs but its kernel still answers: the connection stays up and its
receive window stays shut.

When Ze had routes to send to such a peer, the write blocked with no time
limit. Everything that ends a session waited behind that write before closing
the connection: the hold timer, the send hold timer of RFC 9687, an operator
tearing the peer down, a reload that removed it, and the daemon stopping. So
the session never ended. The peer stayed listed as established, its routes
were never withdrawn, and Ze never reconnected to it, even after the peer
recovered. On a route server, one client in that state could also hold up the
forwarding of another client's routes.

## After

Every write to a BGP session now has a time limit, and closing a session no
longer waits for a write the close itself would end.

- A route write that the peer accepts nothing of for the write budget fails,
  and Ze drops the session. The budget is `ze.fwd.write.deadline`, 30 seconds
  by default. A KEEPALIVE, OPEN or NOTIFICATION has a third of the hold time,
  between 10 and 30 seconds.
- When a hold timer or a send hold timer expires while a write is blocked, Ze
  closes the connection within about a second. Any other close waits no longer
  than the control budget above.
- Ze then reconnects by itself and sends its routes again once the session is
  established.

A close made this way sends no NOTIFICATION, because the peer could not have
read one. A healthy peer still receives its NOTIFICATION before the close, as
before, and a slow peer that keeps reading keeps its session.

Two new problems are reported in the log, in `ze show doctor` and in
`ze show errors`, and the peer's last error in `ze show bgp peer` names the
cause:

- `doctor-bgp-peer-not-reading`: the peer accepted no data for the whole write
  budget, so Ze closed the session.
- `doctor-bgp-write-path-held`: Ze had to close a session while a write to it
  was still blocked.

`ze show diagnostic <code>` explains each one and what to check on the peer.

Two limits remain. On a route server, a client that stops reading can still
delay the reading of the peer whose routes are being forwarded to it, for up
to the write budget. And a plugin that stops taking the events Ze sends it can
still keep a peer from reconnecting until that plugin recovers or stops.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
