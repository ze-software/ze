---
spec: lg-search-matches-what-the-pages-promise
topic: Looking Glass
feature: looking-glass
found-by: review
---

## Before

A host address typed in the looking glass prefix field, the graph or the
`routes/prefix` and `routes/search` API endpoints found only a route for that
exact host, never the network containing it: `10.1.2.3` missed `10.0.0.0/8`,
although the help page promised the covering prefix. A prefix with its length
also listed longer prefixes whose text starts the same way: `192.0.0.0/2`
listed `192.0.0.0/24` too, and the route detail asked the engine the same text
question. An AS path search with more than one AS number, such
as `65001 65002`, was refused by the engine as an unknown keyword, and pattern
characters the help page implied (`$`, `|`, brackets) were refused as invalid
AS numbers.

## After

A bare address lists every route whose prefix contains it, the most specific
prefix first for each peer, so a default route comes last. A prefix with its
length lists exactly that prefix, with host bits cleared (`10.0.0.1/8` asks for
`10.0.0.0/8`). The search page, the graph, the route detail and both API
endpoints give the same answer. The lookup reads only the routing table entries
on the path to the address, so it stays fast on a full table.

An AS path search takes AS numbers separated by spaces, matched as a run
anywhere in the path, with an optional leading `^` that anchors the run at the
start. Any other pattern character is refused with a message that names the
value and the form the field takes.

The CLI gains the same two lookups: `show bgp rib received covering <address>`
and `show bgp rib received exact <prefix>`. `prefix <text>` keeps its text
match, partial form included.

When the engine cannot answer, the looking glass pages and the birdwatcher
JSON `error` now carry three lines: the command and the failure class (the
first line is unchanged), why there is no answer, and
`fix: ze show log recent level warn component lg.server`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
