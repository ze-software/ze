---
spec: eor-treats-route-processes-as-peers
topic: BGP
---

## Before

A process or an ExaBGP script that asked for an End-of-RIB (`update text nlri
eor`, `mp <family> eor`, `announce eor`) while the peer's session was still
sending its initial routes had the request dropped, and the answer said the
marker was sent. On a peer with `behavior { manual-eor true; }`, which never
gets Ze's own marker, that peer then received no End-of-RIB at all for the
session. The same peer did receive the End-of-RIB a route server or route
reflector sends when its replay ends, although the setting asks Ze to send no
marker of its own.

A panic inside the initial route send was recovered, but the session stayed up
with routes still queued that nothing would send, and every later marker for
that session was treated as redundant.

`request quiesce` could report the daemon settled while a route server's or
reflector's replay to a peer, or a requested End-of-RIB, was still waiting to be
written, and its timeout named only the subsystem.

<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- announceEOR -->

## After

A requested End-of-RIB is never dropped. It is written after the routes already
queued for that peer, and the answer counts a marker that is written. With
`manual-eor true`, Ze sends no automatic End-of-RIB to the peer, neither the
initial send's nor a replay's, and every marker a process or the operator asks
for is written.

A panic during the initial route send now ends the session with a Cease
(subcode 0, Unspecific). The routes still queued are sent when the next session
comes up.

`request quiesce` also waits while a replay to a peer has not reported done,
while a policy change is re-sending routes, and while the peer's forward queue
holds items. When it times out, the error names each peer still waiting and
what it waits for, including the plugin whose replay has not reported.

<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- announceEOR, logEORWithheldByManualEOR -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- endSessionAfterSyncPanic -->
<!-- source: internal/component/bgp/reactor/peer_sync_owed.go -- syncOwed, peerSyncOwedError -->

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
