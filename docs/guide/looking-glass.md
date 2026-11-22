---
title: Looking Glass (Guide)
---
# Looking Glass

Ze includes a built-in looking glass that provides public, read-only access to BGP session state and route information. It runs as a separate HTTP server from the authenticated web UI.

<!-- source: internal/component/lg/server.go -- LGServer -->

## Configuration

Add the `looking-glass` block under `environment` in your Ze config:

```
environment {
    looking-glass {
        enabled true
        server main {
            ip 0.0.0.0
            port 8443
        }
        next-hop-range 198.51.100.0/24 {
            name ams-ix-lan
        }
        next-hop-range 2001:db8:100::/64 {
            name ams-ix-lan
        }
    }
}
```

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- YANG schema -->

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable the looking glass server. |
| `server <name> { ip }` | `0.0.0.0` | Listen address. Set to `127.0.0.1` to restrict to local access. |
| `server <name> { port }` | `8443` | Listen port. Must differ from the web UI port. |
| `tls` | `true` | Serve HTTPS. Set `false` to serve plaintext. |
| `certificate` | (empty) | Name of a `pki { certificate <name> }` entry to serve. Empty selects a self-signed certificate in the live store. |
| `token` | (empty) | Bearer token. When set, every `/api/` and `/lg/` route needs `Authorization: Bearer <token>`. Empty leaves the looking glass open. |
| `next-hop-range <prefix> { name }` | (none) | A name for one range of next-hop addresses, shown beside a next hop inside it. See [Names for next hops and peers](#names-for-next-hops-and-peers). |

Environment variable overrides: `ze.looking-glass.listen=ip:port`, `ze.looking-glass.enabled=true`, `ze.looking-glass.tls=false`, `ze.looking-glass.token=<token>`, `ze.looking-glass.certificate=<name>`.

### TLS is on by default

The looking glass binds `0.0.0.0` and publishes route data and session state, so
it serves HTTPS unless you turn TLS off. Two rules apply:

- Write `tls false` (or set `ze.looking-glass.tls=false`) to serve plaintext,
  for example behind a proxy that terminates TLS.
- Without a live store or named certificate, Ze cannot persist a self-signed
  certificate. Explicit `tls true` refuses the looking-glass listener.
  With the default TLS setting, it serves plaintext and warns that `ze init`
  is required for persistence. An explicit-file start creates a live store.

### Serve your own certificate

A visitor to a public looking glass has not installed your root, so a
self-signed certificate gives them a browser warning. Set `certificate` to the
name of a `pki { certificate <name> }` entry and the listener serves that leaf
and every intermediate the entry holds:

```
environment {
    looking-glass {
        enabled     true
        certificate lan
    }
}
```

| Rule | Detail |
|------|--------|
| Default | No `certificate` leaf means Ze generates and serves a self-signed certificate, which is the behavior of every release before this leaf existed. |
| Fail closed | A name the store does not hold, or an entry with no `private { key }`, refuses the start: Ze exits and names the missing certificate. A reload that names one is rejected as a whole, and the running looking glass keeps the chain it is serving. Ze never falls back to a self-signed certificate for a name you configured. |
| Rotation | Load new material under the same name and reload: the listener serves the new chain from the next handshake, with no rebind, so a viewer's open connection survives. |
| TLS off | `tls false` serves no certificate, so the leaf is inert. Neither the start nor a reload reads the name. |
| Plaintext by downgrade | A storeless stdin daemon took the TLS default and started the looking glass in plaintext. A later certificate name does not enable TLS; restart to serve the named chain. |
| No live store needed | A named certificate comes from the `pki {}` container. Only the self-signed certificate needs the live store. |
| Name | 1 to 255 characters, `A-Z a-z 0-9 . _ -`. It is a store key, never a file path. |
| Own leaf | The looking glass and the web UI have separate `certificate` leaves, so each listener serves the certificate that matches its own hostname. |
| Env override | `ze.looking-glass.certificate` takes precedence over the config file. |

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf certificate -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial (fail-closed selection) -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the looking-glass certificate startup gate -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName and the reload refusal -->
<!-- source: cmd/ze/hub/listener_migrate.go -- updateLGCertificate (rotation without rebind) -->

### Optional bearer token

A looking glass is normally public and read-only, so `token` is off by default.
Set it to require a bearer token on every route. Ze compares the token
constant-time over SHA-256 digests. A request with no token, a wrong token, or
an `Authorization` header that is not `Bearer <token>` gets `401`.

When the `looking-glass` block is absent, no HTTP server is started and no resources are consumed.

<!-- source: cmd/ze/hub/service_lg.go -- buildLGService -->

## Web UI

The HTMX web UI is available at `https://<host>:<port>/lg/` (`http://` when you
set `tls false`). No authentication is required unless you set `token`.

| Tab / View | URL | Description |
|------------|-----|-------------|
| Peers tab | `/lg/peers` | All peers with state, session history, remote AS, route counts. The remote AS cell shows the number; its AS name (from Team Cymru, through `resolve cymru asn-name`) shows when the pointer rests on the cell. An AS it names nothing for, such as a private AS number, or a lookup that failed, has no name to show. Live SSE updates. |
| Lookup tab | `/lg/lookup` | Prefix/IP lookup with inline all-peers AS path topology graph (SVG). |
| Search tab | `/lg/search` | Unified search: prefix, AS path, or community. Type selector. |
| Per-peer routes | `/lg/peer/{address}` | Routes received from a specific peer (inline below peers table). |
| Route detail | `/lg/route/detail` | Expanded route attributes and the route's age (HTMX fragment, click-to-expand). |
| AS path graph | `/lg/graph?prefix=X` | Server-side SVG topology from all peers (auto-loaded on lookup). Each AS is labelled `AS<n>`, and its name, when Team Cymru gives one, shows on mouse-over only, so a name never widens a node. The text form (`format=text`), read by scripts, still writes the name, as it always did. |

### Session history on the peers page

Uptime says nothing about a session that is down, and a session that drops
every hour shows an uptime of an hour. The peers table therefore carries two
history columns, and the header of a peer's own page carries a third value:

| Shown | Where | What it is | Read from `show bgp` |
|-------|-------|------------|----------------------|
| Last Change | A column of the peers table, and the header of `/lg/peer/{address}` | The time the session last changed state, as the engine writes it (RFC 3339, such as `2026-07-15T10:30:00Z`). It is the most recent transition of the session's state machine, so a session that keeps retrying shows the time of its last attempt | `state-changed` |
| Drops | A column of the peers table, and the header of `/lg/peer/{address}` | How often the session has dropped. A peer that never dropped shows `0` | `connections-dropped` |
| Last error | The header of `/lg/peer/{address}`, which a click on the peer's row opens below the table | Why the session last closed: the NOTIFICATION code and subcode by name, such as `Cease/Administrative Shutdown`, with ` (not sent)` when Ze could not send it | `last-error` |

The last error has no column, because its text is long and the table has to
stay readable on a small screen.

The looking glass shows these three values as the engine answers them and
computes nothing from them. A value the engine did not send is an empty cell,
or no entry in the header. It is never a `0`: a zero means the session never
dropped, which the page says only when the engine said it.

### Names for next hops and peers

A next hop is an address, and an address does not say whether it belongs to an
exchange LAN or to a transit link. Name the ranges once in the config, and the
looking glass shows the name beside each next hop inside them:

```
environment {
    looking-glass {
        next-hop-range 198.51.100.0/24 {
            name ams-ix-lan
        }
        next-hop-range 198.51.100.200/32 {
            name ams-ix-rs1
        }
    }
}
```

| Rule | Behavior |
|------|----------|
| Where the name shows | Beside the next hop in the search results, in a peer's route table and in the route detail, and as the label of the node in the next-hop graph. The graph's text form (`format=text`) adds `name=<name>` to the node's line |
| Several ranges hold the next hop | The longest prefix names it. In the example, `198.51.100.200` is `ams-ix-rs1` and `198.51.100.7` is `ams-ix-lan` |
| No range holds the next hop | The page shows the address alone. Ze derives no name from DNS, from a peer or from the address: a router has a name only where you named its range |
| One range, one name | Two entries for one range are refused at commit, and the refusal names both entries and ends with the line that removes the second. `198.51.100.1/24` beside `198.51.100.0/24` is such a pair, because both are the range `198.51.100.0/24`. Nested ranges are distinct and commit. One name may label several ranges, such as the IPv4 and IPv6 LANs of one exchange |
| An IPv4 range is written as IPv4 | A range written as IPv4-mapped IPv6, such as `::ffff:198.51.100.0/120`, is refused at commit, and the refusal names the IPv4 range to write, `198.51.100.0/24`. The looking glass reads an IPv4-mapped next hop as its IPv4 address, so only the IPv4 range names it |
| Limits | 256 entries at most. A name takes 1 to 64 characters from `A-Z a-z 0-9 . _ -` and starts with a letter or a digit |
| Reload | A reload replaces the whole list on the running server. The listener keeps its socket, so an open connection reads the new names on its next request. A reload that fails after that step and is rolled back puts the previous list back |
| `enabled` | The list is read whether or not the block says `enabled true`, so a looking glass started by `ze.looking-glass.enabled` or `ze.looking-glass.listen` shows the names too. The list has no environment variable |

The peers table has a Description column. It shows the text of the peer's own
`description` leaf, written on the peer or inherited from its group, and it is
empty for a peer that has none. The peer's group is apart: it is the tooltip
of the description cell and a `Group` entry in the header of
`/lg/peer/{address}`. The birdwatcher API answers the same text in its
`description` field.

A peer's description and the names of your next-hop ranges are public where
the looking glass is public. Write nothing in them that a visitor must not
read, or set `token`.

<!-- source: internal/component/config/lg_next_hop_ranges.go -- ExtractLGNextHopRanges: the reader, the one-range-one-name refusal, the 256 bound -->
<!-- source: internal/component/lg/nexthop_ranges.go -- nextHopRanges.nameOf (longest prefix), UpdateNextHopRanges, validateNextHopRanges -->
<!-- source: internal/component/lg/handler_ui.go -- extractPeers, peerInfoFrom read description and group -->

### Route age

Every route table has an Age column, the last one: how long ago the route last
changed. The search results and a peer's route table show it, and so does the
route detail a click on a row opens.

| Rule | Behavior |
|------|----------|
| What the age counts | The time since `bgp-rib` last stored a change to the route: when it was first learned, or when one of its attributes changed. The same route announced again with equal attributes keeps its age, so a route refresh does not make a stable route look new |
| How it is written | With its largest units, as the Uptime column of the peers table: `42s`, `3m 12s`, `2h 5m 3s`, `4d 3h 0m` |
| The exact value | The cell carries the whole seconds in its `data-seconds` attribute, such as `<td class="route-age" data-seconds="192">3m 12s</td>`, for a script that reads the page |
| A route with no age | The cell is empty and carries no `data-seconds`, and the route detail has no Age entry. It is never `0`: a zero means the route changed this second |
| Resolution | One second. The page and the API read the same value, so they differ only by the time between the two requests |
| Restart | Ages start again from zero when the daemon restarts, because it learns every route again. The age says how long this daemon has held the route unchanged, which is at most the time since its start |

The looking glass computes nothing here: `bgp-rib` writes the seconds in the
`age` member of each received route when it answers, and the page shows them.
The birdwatcher API answers the same seconds in its `age` field, with
`age_available` beside it (see below).

<!-- source: internal/component/lg/handler_ui.go -- routeAgeFrom, routeRows -->
<!-- source: internal/component/lg/view.go -- routeAge -->
<!-- source: internal/component/lg/render.go -- formatElapsed -->
<!-- source: internal/component/bgp/plugins/rib/rib_route_age.go -- writeRouteAge -->

### Inside by address, outside by AS number

The next-hop graph and the trace draw the two sides of your network's edge
differently, so a reader tells them apart at a glance:

| What the address is | How the graphs draw it | Why |
|---------------------|------------------------|-----|
| A router of your AS: a peer whose session with Ze is internal (`peer-type` `internal` in `show bgp`) | By its address, with the name of its `next-hop-range` when it is in one. Green when it is an egress | Inside your AS the address is what tells one router from another |
| A neighbour outside your AS: a peer whose session with Ze is external (`peer-type` `external`) | By its AS number only, `AS<n>`, in a dashed box, with the AS name on mouse-over as the AS path graph shows it. Its address is nowhere in the graph. Two neighbours of one AS are one node, and the edges to either meet on it | Outside your AS the looking glass states who the traffic is handed to, never which address of theirs |
| An address that is no peer of Ze: a next hop with no session, or a route held for a peer with no session | By its address, as a route wrote it | Ze knows no AS for it, and nothing is derived |

In the next-hop graph's text form (`mode=nexthop&format=text`) a router or an
address reads `node <address> layer=N [egress] [name=<name>]`, and an AS
outside reads `node AS<n> layer=N outside`. `egress` stays a statement about a
router of your AS: it announces its own address as next hop. An AS outside is
never `egress`.

The next-hop graph therefore reads the peer summary beside the routes. When
the summary cannot be read, or does not say for a peer whether its session is
internal or external, the graph is refused with the reason and
`ze show bgp`, as the trace is. It is never drawn with every node as an
address, because that would show a neighbour as if it were one of your
routers.

<!-- source: internal/component/lg/graph_nexthop.go -- nextHopOutsideFrom, buildNextHopGraph, renderNextHopGraphText -->
<!-- source: internal/component/lg/handler_graph.go -- handleNextHopGraph, nextHopSummaryRefusal -->
<!-- source: internal/component/lg/layout_nexthop.go -- outsideNodeClass, renderNextHopGraphSVG -->

### Tracing a prefix through the network

When every router of your network holds a session to Ze, the looking glass
holds each router's chosen route for a prefix, and the BGP next hop that
router uses. Search for a prefix and click **Trace** above the route table:
the graph follows the prefix from each router of the AS to the origin AS.
The same graph is at `/lg/graph?prefix=<prefix>&mode=trace`, and
`&format=text` answers it as text.

A walk starts at each router of the AS that sent a route for the prefix. A
router of the AS is a peer whose session with Ze is internal (iBGP): `show bgp`
lists it with `peer-type` `internal`. The walk reads the router's
next hop, moves to the router whose session address that is, and repeats.
Routers are drawn left to right in the order a walk crosses them, the exit
router in green, then the AS path of the exit route with the origin AS on the
right. Under the graph, one line per router states how its walk ended:

| The router's next hop, or what Ze holds | The line says | What follows |
|------------------------------------------|---------------|--------------|
| Its own address | It leaves the AS at that router, which announces its own address as next hop | The AS path of that route |
| The address of no router of the AS | It leaves the AS at that router toward that address | The AS path, starting at the AS the traffic is handed to: the exit router links straight to it. The address is outside your AS, so it is in the line and in the text form's `next-hop=` field, and is never a node. The neighbour at that address needs no session with Ze |
| Either of the two, with an empty AS path | The prefix originates inside the AS at that router | No AS. A next hop that is no router's address is still drawn by its address: the path is empty, so there is no AS to draw it as, and nothing is derived |
| A router the walk already crossed | The walk would loop, and at which router | Nothing |
| A router that sent Ze no route for the prefix | Ze holds no route from that router for the prefix | Nothing |
| Ze holds several different routes from the router (ADD-PATH) | No row says which one the router chose | Nothing |
| A route whose next hop or AS path the trace cannot read | Its next hop or its AS path cannot be read | Nothing |

A walk never ends without its line. The looking glass computes nothing about
a route: it follows what each router sent.

When no walk starts, the graph is replaced by one line that begins with
`No trace:`. It says which fact left nothing to draw, and ends with three
counts: the peers of the looking glass, the routers of the AS among them, and
the routes the query answered. Zero routers means no peer has an internal
session. Zero routes means Ze holds no received route for the prefix. The line
names no command, because a visitor of a public looking glass cannot run one:
as the operator, read the peers and their `peer-type` with `ze show bgp`.

**A router is matched by its session address.** The trace finds the next
router by comparing a next hop with the address each router's session with Ze
uses. A router that writes another of its addresses as next hop, a loopback
it does not peer from for example, is read as a neighbour outside the AS, and
the walk leaves the AS there. Peer with Ze from the address your routers
write as next hop.

The same holds across address families. A router that holds an IPv4 session
with Ze and writes an IPv6 next hop on the routes it sends over it (RFC 8950,
or IPv6 routes carried over the IPv4 session) is known to the trace by its
IPv4 address only. Its IPv6 next hop is the address of no router the trace
knows, so the walk states an exit toward an external address at that router,
even when the next hop is the router itself or another router of the AS. Give
each router a session with Ze from the address family of the next hops it
writes.

**The trace is BGP's view.** It follows each router's BGP next hop, from one
router of the AS to the next. The hops your IGP takes between two consecutive
next hops are in no route Ze holds, so they lie outside what the graph draws.
The graph and the help page say so.

The graph draws at most 100 nodes (routers, ASes, and the next hops of
prefixes that originate inside the AS, together) and says so when a prefix
has more. Names from `next-hop-range` label the routers and those next hops.
An AS is labelled `AS<n>` and its name shows on mouse-over.

The lines under the graph, and the `trace` lines of the text form, stop at
100. A bare address asks for every prefix that contains it, and each prefix
is walked from every router that sent it, so such a lookup can hold more
walks than that. Every walk still gives the graph its routers and
ASes; the last line then says how many traces are not listed, and the text
form ends with an `unlisted traces=<count>` line. Ask for one prefix with its
length to read all of its traces.

<!-- source: internal/component/lg/handler_graph.go -- handleGraph, handleTraceGraph -->
<!-- source: internal/component/lg/graph_trace.go -- traceRoutersFrom, traceWalkFrom, traceWalk.statement, traceNote, traceGraph.listedWalks, traceCounts.noTrace -->
<!-- source: internal/component/lg/layout_trace.go -- computeTraceLayout, renderTraceGraphSVG -->

### Live updates and what is public

The live stream (`/lg/events`) sends the whole rows, so the two columns of an
open peers page follow the sessions: a drop appears on the next message, at
most five seconds later.

**The last error is public where the looking glass is public.** Anyone who can
open `/lg/peer/{address}` reads why each session last closed. The text holds
only names Ze knows for the NOTIFICATION code and subcode, or fixed words for
a session Ze closed itself. It never holds bytes the peer sent: an unknown
code is shown as `Unknown(N)`. Set `token` when the reason your sessions close
is not for everyone to read.

<!-- source: internal/component/lg/handler_ui.go -- extractPeers, peerInfoFrom, connectionsDroppedFrom, peerStreamBody -->
<!-- source: internal/component/lg/view.go -- peerRow, connectionsDropped, peersColumns -->
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- lastErrorString, stateChangedString -->

The prefix field of the search, the graph and the API's `prefix` parameter
read the value the same way:

| You type | The looking glass lists | Sent to `bgp-rib` |
|----------|------------------------|-------------------|
| A prefix with its length, such as `10.0.0.0/8` | The routes of exactly that prefix, never a longer prefix inside it. Host bits are cleared first, so `10.0.0.1/8` asks for `10.0.0.0/8` | `show bgp rib \| received \| exact 10.0.0.0/8` |
| A bare address, such as `10.1.2.3` or `2001:db8::1` | Every route whose prefix contains the address, the most specific prefix first for each peer, so a default route comes last | `show bgp rib \| received \| covering 10.1.2.3` |
| Anything else, or a length past 32 (IPv4) or 128 (IPv6) | Nothing: the page shows the value, why it is refused, and a valid form | Nothing |

An address finds IPv4 and IPv6 routes only. Routes keyed by a route
distinguisher, such as VPN routes, are not searched this way.

<!-- source: internal/component/lg/prefix_lookup.go -- parsePrefixLookup, prefixLookup.receivedCommand -->

The AS path field of the search takes AS numbers separated by spaces, as a
router prints a path:

| You type | The looking glass lists | Sent to `bgp-rib` |
|----------|------------------------|-------------------|
| `65001 65002` | The routes whose path holds 65001 directly followed by 65002, anywhere in the path. A path holding 65002 then 65001 is not listed | `path 65001,65002` |
| `^65001` | The routes whose path starts with 65001 | `path ^65001` |
| `^65001 65002` | The routes whose path starts with 65001 then 65002 | `path ^65001,65002` |
| `65001$` | The routes whose path ends with 65001: the routes AS 65001 originates. A path that crosses 65001 and ends at another AS is not listed. A path ending `65001 65001 65001` is listed, because prepending does not change the origin | `path 65001$` |
| `^65010 65001$` | The routes whose path is exactly 65010 then 65001. A longer path that holds the pair is not listed, and neither is `65010 65001 65001` | `path ^65010,65001$` |
| `65001$$`, `$65001`, `65001 $`, `(65001\|65002)`, or any other pattern character | Nothing: the page names the value and says the field takes AS numbers separated by spaces, with an optional leading `^` and an optional `$` directly after the last AS number | Nothing |

The form of the field is `[^] <asn> [<asn> ...] [$]`. The `$` is read in one
place only, directly after the last AS number, with no space before it.
Commas work as separators too. An AS number may be written as `65546` or as
`1.10`, and is sent as `65546`. Hover the field to read the form.

<!-- source: internal/component/lg/aspath_pattern.go -- parseASPathPattern -->

Navigation uses a single-page tab layout with HTMX fragment swapping. The real htmx.min.js (v4.0.0) is embedded, and the peers page also loads hx-sse.min.js, htmx 4's SSE extension.

When the engine cannot answer, a page says so. The peers and search pages show
a banner. The per-peer page, its download, the route detail and the graph
answer 502 (the command failed) or 503 (the engine answered nothing usable)
instead of a "not found" or an empty result. The message names the command and
the kind of failure on its first line, says why there is no answer on the
second, and ends with the command that shows the full reason from the daemon
log:

```
show bgp: engine error
the engine failed or refused this command, so there is no answer to show
fix: ze show log recent level warn component lg.server
```

<!-- source: internal/component/lg/handler_ui.go -- UI handlers -->
<!-- source: internal/component/lg/server.go -- ask, engineFailure, engineRefusal -->
<!-- source: internal/component/lg/handler_graph.go -- Graph handler -->

## Birdwatcher REST API

The looking glass exposes a birdwatcher-compatible JSON API for integration with tools like Alice-LG.

| Endpoint | Description |
|----------|-------------|
| `GET /api/looking-glass/status` | Router ID, version, start time, and when the last config reload finished. It answers 502 when one of the four commands behind it fails, and never a status object with an empty router ID. |
| `GET /api/looking-glass/protocols/bgp` | Peer list with state and route counts. |
| `GET /api/looking-glass/routes/protocol/{name}` | Routes from a named peer. |
| `GET /api/looking-glass/routes/table/{family}` | Best routes by address family (URL-encode the `/` in family, e.g., `ipv4%2Funicast`). |
| `GET /api/looking-glass/routes/filtered/{name}` | Filtered routes per peer. |
| `GET /api/looking-glass/routes/search?prefix=X` | Prefix lookup across all peers: the routes of exactly one prefix given with its length, or every route covering a bare address, the most specific first for each peer. `routes/prefix` answers the same. |

All API responses use `Content-Type: application/json` with birdwatcher-convention `snake_case` field names (not Ze's standard `kebab-case`).

An endpoint whose command fails answers 502, or 503 when the engine answered
nothing usable, with a JSON `error` that carries the same three lines as the
pages, for example `show bmp peers: engine error` on the first line from
`/protocols/bmp` on a daemon that runs no BMP. It never answers 200 with an
empty result. The last line names the command that shows the full reason.
<!-- source: internal/component/lg/handler_api.go -- engineAnswer, writeEngineFailure -->

The per-peer route counts on `/protocols/bgp` come from the `bgp-rib` plugin's
Adj-RIB-In and Adj-RIB-Out sizes, merged into `show bgp`:
`routes_received` and `routes_imported` are both the Adj-RIB-In size (Ze retains
only accepted routes, so there is no distinct pre-policy received count here),
and `routes_exported` is the Adj-RIB-Out size. `routes_filtered` is always `0`:
Ze does not retain import-filtered routes (unlike BIRD's "import keep filtered"),
so the `/routes/filtered/{name}` endpoint also returns an empty list.

Ze emits the four counts on every peer, whatever it knows. When it has no
source, each count is `0` and `routes_counts_available` is `false`. That happens
when the `bgp-rib` plugin is not loaded, and on every BMP-monitored peer. A
client that must tell "no routes" from "unknown" reads
`routes_counts_available`, never a count of `0` on its own. The normative
statement is [Birdwatcher compatibility](../architecture/api/birdwatcher-compat.md),
Section 7.2.

Every route carries `age`, the whole seconds since the route last changed, and
`age_available`. A route Ze holds a change time for answers its age and
`age_available: true`: the routes a peer sent, on `routes/protocol/{name}`,
`routes/peer/{peer}`, `routes/prefix` and `routes/search`. A route with no
reading answers `age: 0` and `age_available: false`: every route of
`routes/export/{name}` (routes Ze sent) and of `routes/table/{family}` (best
paths). A client that shows an age reads `age_available` first, and never
takes an `age` of `0` on its own for a route learned this second. Ages start
again from zero when the daemon restarts. The normative statement is Section
6.2 of the same page.

<!-- source: internal/component/lg/handler_api.go -- transformRoutes -->
<!-- source: internal/component/lg/handler_ui.go -- routeAgeFrom -->
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- fetchRibRouteCounts, mergeRibRouteCounts -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- status per-peer route-counts -->
<!-- source: internal/component/lg/handler_api.go -- API handlers and birdwatcher transform -->
<!-- source: internal/component/lg/handler_api.go -- routeCountsAvailable -->

## RFC 8522 Looking Glass Command Set API

The looking glass also implements the [RFC 8522](https://www.rfc-editor.org/info/rfc8522) Looking Glass Command Set under `/.well-known/looking-glass/v1/`, answering every request in JSend JSON (`status` of `"success"`, `"fail"`, or `"error"`).

| Endpoint | Requirement | Description |
|----------|-------------|-------------|
| `GET /.well-known/looking-glass/v1/ping/{host}` | Mandatory | Runs `show ping` from the router and returns RTT statistics and output. |
| `GET /.well-known/looking-glass/v1/traceroute/{host}` | Optional | Runs `show traceroute` from the router. |
| `GET /.well-known/looking-glass/v1/show/route/{addr}` | Mandatory | Queries `show route <cidr>` or `show route lookup <ip>`. |
| `GET /.well-known/looking-glass/v1/show/bgp/{addr}` | Optional | Queries received BGP routes for a prefix or covering address. |
| `GET /.well-known/looking-glass/v1/show/bgp/summary` | Optional | Returns BGP peer summary (`show bgp`). |
| `GET /.well-known/looking-glass/v1/show/bgp/neighbors/{addr}` | Optional | Returns BGP peer details (`show bgp peer <addr> detail`). |
| `GET /.well-known/looking-glass/v1/routers` | Mandatory | Lists the router available on this looking glass. |
| `GET /.well-known/looking-glass/v1/routers/{number}` | Mandatory | Returns router details (`id`, `name`, supported `format`, `vendor`, `autonomous_system`). |
| `GET /.well-known/looking-glass/v1/cmd` | Mandatory | Lists all supported commands and their URI templates. |

HTTPS is required for `/.well-known/looking-glass/` per RFC 8522 Section 6: plaintext HTTP requests are refused with HTTP `400` and a JSend error envelope, while `/api/looking-glass/` and `/lg/` continue to honor `tls false`. Requests are rate-limited per client IP (default 60 requests per minute, HTTP `429` when exceeded). Supported query parameters are `protocol`, `router`, `routerindex`, `random` (ignored), `vrf`, `runtime` (seconds, default 30, 0 disables), and `format` (`text/plain` or `application/json`).

<!-- source: internal/component/lg/handler_rfc8522.go -- handleRFC8522, rfc8522RateLimiter -->

## Alice-LG Integration

To use Ze as a data source for [Alice-LG](https://github.com/alice-lg/alice-lg), point Alice-LG's birdwatcher source configuration at the looking glass API:

```yaml
sources:
  - name: "Ze Router"
    type: birdwatcher
    birdwatcher:
      api: "https://ze-host:8443/api/looking-glass"
```

## Security

The looking glass is designed for public IXP deployment. It is read-only, and open unless you set `token`. Security measures include:

- Strict input validation on all query parameters (character allowlists, length limits).
- `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` headers.
- `X-Ze-Version` names the exact build. Write `environment { hide-version true; }` to keep it off the looking glass and the web interface together.
- SSE connection limit (100 concurrent clients).
- All HTML output rendered through templ, which escapes every interpolated value. The two SVG graph builders stay in Go and escape their labels with `template.HTMLEscapeString`.
- The `Content-Security-Policy` is `default-src 'self'`, so no page carries an inline script or an inline event handler. A test refuses one in any `.templ` source of the package.
- No direct RIB or plugin imports; all data accessed via command dispatcher.
<!-- source: internal/component/lg/markup_check_test.go -- TestTemplatesAvoidInlineScriptAndStyle, lgMarkupExempt -->
<!-- source: internal/component/lg/layout.go -- renderGraphSVG label escaping -->

When TLS is enabled, the server uses TLS 1.2 minimum. It reads its own
`certificate` leaf, resolved against the same PKI store the web UI reads, and
serves a self-signed certificate when that leaf is unset.
