# The Birdwatcher-compatible Looking Glass API

## Status of this document

This is a specification of the JSON API Ze's looking glass serves for Alice-LG
and other birdwatcher clients. It is normative for Ze: an implementation change
that contradicts a MUST here is a defect in the implementation or in this
document, and one of the two is wrong.

It exists because there is no other reference. Upstream birdwatcher publishes no
schema for its own responses, and its README defers to "an API defined by Barry
O'Donovan's birds-eye". Every statement below about UPSTREAM behavior was
established by reading `alice-lg/birdwatcher`'s source and names the function it
came from; nothing here is quoted from upstream documentation, because none
exists.

## 1. Conventions

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT,
RECOMMENDED, MAY and OPTIONAL in this document are to be interpreted as described
in RFC 2119.

"Server" means Ze's looking glass. "Client" means Alice-LG or any other consumer
of this API. "Upstream" means `alice-lg/birdwatcher`.

## 2. Transport and framing

The server MUST serve every endpoint over HTTP GET. It MUST NOT require any
other method, and it MUST NOT define side effects for any endpoint: this surface
is read-only.

The server MUST set `Content-Type: application/json`.

The server MUST answer a request under `/api/looking-glass/` that matches no
endpoint with a JSON error body, and MUST NOT fall through to an HTML error
page. A client parsing JSON must never receive markup.
<!-- source: internal/component/lg/server.go -- route registration -->

Field names MUST be `snake_case`. This is the one surface where Ze abandons its
own `kebab-case` convention, because the client vocabulary is birdwatcher's.
Translation between the two is the responsibility of the `transform*` functions.
<!-- source: internal/component/lg/handler_api.go -- transformProtocols -->

## 3. Endpoints

The server MUST expose the following. All are relative to
`/api/looking-glass/`.

| Endpoint | Payload key | Returns |
|----------|-------------|---------|
| `status` | `status` | Daemon status and version |
| `protocols/bgp` | `protocols` | Every BGP session as a protocol object |
| `protocols/short` | `protocols` | The same set, reduced |
| `protocols/bmp` | `protocols` | BMP-monitored peers as protocol objects |
| `routes/protocol/{name}` | `routes` | Routes the session sent Ze (Section 3.2) |
| `routes/peer/{peer}` | `routes` | Routes one peer address sent Ze (Section 3.2) |
| `routes/table/{family}` | `routes` | Routes in one address family |
| `routes/filtered/{name}` | `routes` | Routes an import policy rejected |
| `routes/export/{name}` | `routes` | Routes Ze sent a session (Section 3.2) |
| `routes/noexport/{name}` | `routes` | Routes withheld from a session |
| `routes/count/protocol/{name}` | `routes` | A count only: `routes` is a number, not a list. It counts what `routes/protocol/{name}` lists |
| `routes/prefix` | `routes` | Routes peers sent Ze for the `prefix` parameter: the routes of exactly one prefix given with its length, or every route covering a bare address, the most specific first within each peer (Section 3.3) |
| `routes/search` | `routes` | The same lookup as `routes/prefix`, under a Ze-specific path (Section 3.3) |
| `routes/bmp/{name}` | `routes` | Routes seen for a BMP-monitored peer. `{name}` is a key `protocols/bmp` lists |

A `{name}` or `{peer}` path segment MUST be validated before use. The server
MUST answer an invalid one with 400 and MUST NOT pass it to the engine.

Endpoints returning routes MAY accept `limit` and `offset` query parameters. When
a client sends a `limit` above the server's maximum, the server MUST answer 400
and MUST NOT silently clamp: a client must never believe it received a full page
when it did not.
<!-- source: internal/component/lg/handler_api.go -- parsePagination -->

### 3.1 A command the engine refuses

`status` reads four commands, because no single command holds the whole
identity: `show bgp` answers the router id, `show version` the release,
`show uptime` the start time, and `show reload-status` the last configuration
change. Every one of them MUST answer. The server MUST NOT build a status
object from a command that failed.

