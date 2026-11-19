---
spec: lg-peer-history-origin-search-and-named-nodes
topic: Looking Glass
feature: looking-glass
---

## Before

In `show bgp` output, each peer's `description` held the name of the group the
peer belongs to. The text written in a peer's `description` configuration leaf
appeared nowhere: the daemon accepted it and never showed it. A peer outside
any group had an empty `description`.

The looking glass peers page showed a session's state and uptime only. It did
not show when a session last changed state, how often it dropped, or why it
last closed, so a session that dropped every hour looked like one that had been
up for an hour.

The AS path field of the looking glass search refused `$`. The routes an AS
originates, and the routes with one exact AS path, could not be asked for.

The looking glass showed a next hop as a bare address, in its route tables and
in its next-hop graph, so an exchange LAN could not be told from a transit
link. Its peers page showed the peer's group under "Description".

The daemon kept no time for a route. The looking glass showed no age, and its
birdwatcher API answered `age: 0` for every route, which a client could not
tell from a route learned that second.

The looking glass drew two graphs for a prefix: the AS path, and the routers
with their next hops. Neither showed which routers of the network a prefix
crosses before it leaves, joined to the AS path it takes outside.

A neighbor that connected while the daemon was still starting, after it began
to listen and before it started its peers, could wait for an OPEN that never
came. The daemon kept the connection, sent nothing on it and logged nothing,
until the neighbor timed out and connected again.

## After

In `show bgp` output, `description` no longer holds the group name. It holds
the text of the peer's `description` configuration leaf, written on the peer or
inherited from its group, and it is empty when the configuration states none.
A new `group` key holds the group name that `description` used to carry, and
the table gains a `group` column after `description`. A script that read the
group from `description` reads `group` now.

Changing a peer's `description` and reloading the configuration updates the
output and leaves the session up.

The looking glass peers page has two new columns, Last Change and Drops: the
time of the session's last state change and the number of times it dropped.
The page of one peer, `/lg/peer/{address}`, shows both and adds the last
error, the reason the session last closed. An open peers page updates these
values from its live stream. A value the daemon did not report is an empty
cell, never a zero. The last error is readable by anyone who can open the
looking glass; set `token` to restrict it.

The AS path field of the looking glass search accepts a `$` directly after the
last AS number. `65001$` lists the routes AS 65001 originates, prepended or
not, and `^65010 65001$` lists the routes whose path is exactly 65010 then
65001. A `$` anywhere else is still refused.

A new list, `environment looking-glass next-hop-range <prefix> { name <name> }`,
names ranges of next-hop addresses. The looking glass shows the name beside a
next hop inside a range, in the search results, a peer's routes, the route
detail and the next-hop graph. When several ranges hold a next hop, the
longest prefix names it. A next hop in no range shows its address alone:
nothing is derived from DNS or from a peer. Two entries for one range are
refused at commit, the list takes 256 entries, and a reload replaces it on the
running looking glass without closing a connection. A reload that is rolled
back leaves the names it replaced. A range written as an IPv4-mapped IPv6
prefix, such as `::ffff:198.51.100.0/120`, is refused at commit, and the
refusal names the IPv4 range to write in its place, `198.51.100.0/24`.

The Description column of the looking glass peers page shows the peer's own
`description`, and is empty for a peer that has none. The group is shown apart,
as the tooltip of that cell and in the header of the peer's page. A peer's
description and the range names are readable by anyone who can open the
looking glass; set `token` to restrict them.

The daemon records when each received route last changed: when it was first
learned, or when an announcement changed one of its attributes. The same route
announced again unchanged keeps its age. `show bgp rib` answers it as `age`,
in whole seconds, on each received route. The looking glass route tables have
a new last column, Age, written like `3m 12s`, with the exact seconds in the
cell's `data-seconds` attribute, and the route detail shows the same. In the
birdwatcher API, `age` holds those seconds and a new `age_available` field
says whether the value is a reading. Routes the daemon sent
(`routes/export/{name}`) and best-path rows (`routes/table/{family}`) have no
age: the page shows an empty cell, and the API answers `age: 0` with
`age_available: false`. A client that shows an age reads `age_available`
first. Ages start again from zero when the daemon restarts, because it learns
every route again.

The looking glass has a third graph, Trace, beside AS Path and Next Hop, at
`/lg/graph?prefix=<prefix>&mode=trace`. It needs every router of the network
to hold an internal session with Ze. From each router that sent a route for
the prefix, it follows the BGP next hop to the router whose session address
that is, router by router, to the router where the prefix leaves the network,
and then draws the AS path of that route to the origin AS. Under the drawing,
one line per router states where its trace ended and why: at an exit, at a
router that sent no route for the prefix, at a loop, or at a router that
originates the prefix. The list stops at 100 lines and says how many traces it
left out. The trace follows BGP next hops only: the hops of the IGP between
two next hops are not drawn, and the page says so. `format=text` answers the
same graph as text.

A neighbor that connects while the daemon is starting now gets its session:
the daemon keeps the connection for the peer and runs the session on it when
the peers start. If the configuration no longer holds that peer, the daemon
sends a NOTIFICATION, Cease with subcode Connection Rejected, and closes the
connection.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
