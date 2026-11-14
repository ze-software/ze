---
spec: single-owner-lifecycle-2-bgp-session
topic: BGP
found-by: review
---

## Before

Three things went wrong around a BGP session ending and starting.

When a session dropped and the peer reconnected at once, work left over from
the first session could still run after the second one came up. Routes queued
for the new session could be taken off the queue and written to the connection
that was gone, and an End-of-RIB meant for the old session could reach the new
one, so the peer could see End-of-RIB before the routes it was supposed to
close, or miss the new session's own marker.

The reconnect wait never came back down. Ze waits 5 seconds before redialling
a peer whose session failed and doubles the wait after each failure, up to 60
seconds, but only stopping the peer reset it. A peer that ran for a week and
then lost its session once was redialled on whatever wait its history had
left. The same held for the wait after a prefix-limit teardown under
`reconnect timer`.

A passive peer added while a standalone `ze` was still waiting for its plugins
at start got no listener, so nothing accepted its connection.

## After

Each session's initial routes and End-of-RIB belong to that session only. Ze
finishes or abandons the old session's initial update before the next session
starts, and a reconnecting peer receives the new session's routes, then one
End-of-RIB per family.

After a session that stayed established for at least 60 seconds, the next
reconnect wait starts again from 5 seconds, and the prefix-limit `timer` wait
from `idle-timeout`. A peer that flaps faster than that is still slowed down.

A passive peer added during the standalone start gets its listener as soon as
the start has opened the configured ones. Its connection waits until peers
start, as before.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
</content>
</invoke>