The server MUST answer `502 Bad Gateway` when the engine returns an error, and
`503 Service Unavailable` when the engine returns nothing the server can parse.
The body MUST be a JSON object carrying `error`, and that message MUST name the
command that failed. It is an operator error of three lines. The first names
the command as `<command>: <class>`, where the class is `engine error` (the
dispatch failed, or the engine answered its error object), `engine unavailable`
(the engine answered nothing) or `unreadable reply` (the answer is not a JSON
object). The second says why there is no answer. The last is
`fix: ze show log recent level warn component lg.server`, the command that
shows the full reason, which the server logs. The message MUST NOT carry the
dispatcher's own error: this API answers anybody who can reach it, and that
text can name a socket path or another detail of the host.
<!-- source: internal/component/lg/server.go -- ask, engineRefusal, failureWhy -->

Before 2026-09-20 the endpoint asked for `bgp status`, which the daemon does not
serve. The dispatch failed on every request, and the endpoint answered 200 with
an empty `router_id` and an empty `version`. A client cannot tell that answer
from a router that has no identity, which is why the requirement above is a
refusal rather than an empty field.

`routes/table/{family}` and `routes/count/protocol/{name}` hold the same
requirement. Before 2026-10-08 both answered 200 over an engine error. The table
asked for `show bgp rib best <family>`, which `bgp-rib` refused because the
family is a filter (`show bgp rib best | family <family>`), and rendered the
refusal as an empty table. The count asked for `show bgp rib count peer <name>`,
which `bgp-rib` refused because nothing could follow a terminal, and rendered
the refusal as a count of 0. A count answer that carries no `count` is refused the
same way, and is never reported as 0.

`protocols/bgp`, `protocols/short`, `protocols/bmp` and `routes/prefix` hold
it too. Before 2026-10-14 the server turned a failed dispatch into the string
`{"error": ...}` and handed it on as the answer. It parsed, so these four
transformed the refusal into an empty protocols object or an empty routes
array and answered 200 over it. A failed dispatch is now an error the handler
receives in place of an answer.

`routes/protocol/{name}`, `routes/peer/{peer}`, `routes/export/{name}` and
`routes/bmp/{name}` answer a failed dispatch the same way, with 502 or 503.
They answer 404 when the engine's answer is an error object, which is how
`bgp-rib` refuses a command it cannot parse, and that body carries the
engine's own refusal: it is the engine's verdict on the request, not a
transport error.
<!-- source: internal/component/lg/server.go -- query, ask -->
<!-- source: internal/component/lg/handler_api.go -- engineAnswer, peerRoutesAnswer -->
<!-- source: internal/component/lg/handler_api.go -- handleAPIRoutesTable, handleAPIRoutesCount -->

### 3.2 Which direction each endpoint answers

`bgp-rib` stores both directions of a session: the routes the peer sent Ze
(its Adj-RIB-In, the `received` scope) and the routes Ze sent the peer (its
Adj-RIB-Out, the `sent` scope). A `show bgp rib` command that names no scope
answers `sent-received`, both at once, each row carrying its `direction`.

Owner decision, 2026-10-08: the looking glass shows what a peer sent Ze and
what Ze sent it, each in its own field. So every route endpoint MUST name its
scope and MUST NOT rely on the default.

| Surface | Scope | Command |
|---------|-------|---------|
| `routes/protocol/{name}`, `routes/peer/{peer}` | `received` | `show bgp rib \| received \| peer <address>` |
| `routes/count/protocol/{name}` | `received` | `show bgp rib \| received \| peer <address> \| count` |
| `routes/export/{name}` | `sent` | `show bgp rib \| sent \| peer <address>` |
| `routes/prefix`, `routes/search`, given a prefix with its length | `received` | `show bgp rib \| received \| exact <prefix>` |
| `routes/prefix`, `routes/search`, given a bare address | `received` | `show bgp rib \| received \| covering <address>` |
| `routes_received`, `routes_imported` (Section 5) | `received` | `route-counts.in` of `show bgp rib status` |
| `routes_exported` (Section 5) | `sent` | `route-counts.out` of `show bgp rib status` |
| UI peer page, its route rows, histogram and CSV download | `received` | `show bgp rib \| received \| peer <address>` |
| UI peer page, sent count | `sent` | `show bgp rib \| sent \| peer <address> \| count` |
| UI search, its route detail (`/lg/route/detail`), `/lg/graph` | `received` | `show bgp rib \| received \| ...` |

