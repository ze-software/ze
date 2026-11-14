---
spec: single-owner-lifecycle-5-session-end-locals
topic: Quality Improvement
---

## Before

Three things went wrong when a session or a plugin ended.

An LDP session that had been up for days and then closed was treated as a
failed setup: Ze waited 15 seconds before dialling the neighbor again, then 30,
up to 2 minutes, and the delay never cleared. A setup attempt that never became
a session still announced a session-down, so OSPF LDP-IGP synchronization cost
the whole link out, even while another neighbor's session on the same link was
up.

On an L2TP LNS, a subscriber cleared by the operator, a timeout or a RADIUS
Disconnect while its address was being assigned could reappear: the late
address event re-created the subscriber entry, started a RADIUS accounting
session nothing ever stopped, and re-injected the subscriber route.

The ddos-flowspec plugin withdrew its FlowSpec discard rule only on a config
change that kept the plugin running. Removing the plugin, removing the whole
`ddos` block, or the plugin ending left the rule at every BGP peer, and a
restarted plugin did not know it existed. A reload that removed the
`ddos flowspec` section or its `ddos` parent was refused at verify.

## After

An LDP session that reached operational and then ends is redialled on the
neighbor's next Hello, and it announces exactly one session-down. A failed
setup still backs off from 15 seconds to 2 minutes, and announces nothing, so
OSPF keeps the link as it was. `ldp: session operational` is now logged at
INFO.

The events of one L2TP session reach RADIUS accounting, the subscriber table,
the route observer, the pool, the shaper and CoS in the order the session's
state changed. A cleared session leaves no subscriber entry, one accounting
Stop for its Start, and no injected route.

ddos-flowspec withdraws its rule when the daemon removes the plugin, and when
the `ddos flowspec` section or the whole `ddos` block is removed by a reload,
which now passes verify. It records the rule while it is announced. When it
could not withdraw (the daemon stopped, the plugin was killed, or it ended on
its own while running inside the daemon), its next start withdraws the
recorded rule. The daemon does not restart the plugin by itself: while it is
configured and stopped, `ze doctor` reports it as stopped
(`doctor-plugin-stopped`) and, through the daemon, reports the rule it left
with the victim prefix, both with `ze request plugin restart ddos-flowspec`;
that start withdraws the rule. With the section removed, `ze doctor` reports
the recorded rule as `doctor-ddos-flowspec-orphan-announcement` with the victim
prefix and the withdrawal to send. When the plugin could not confirm that the
rule was recorded, its log line carries that withdrawal too.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
