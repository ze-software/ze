---
spec: route-policy-1-gate-proof-and-forked-sysrib
topic: Plugins
feature: route-policy
found-by: review
---

## Before

An external system RIB lacked the daemon's selected routing state. It could
not provide the internal mode's administrative distances, next-hop resolution,
ECMP and candidate commands.

Testing both placements against a real kernel found four kernel-route faults
in either placement. An IPv4 prefix with an OSPF fast-reroute backup got no
kernel route at all: Linux refused the whole route. A withdrawn route whose
next hops were all directly connected interfaces stayed in the kernel, and
stayed after Ze stopped. A Ze route in a named routing table survived the stop
flush and the crash sweep, while the flush reported clean. A route whose
gateway is reached through a connected route in the same update could be
refused with "network is unreachable" and was never added.

When the crash sweep at startup could not list the kernel's routes, it said so
only in a log line: routes a crashed run left behind kept forwarding and
`ze doctor` reported nothing. An address given to an interface at the moment
the interface was created could be lost for as long as Ze ran: the kernel held
the address, but `show rib` had no connected route for it, so routes resolved
through that link were missing. After an update installed a route the kernel
had first refused, or one kept from a previous run, the
`ze_fibkernel_routes_installed` metric read one low until the next add or
removal.

Routes a previous run left in the kernel were not always handled as Ze's own.
When the kernel refused the startup sweep's delete of one, the route could
outlive Ze's stop with nothing reported at stop. A protocol installing a prefix
into the table that held such a route was refused by it: the install was
logged as a sync failure while the stale next hop kept forwarding. And the
sweep could warn "orphan routes removed from FIB" for a route a protocol had
just refreshed or withdrawn, which was no orphan.

## After

Internal and external system RIBs now consume the daemon's routes after import
policy and selection. Both use the same distance updates, next-hop resolution,
ECMP and candidate commands. A lost plugin connection withdraws its published
routes before replacement.

A route with a fast-reroute backup is installed with its primary path, and
`show warnings` reports `doctor-fib-backup-not-programmed`: the backup does not
take over when the primary link fails, and traffic recovers when OSPF
reconverges. Ze removes its routes at whatever scope the kernel holds them, its
stop flush and crash sweep cover every routing table and report a route they
could not remove (`doctor-fib-sweep-incomplete`), and a gateway route is always
sent after the route it needs. A startup sweep that cannot list the kernel now
raises `doctor-fib-sweep-incomplete` with the error and the remedy. The
interface monitor keeps an address announced before its interface, and an
address it still cannot attribute raises `doctor-iface-address-unattributed`
with the commands that announce it again. The installed-routes metric counts
a route an update installs.

A stale route whose sweep delete the kernel refused stays Ze's: the stop flush
tries it again and reports it if it is still there. An install into its table
replaces it, and the sweep deletes and counts only the routes startup listed
that nothing has replaced or removed since.

The `show rib candidates` command has one answer path in both placements.

## Corrects

none

## Exposure

A system RIB run as an external plugin (`plugin { external <name> { run "ze
plugin rib"; } }`). On the kernel FIB writer in either placement: an IPv4
route with an OSPF fast-reroute backup, a route whose next hops are all
directly connected interfaces, a Ze route in a named routing table, and a
gateway route announced in the same update as the connected route it needs.
Any interface created and addressed in one step while Ze runs. Any start of
Ze that found routes of a previous run still in the kernel, after a crash or
an unclean stop.

## Upstream

none

## Asked by

none