The UI peer page shows the received count and the sent count side by side. A
count the engine does not answer renders as an empty cell, never as `0`.

`{name}` in `routes/protocol`, `routes/count/protocol` and `routes/export` is
the protocol id `protocols/bgp` lists a session under: its configured name,
else its address. `bgp-rib` keys every session by its address, so each of the
three first asks `show bgp` for the session listed under that id and sends
bgp-rib the session's address. The session's address is accepted as an id
too, because an address names exactly one session and these endpoints answered
it before `protocols/bgp` listed configured names. When a configured name
spells another session's address, the name wins: it is the id
`protocols/bgp` lists. An id that is neither a listed name nor a session
address answers 404 naming it and `/api/looking-glass/protocols/bgp`, and no
rib command is sent. Before 2026-10-23 the name went to
bgp-rib as it was, matched no session, and every one of the three answered
200 with no routes, for a listed session and an unknown name alike.
<!-- source: internal/component/lg/handler_api.go -- protocolSession, protocolID -->

Before 2026-10-08 the per-peer list, its count, the peer page and the prefix
queries named no scope. A route Ze sent a session was listed under that session
with `learnt_from` naming it, and counted as learned from it.
<!-- source: internal/component/lg/handler_api.go -- serveRoutesForPeer, handleAPIRoutesExport, handleAPIRoutesCount -->
<!-- source: internal/component/lg/handler_ui.go -- handleUIPeerRoutes, ribCount, handleUISearch, handleUIRouteDetail -->
<!-- source: internal/component/lg/handler_graph.go -- handleGraph -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline.go -- parsePipelineOperations, scopeKeywords -->

### 3.3 The prefix lookup

`routes/prefix`, `routes/search`, the UI search and `/lg/graph` read their
prefix value with one rule, so the graph draws the routes the table lists and
the API answers what the page shows:

| Value | Selector sent | Rows answered |
|-------|---------------|---------------|
| A prefix with its length (`10.0.0.0/8`, `2001:db8::/32`) | `exact <prefix>`, its host bits cleared: `10.0.0.1/8` sends `exact 10.0.0.0/8` | The routes of exactly that prefix, never a longer prefix inside it |
| A bare address (`10.1.2.3`, `2001:db8::1`) | `covering <address>` | Every route whose prefix contains the address, the most specific prefix first within each peer |
| Anything else | none | 400, with an `error` that names the value, says why it is refused, and ends on a valid form |

The server MUST parse the value as an address or a prefix before it builds a
command, and the value it sends MUST be the text the parser writes for it, not
the text the client sent: a space, a pipe or a keyword in the parameter then
cannot reach the engine. An IPv4 length above 32 and an IPv6 length above 128
are refused, and so is an address with a zone, because no route prefix
carries one. A bare address answers routes of the address-keyed families only:
a route keyed by a route distinguisher has no single order of covering
prefixes.

Before 2026-11-18 both endpoints sent every value as `prefix <value>`, which
`bgp-rib` reads as a text match on the start of the prefix: `10.1.2.3` found a
`10.1.2.3/32` route and never `10.0.0.0/8`, while this table called
`routes/prefix` a containing lookup, and `192.0.0.0/2` also listed
`192.0.0.0/24`, whose text starts the same way.
<!-- source: internal/component/lg/prefix_lookup.go -- parsePrefixLookup, prefixLookup.receivedCommand -->
<!-- source: internal/component/lg/handler_api.go -- handleAPIRoutesPrefix -->

## 4. Response envelope

Every response MUST be a JSON object carrying an `api` member and exactly one
payload member.
<!-- source: internal/component/lg/handler_api.go -- apiEnvelope -->

```json
{
  "api": {
    "Version": "Ze Looking Glass",
    "result_from_cache": false
  },
  "protocols": {}
}
```

| Member | Type | Requirement |
|--------|------|-------------|
| `api.Version` | string | MUST be present. The capitalized spelling is REQUIRED, because it is birdwatcher's. Ze reports a product name; a client MUST NOT parse it as a semantic version |
| `api.result_from_cache` | bool | MUST be present. Ze MUST report `false`: it answers from live state and operates no response cache |

A response to a routes endpoint MUST carry `routes_count` beside the envelope,
giving the number of route objects in `routes`.

A client MUST ignore members it does not recognize. The server MAY add members in
a later revision, and MUST NOT treat that as a breaking change.

## 5. Protocol object

`protocols/bgp` and `protocols/short` MUST return an object keyed by protocol
name, whose values are protocol objects.
<!-- source: internal/component/lg/handler_api.go -- transformProtocols -->

`protocols/bmp` keys each BMP-monitored peer by the `rib-peer` that
`show bmp peers` reports, the key the RIB stores that peer's routes under.
`routes/bmp/{name}` passes the name unchanged to
`show bgp rib protocol bmp | peer <name>`, so every name `protocols/bmp` lists
is one `routes/bmp/{name}` answers for. The BMP plugin gives each monitored
peer its own key, so two peers never share a name (`bmpCompositeKey`). A peer
row is left out of `protocols/bmp` when it has no `rib-peer`, when an earlier
row already listed its `rib-peer`, or when `routes/bmp/{name}` would refuse
it; the handler counts each reason and logs one WARN line naming the three
counts. The name admits letters, digits, `-`, `_`, `.`, `:`, `[` and `]`,
which covers the bracketed IPv6 router address, plus a `%` zone closed by `]`,
as a link-local router address carries (`[fe80::1%eth0]:4000:...`), whose
characters are letters, digits, `-`, `_` or `.`. Any other name is refused
with 400, and the same check decides which rows `protocols/bmp` lists.
<!-- source: internal/component/lg/handler_api.go -- transformBMPProtocols, handleAPIBMPRoutes, isValidBMPPeerKey -->
<!-- source: internal/component/bgp/plugins/bmp/bmp.go -- bmpCompositeKey -->

| Member | Type | Requirement |
|--------|------|-------------|
| `bird_protocol` | string | MUST equal the key this object is stored under |
| `state` | string | MUST be present |
| `state_changed` | string | RFC 3339 timestamp of the last transition. MUST be present, MAY be empty |
| `neighbor_address` | string | MUST be present |
| `neighbor_as` | number | MUST be present. The `bgp { as-notation }` leaf does NOT reach it. The value is the AS number itself, read whatever notation wrote the peer row. A row carrying no readable AS number writes 0 and logs at WARN, because the field is mandatory (`setNeighborAS`, `internal/component/lg/handler_api.go`) |
| `description` | string | MUST be present, MAY be empty |
| `last_error` | string | MUST be present, and MUST be empty when there is no error. The value names the NOTIFICATION code and subcode that ended the session, for example "Cease/BFD Down". A reason ze built and could not write carries the " (not sent)" qualifier: RFC 9384 Section 4 asks for the reason in operational state even where the Cease could not be sent, and this schema holds one string, so the qualifier goes in it. `show bgp peer <address>` carries the same fact structured, as `last-notification.direction` |
| `table` | string | MUST be `master` for a BGP session and `bmp` for a BMP-monitored peer |
| `uptime` | number | Seconds. MUST be present |
| `routes_received` | number | See Section 7 |
| `routes_imported` | number | See Section 7 |
| `routes_exported` | number | See Section 7 |
| `routes_filtered` | number | See Section 7 |
| `routes_counts_available` | bool | See Section 7. This member is Ze's own and MUST NOT be expected from upstream |
| `routes` | object | MUST carry `imported`, `filtered`, `exported` and `preferred`, mirroring the four flat counts |

The server MUST emit both the flat counts and the nested `routes` object, because
clients differ in which they read.

## 6. Route object

Every routes endpoint MUST return an array of route objects under `routes`.
<!-- source: internal/component/lg/handler_api.go -- transformRoutes -->

| Member | Type | Requirement |
|--------|------|-------------|
| `network` | string | The prefix. MUST be present |
| `gateway` | string | Next hop. MUST be present |
| `metric` | number | MULTI_EXIT_DISC. MUST be present |
| `interface` | string | MUST be present. Ze MUST report the empty string: it does not attribute a route to an interface on this surface |
| `from_protocol` | string | The learning session. The server MUST substitute the peer address when one is known |
| `age` | number | Whole seconds since the route last changed. MUST be present. See Section 6.2 |
| `age_available` | bool | Whether `age` is a reading. See Section 6.2. This member is Ze's own and MUST NOT be expected from upstream |
| `learnt_from` | string | Peer address. MUST be present |
| `primary` | bool | Whether this is the best path. MUST be present. Every row of `routes/table/{family}` is `true`, because that endpoint answers only best paths. The received endpoints (`routes/protocol/{name}`, `routes/peer/{peer}`, `routes/prefix`, `routes/search`) read the `best` member of each `bgp-rib` row: `true` on the path the election recorded as its prefix's winner, `false` on every other path. A row of a family whose winner the RIB keys by wire NLRI (VPN, EVPN and the other non-CIDR families) carries no `best`, and `routes/export/{name}` rows are sent routes, which never carry one: `primary` is then `false` |
| `bgp` | object | MUST be present, as below |

### 6.1 The `bgp` member

| Member | Type | Attribute |
|--------|------|-----------|
| `origin` | string | ORIGIN |
| `as_path` | array of numbers | AS_PATH. The `bgp { as-notation }` leaf does NOT reach this field. It is a machine contract: a client that parses an integer must not fail because an operator changed a display preference (`asPathNumbers`, `internal/component/lg/handler_api.go`) |
| `next_hop` | string | NEXT_HOP |
| `local_pref` | number | LOCAL_PREF |
| `med` | number | MULTI_EXIT_DISC |
| `communities` | array | COMMUNITY, as `[asn, value]` pairs |
| `large_communities` | array | LARGE_COMMUNITY, as `[global, local1, local2]` triples |
| `ext_communities` | array | EXTENDED_COMMUNITY |

The server MUST emit `communities` and `large_communities` in the shapes above.
A client MUST NOT assume a flat list.

### 6.2 Route age and its availability

`age` is the number of whole seconds since `bgp-rib` last stored a change to
the route: since it was first learned, or since an announcement changed one of
its attributes. An announcement with equal attributes leaves it alone. The
server MUST copy the seconds the engine answered and MUST NOT compute an age
itself.

`bgp-rib` holds that time for the routes a peer sent, and writes it as the
`age` member of each such row when it answers. It writes no `age` on a row it
has no reading for:

| Endpoint | Rows | `age` in the engine's row |
|----------|------|---------------------------|
| `routes/protocol/{name}`, `routes/peer/{peer}`, `routes/prefix`, `routes/search` | Received routes | Present |
| `routes/export/{name}` | Routes Ze sent | Absent: the time a route was sent is not recorded |
| `routes/table/{family}` | Best paths | Absent: `bgp-rib` writes no `age` on a best path row, in the walk and under `\| json` alike. It records when the winning route last changed, not when it became best. |

The server MUST emit `age` on every route object, including one built from a
row with no reading, in which case it MUST be `0`. This follows the owner
decision Section 7.2 records for the route counts: on this compatibility
surface a member a client expects is kept, and the truth travels beside it.

Because that zero is not a measurement, the server MUST also emit
`age_available`:

- It MUST be `true` only when the engine's row carried an age the server could
  read as a whole number of seconds, zero included: a route stored this second
  has a true age of `0`.
- It MUST be `false` when `age` is the placeholder.
- The server MUST derive it from the VALUE, not from the presence of the key:
  a member holding a string, a negative number or a fraction is no reading.
- A client that shows an age MUST consult it, and MUST NOT read an `age` of `0`
  alone as "learned this second".

The route table of the UI reads the row through the same function, and shows
an empty cell where the API answers `age_available: false`.

An age MUST be expected to start again from zero when the daemon restarts: Ze
keeps the time in memory and learns every route again. An age is therefore at
most the time since the daemon started.

Before 2026-11-18 the RIB kept no time for a route and wrote no `age`, and the
server read the missing member as `0`: every route of every endpoint answered
`age: 0`, with nothing to tell it from a measurement.
<!-- source: internal/component/lg/handler_api.go -- transformRoutes -->
<!-- source: internal/component/lg/handler_ui.go -- routeAgeFrom, wholeNumber -->
<!-- source: internal/component/bgp/plugins/rib/rib_route_age.go -- writeRouteAge -->

## 7. Route counts and their availability

This section is the one place Ze knowingly diverges from upstream, and the
divergence is deliberate.

### 7.1 What upstream does

Upstream OMITS a count member entirely when BIRD reports the count as
unavailable. Its parser returns before writing anything when BIRD prints `---`:

```go
func setChangeCount(name string, value string, res Parsed) {
	if value == "---" { // field not available for protocol
		return
	}
	res[name] = parseInt(value)
}
```

Established by reading `alice-lg/birdwatcher`, `bird/parser.go`,
`setChangeCount`. Absence, upstream, means "no such number for this protocol".

### 7.2 What Ze does, and why

Ze MUST emit all four count members unconditionally, including when it has no
source for them, in which case each MUST be `0`.

This contradicts Section 7.1 on purpose. Owner decision, 2026-08-05: this is a
compatibility surface, Alice-LG's reaction to an absent count could not be tested
at the time, and a fabricated zero was judged the lesser risk against a client
that might render blank or error.

Because that zero is not a measurement, the server MUST also emit
`routes_counts_available`:

- It MUST be `true` only when the counts were produced from a real source.
- It MUST be `false` when the counts are placeholders.
- The server MUST derive it from the count VALUE, not from the presence of the
  count key. A key whose value the server cannot read as a number publishes the
  same fabricated zero as an absent key, so it is a placeholder.
- A client that distinguishes "no routes" from "unknown" MUST consult it, and
  MUST NOT infer either from a count of `0` alone.

<!-- source: internal/component/lg/handler_api.go -- routeCountsAvailable -->

The producer already draws this distinction and the server MUST preserve it:
`fetchRibRouteCounts` omits the count keys when the `bgp-rib` plugin is not
loaded, recording that they are "never faked to 0".
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- fetchRibRouteCounts -->

For a BMP-monitored peer the server MUST report `routes_counts_available` as
`false`: no source is consulted for those four members at all.
<!-- source: internal/component/lg/handler_api.go -- transformBMPProtocols -->

### 7.3 Known equalities and absences

`routes_exported` is the Adj-RIB-Out size for the session: the routes Ze sent
it, never added to the two counts below.

`routes_imported` MUST be expected to equal `routes_received`. Both are the
Adj-RIB-In size, because Ze drops rejected routes before storage and keeps no
separate pre-policy count. A client MUST NOT infer a policy drop from the
difference, which is always zero. BIRD reports received greater than or equal to
imported, so this is a real behavioral difference and not a naming one.
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- mergeRibRouteCounts -->

`routes_filtered` MUST be expected to be `0`. Ze retains no filtered routes: the
reject gate drops the route rather than storing it, so nothing can produce the
number. `routes/filtered/{name}` MUST return an empty array for the same reason,
and MUST do so with a success status rather than an error, because an empty
result is the honest answer.

Adding a filtered-route store is `plan/post-release/features/spec-bgp-filtered-route-storage.md`. Until
it lands, this section is the contract.

## 8. Divergence summary

A client written against upstream birdwatcher and pointed at Ze will observe
exactly these differences.

| # | Divergence | Deliberate |
|---|-----------|-----------|
| 1 | Unavailable counts are `0` rather than absent, with `routes_counts_available` carrying the truth | Yes, Section 7.2 |
| 2 | `routes_imported` always equals `routes_received` | Yes, Section 7.3 |
| 3 | `routes_filtered` is always `0` and `routes/filtered/{name}` is always empty | Yes, Section 7.3 |
| 4 | `interface` is always empty | Yes, Section 6 |
| 5 | `api.result_from_cache` is always `false` | Yes, Section 4 |
| 6 | `age` is `0` with `age_available` `false` on sent routes and best-path rows, and every age starts again when the daemon restarts | Yes, Section 6.2 |

## 9. Testing

`internal/component/lg/handler_api_test.go` gives `transformProtocols` a JSON
example of the engine's summary. The aggregates and peer rows are siblings,
the address key is `address`, AS numbers and route counts are numbers, and
uptime is a duration string. These types must survive the command boundary
before the looking glass translates them into the birdwatcher format.
<!-- source: internal/component/lg/handler_api_test.go -- realSummaryJSON, TestTransformProtocolsRealSummaryShape -->
<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- handleBgpSummary, mergeRibRouteCounts -->

`TestMockSummaryMatchesRegisteredProducer` runs the registered BGP overview
producer and compares its encoded payload with the handler golden's fixture.
The RIB-owned additions have a separate contract test: received and accepted
are equal numeric counts, sent is numeric, and filtered is absent.
`TestAPIProtocolsMockPreservesProducerNumbers` checks those values through the
public HTTP handler, including count availability and uptime in seconds.
<!-- source: internal/component/lg/summary_producer_test.go -- TestMockSummaryMatchesRegisteredProducer -->
<!-- source: internal/component/lg/summary_fixture_test.go -- TestMockSummaryRouteCountsAreProducerValues, TestAPIProtocolsMockPreservesProducerNumbers -->

The fixture must follow the producer. An earlier summary envelope mismatch
dropped every peer while the fixture stayed green.
`test/ui/lg-peer-table-flat-payload.ci` also drives the peer table over HTTP.
<!-- source: internal/component/lg/handler_api.go -- summaryPeers -->
<!-- source: test/ui/lg-peer-table-flat-payload.ci -- peer table HTTP checks -->

The same lesson applies to the RIB. `internal/component/lg/ribfake_test.go`
answers every `show bgp rib` command with the shape `bgp-rib` produces, and it
refuses the spellings the plugin's parser refuses. Before 2026-10-08 the fake
answered `show bgp rib best` with `{"routes":[...]}` whatever followed it, while
the plugin answers under `best-path`, each row naming its winner in `best-peer`
and nesting its attributes under `attributes`. The fake answers each scope
apart, and a command naming no scope gets both directions, as the plugin's
default does, so a caller that forgets its scope lists a route Ze sent. The best-routes table was empty on
every router and green in every test. `test/plugin/lg-best-table.ci` drives the
real `bgp-rib` and checks the table and the count over HTTP, so it is the check
that the fake still matches the plugin. `test/plugin/lg-received-sent.ci` drives
a session that both sends and receives routes, and checks that each endpoint in
Section 3.2 answers its own direction.

The fake parses every filter, and applies the ones a lookup test reads, each
with the plugin's meaning: `covering`, the rows whose prefix contains the
address, the most specific first within each peer; `exact`, the rows of that
one prefix; `prefix`, the rows whose prefix text starts with the value; and
`path` and `aspath`, the rows `matchASPath` keeps, the `path` value checked as
`validatePathPattern` checks it: a run anywhere in the path, at its start
after a leading `^`, at its end before a `$` that directly follows the last
AS number, and the whole path between both. A `$` anywhere else is refused,
as `parsePathPattern` refuses it. It refuses `covering` and `exact` where the
plugin does: on `show bgp rib best`, on every scope but `received`, together,
and with a value that is not the address or the masked prefix each takes. A
fake that accepted a keyword and returned its fixed rows would list a route
the question excludes, and a test of the lookup would stay green over a
plugin that ignored it. `test/plugin/lg-covering-lookup.ci` drives the real
`bgp-rib` with two peers holding nested prefixes, and checks the rows and
their order on the search page and on both endpoints of Section 3.3, a
`192.0.0.0/2` and `192.0.0.0/24` pair included. `test/plugin/lg-aspath-search.ci`
drives it with routes whose AS paths hold a run, its reverse and an anchor,
and checks what the search page lists for each. `test/plugin/lg-origin-search.ci`
does the same for the end anchor: routes one AS originates behind one AS,
behind two and prepended, and a route that only crosses it.

`TestTransformRoutesAgeAvailability` (`internal/component/lg/route_age_test.go`)
feeds `transformRoutes` rows with an age, with an age of zero and with none,
and reads `age` and `age_available` of each. `test/plugin/lg-route-age.ci`
drives the real `bgp-rib`: the `age` of a stored route grows, survives an
equal announcement, starts again after a changed one, and the route table
shows the same seconds.

The AS path is a field of the search page only. No endpoint of Section 3
takes one, so the end anchor adds nothing to this API.
<!-- source: internal/component/lg/ribfake_test.go -- fakeRIBPipeline, fakeRowSelected, fakeSelectedAnswer, fakePathPatternBody, fakeMatchASPath -->
<!-- source: internal/component/lg/handler_ui.go -- normalizeBestPathRows -->
