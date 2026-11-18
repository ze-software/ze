# Spec: lg-peer-history-origin-search-and-named-nodes

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P3 |
| Release reason | New looking glass capabilities for a route collector: session history, origin and exact AS path search, route age, and operator names on peers and next hops. None repairs shipped behavior; the repairs are in the companion fix spec |
| RFC gaps | - |
| Depends | spec-lg-search-matches-what-the-pages-promise.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The design below is a proposal. It was written without an approval gate and
waits for the owner's review before the status moves to `ready`. The companion
fix spec is `plan/release/fixes/spec-lg-search-matches-what-the-pages-promise.md`.

## Task

Ze can run as a route collector: every router of a network, or every member of
an exchange, holds a session to it, and the looking glass shows what each one
sent. The looking glass shows the present state of the sessions and the routes.
An operator who opens it during an incident also needs four things it does not
give.

| # | Need | Today |
|---|------|-------|
| 1 | Session history: how long a session has been down, how often it dropped, why it last closed | The engine publishes the last state change, the last error and the count of dropped connections for each peer. The peers page shows uptime only, so a session that is down shows nothing, and a session that drops every hour looks like one that has been up for an hour |
| 2 | Origin and exact AS path search: every route an AS originates, or one exact path | The engine matches a run of AS numbers anywhere, or anchored at the start. Nothing anchors the end, so "originated by" and "exactly this path" cannot be asked |
| 3 | Route age: when a route last changed | The RIB keeps no time for a route. The birdwatcher API has a mandatory `age` field, and it always reads zero |
| 4 | Names an operator recognizes: which router a peer is, and which exchange or transit a next hop belongs to | The peers page shows the peer's group name under "Description". The next-hop graph shows bare addresses. Its node type has a name field that nothing fills |

The goal: each of the four is visible in the page and, where the contract has a
field for it, in the API.

### Added 2026-11-18: a simulated network, and one trace from inside to outside

The owner asked for the looking glass to be proven on a network that behaves
like a real one, and for a prefix to be traceable through it.

| # | Need | Today |
|---|------|-------|
| 5 | A simulated network: about five routers of one AS, each a fake peer holding a real BGP session to Ze, plus more external neighbours on eBGP sessions. Code generates the topology and its routes, and computes for each speaker what that router would announce, with next hop rewritten to itself on the routes it passes to its iBGP neighbours | `lg-graph-lab` describes twelve routers by hand: thirty-six typed rows of peer, prefix, next hop and AS path, injected into the RIB through a command. No session exists for those routers, and the next-hop-self relationships are written by hand, not computed |
| 6 | One trace of a prefix: from each router, to the exit router it uses, to the external neighbour that announced the prefix, then through every AS of the path to the origin. And the list of exits announcing a prefix | The next-hop graph draws router to next hop and stops at the exit router. The AS-path graph draws the AS chain and starts at the first AS. Nothing joins the two, so the exit a path leaves by cannot be read beside the path it takes outside |

The trace shows each router's chosen exit, not the hop-by-hop route between
routers: that route belongs to the IGP, which a BGP collector does not see.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: what Ze does for an operator is what the operator configured. Names for next-hop ranges are written by the operator, never derived
  → Constraint: config is YANG-modeled and applied as a transaction. A looking glass setting that changes on reload follows the one reload path
- [ ] `ai/doors/api-surfaces.md` - the looking glass
  → Constraint: no RIB or peer import. Peer history and route age arrive through commands, in the JSON the engine already answers
  → Constraint: recorded trap, an `enabled` gate that discards settings. The new looking glass config is read whether or not `enabled` is set
  → Constraint: markup is templ, with no inline script or style
- [ ] `ai/doors/bgp-plugins.md` - `bgp-rib` and the peer command plugin
  → Constraint: the RIB and the peer command plugin can each run as a forked process. A route's time is stored by the plugin that stores the route
- [ ] `ai/doors/hub-engine.md` - where the looking glass is built
  → Constraint: one reload path. A changed looking glass list reaches the running server through it, as the certificate does
  → Constraint: always-on hub code never imports the looking glass package. The reload migrator holds the server through a small interface (`tlsUpdatable` for the certificate), and the named ranges need one of the same kind
- [ ] `docs/architecture/api/birdwatcher-compat.md` - the API contract
  → Constraint: `age` is a number of seconds and MUST be present. A value Ze cannot produce is the kind of field section 7.2 handles with an availability flag for counts
- [ ] `docs/architecture/plugin/rib-storage-design.md` - route storage
  → Constraint: a stored route is a small fixed record, and its size is paid once per route per peer. Any field added to it is a measured cost

**Key insights:**
- Item 1 needs no engine work: the three values are already in the peer summary.
- Item 3 is the only one that adds state, and it adds it per stored route. The record has unused padding after its three one-byte fields, where a four-byte time fits.
- The fix spec sets the AS path grammar the page speaks. This spec adds one anchor to that grammar, in the engine first.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - each peer row carries `state-changed`, `last-error`, `uptime`, `connections-dropped`, and `description`, which is set from the peer's group name. `lastErrorString` bounds the error text because it reaches the public looking glass
- [ ] `internal/component/lg/handler_ui.go` - `extractPeers` reads address, remote AS, state, uptime, three route counts, two update counts and `description`. It reads none of the history keys. `peerInfoFrom` reads `description` for the peer header. `routeRows` reads no age
- [ ] `internal/component/lg/view.go` - `peerRow`, `peerInfoRow` and `routeRow`, with no history, group or age field
- [ ] `internal/component/lg/peers.templ` - ten columns, none for history. The stream error row spans ten columns
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` reads `description`, `state-changed` and `last-error`. `transformRoutes` reads `age` from each RIB row
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `matchASPath`: a run anywhere, or anchored at the start with `^`. `showRows` writes each row from `serializeRouteItem`, with no age
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - `RouteEntry` holds three one-byte fields, then eight-byte identifiers, an attribute fingerprint and length, and two pool handles. It holds no time. The fingerprint is how an equal re-announcement is recognized
- [ ] `internal/component/lg/graph_nexthop.go` - `NextHopNode.Name` is documented as "Router name from decorator" and `buildNextHopGraph` never assigns it
- [ ] `internal/component/lg/layout_nexthop.go` - `formatNextHopNodeLabel` draws a named node as name then address, and the tooltip carries the name
- [ ] `internal/component/lg/yang/ze-lg-conf.yang` - the looking glass container holds the enable switch, listeners, TLS, certificate and token
- [ ] `internal/component/lg/server.go` - `LGConfig` and `LGServer`. The served certificate is held behind an atomic pointer so a reload replaces it
- [ ] `cmd/ze/hub/service_lg.go` - `buildLGService` fills `lg.LGConfig` from the service dependencies
- [ ] `cmd/ze/hub/listener_migrate.go` - `updateLGCertificate` applies a changed certificate to the running server through the `tlsUpdatable` interface

**Behavior to preserve:**
- Every existing column, count and endpoint of the looking glass
- The peer summary keys other readers use: `connections-dropped` is read by the interop lab checks, `state-changed` and `last-error` by the API transform
- The AS path forms that work after the fix spec: a run anywhere, and `^`
- The looking glass config that exists, and its behavior with no new list present
- Best-path selection: a route's age is information and never an input to it

**Behavior to change:**
- The peers page shows session history
- The engine accepts an end anchor in an AS path pattern
- The RIB records when each route last changed, and answers it
- The page and the API show that age
- An operator can name next-hop ranges, and the page uses those names
- The peers page shows the peer's own description, and its group apart

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The peer summary | `internal/component/bgp/plugins/cmd/peer/summary.go`, the row built for each peer | Publishes state, last change, last error, uptime, counters | Extends: the looking glass reads three keys it ignores today. One key is added for the group |
| The live peer table | `internal/component/lg/handler_ui.go`, `handleUIEvents` and `extractPeers`; `peers.templ` | Pushes the peer rows over server-sent events | Extends: the same rows carry the history columns, so they update live |
| AS path matching | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `matchASPath`, `validatePathPattern` | Start anchor and contiguous run | Extends: one end anchor in the same two functions |
| The page's AS path check | `internal/component/lg/handler_ui.go`, as the fix spec leaves it | Accepts the engine's grammar | Extends: follows the engine for the new anchor, through the shared test table the fix spec adds |
| The stored route record | `internal/component/bgp/plugins/rib/storage/routeentry.go`, `RouteEntry` | Per-route state beside the pooled attributes | Extends: one time value per route |
| Equal re-announcement detection | Same file, `AttrFingerprint` and `AttrLen` | Recognizes an announcement that changes nothing | Extends: the same test decides that the route's time stays |
| RIB row output | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `showRows` and `serializeRouteItem` | Writes each route as JSON | Extends: one `age` member |
| The API route transform | `internal/component/lg/handler_api.go`, `transformRoutes` | Already copies `age` from the row | Does not apply: unchanged. It starts carrying a real value |
| The AS name decorator | `internal/component/lg/server.go`, `ASNDecorator` | Names an AS from a DNS lookup | Does not apply to next hops: an address range has no public name source. The next-hop name comes from config |
| The next-hop node name | `internal/component/lg/graph_nexthop.go`, `NextHopNode.Name`; `layout_nexthop.go`, `formatNextHopNodeLabel` | A field and its rendering, never filled | Extends: this spec fills it |
| Looking glass config | `internal/component/lg/yang/ze-lg-conf.yang`; `cmd/ze/hub/service_lg.go`, `buildLGService` | Schema and extraction for the listener settings | Extends: one list in the same container, read by the same function |
| Certificate rotation on reload | `cmd/ze/hub/listener_migrate.go`, `updateLGCertificate`; `server.go`, the atomic certificate pointer | Applies one changed looking glass setting to the running server | Extends: the named ranges reach the running server the same way, behind their own small interface and their own atomic pointer |
| The hand-written graph lab | `internal/le/interoplab/bgp/lg_lab.go`, `runLGLab` and `lgInjections`; `test/plugin/lg-graph-lab.ci` | Injects 36 typed rows through `request bgp rib inject` and compares six graphs with saved SVG files | Replaces: the generated network supplies the routes over real sessions, and the checks come from the model. The saved-file comparisons go with it, by the test judge's route |
| The scenario generator | `internal/chaos/scenario/generator.go`, `Generate` and `GeneratorParams` | Builds a deterministic set of peer profiles from a seed: iBGP or eBGP, AS number, router id, address, port, route count | Extends: a topology is a new input beside the seed. The profiles keep their shape, and each one gains the routes the model computed for it |
| The peer simulator | `internal/chaos/peer/simulator.go`, `RunSimulator` and `SimulatorConfig` | Holds one real BGP session per simulated peer and announces generated routes | Extends: it announces a given route set per peer. Whether it does so today or announces only seed-generated routes is assumption A-8 |
| The scripted test peer | `le test peer`, driven by a `.ci` peer block | One scripted speaker with hand-written expectations | Does not apply: its routes are typed per test as hex or options, which is the hand-writing this work removes |
| The two graphs | `internal/component/lg/handler_graph.go`, `handleGraph`, `handleASPathGraph`, `handleNextHopGraph`; `graph.go`, `buildGraph`; `graph_nexthop.go`, `buildNextHopGraph` | One query for a prefix, rendered as an AS graph or as a router graph, chosen by `mode` | Extends: a third mode joins them over the same query answer. No second query and no new command |
| Graph text output | `handler_graph.go`, `format=text`; `renderGraphText`, `renderNextHopGraphText` | A deterministic text form of each graph, for tests | Extends: the joined graph has one too, and the model-derived checks read it |
| The peer `description` leaf | The BGP peer schema, "Basic peer identification" | A free-text label the schema says the CLI, the web UI and the logs show | Extends: the looking glass shows it. Where the label travels today is assumption A-4 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A browser or API client asks the looking glass for peers, routes or a graph
- An operator commits a config with named next-hop ranges
- A BGP UPDATE reaches `bgp-rib` and changes a stored route

### Transformation Path
1. On each stored change of a route, `bgp-rib` records the time on that route
2. A RIB query writes each row with its age in seconds, computed when the row is written
3. The peer summary answers each peer row with its history keys, its description and its group
4. The looking glass reads those keys and renders them, and passes `age` to the API transform
5. At start and on reload, the hub reads the named ranges from the config tree and hands them to the looking glass server
6. When the looking glass renders a next hop, in the table or the graph, it looks the address up in the named ranges and shows the name beside it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Looking glass to peer command plugin | `show bgp` summary through the dispatcher | No |
| Looking glass to `bgp-rib` | `show bgp rib` through the dispatcher | No |
| Config to looking glass | The hub's extraction at start and on reload | No |

### Changed-Wire Census

**Search evidence:** searched `internal`, `cmd` and `test`, Go, templ and `.ci` files, for readers of the summary keys `description`, `state-changed`, `last-error` and `connections-dropped`, for readers of a RIB row's `age`, and for writers of the `path` filter. Positive control: the search finds `internal/component/lg/handler_api.go` line 826 reading `description` and line 896 reading `age`. No shell or Python reader exists. The AS path writers are those listed in the fix spec's census.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Peer summary row, `description` | writer | `internal/component/bgp/plugins/cmd/peer/summary.go`, the row literal | Sets it from the group name | Change: carries the peer's description. A `group` key carries the group |
| Same | reader | `internal/component/lg/handler_ui.go`, `extractPeers` and `peerInfoFrom` | Both read `description` | Change: also read `group` |
| Same | reader | `internal/component/lg/handler_api.go`, `transformProtocols` | Copies `description` into the birdwatcher row | Preserve the key. Its content becomes the description, which is what the birdwatcher field means |
| Same | reader | `internal/component/bgp/plugins/cmd/peer/peer.go`, the column list at line 173 | Names `description` as a CLI column | Change: gains the group column |
| Same | reader | An operator or script reading `show bgp` | Public output | Change, stated in the news fragment: `description` no longer holds the group name |
| Peer summary row, history keys | writer | `summary.go`, the row literal | Writes `state-changed`, `last-error`, `connections-dropped` | Preserve |
| Same | reader | `internal/le/interoplab/bgp/names.go` and the checks that read `connections-dropped` | Read the key by name | Preserve |
| Same | reader | `internal/component/lg/handler_ui.go`, `extractPeers` | Reads none of them today | Change: reads all three |
| RIB row, `age` | writer | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `showRows` | Writes no `age` today | Change: writes it |
| Same | reader | `internal/component/lg/handler_api.go`, `transformRoutes` | Reads `age`, gets zero | Preserve the read |
| Same | reader | `internal/component/lg/handler_ui.go`, `routeRows` | Does not read it | Change: reads it for the route table |
| Same | reader | Every other consumer of RIB rows: CLI renderers, REST, MCP, third-party plugins | An added member | Preserve: additive. Golden outputs that pin a full row are refreshed with the judge's approval route |
| `path` filter value | writer | The looking glass search, the CLI, any API client | As in the fix spec | Change: may end with the end anchor |
| Same | reader | `rib_pipeline.go`, `matchASPath` and `validatePathPattern` | Start anchor only | Change: both learn the end anchor |
| Looking glass named ranges | writer | An operator's config | New list | New |
| Same | reader | `cmd/ze/hub/service_lg.go`, `buildLGService`, and the reload path in `listener_migrate.go` | Read the container today | Change: read the list |

### Integration Points
- `extractPeers` and `peersTableBody` - history columns
- `matchASPath` - the anchor
- The route insert path of `bgp-rib` - the time write
- `buildLGService` and the reload migrator - the named ranges

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | History and age arrive in command answers. Names arrive from config through the hub |
| No unintended coupling (components stay isolated) | Yes | The looking glass gains no import. `bgp-rib` learns nothing about the looking glass. The hub reaches the server through an interface |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every item extends a mechanism in the table above. The unused node name field is filled, not replaced |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The time is a value in the route record. Rows are written as today |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The list is in the looking glass's own schema and read by its own service builder |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | The CLI column list in `peer.go` gains `group` by hand, as it holds every other column by hand. Searched: the summary row, that column list, the looking glass readers |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A time of one-second resolution per route is enough | A looking glass shows ages in seconds, and the API field is seconds | A finer clock costs more bytes per route | Owner review | unvalidated |
| A-2 | A four-byte time fits in the route record's existing padding, so the record does not grow | `RouteEntry` opens with three one-byte fields followed by an eight-byte field, which leaves five unused bytes | The record grows by eight bytes per route, and AC-9 reports it | The size benchmark of AC-9, run before and after | unvalidated |
| A-3 | The time survives nothing: after a daemon restart every route is new | Routes are relearned from the peers at restart | Persisting it needs a store and is not wanted | Owner review | unvalidated |
| A-4 | The peer's `description` leaf does not reach the peer summary today | `summary.go` fills `description` from the group name, and the search found no other producer of the label | If it reaches it under another key, only the looking glass reader changes. If the label is dropped before the peer record, that part is a defect and moves to the fix spec | Tracing the leaf from the schema to the peer record before phase 5 | unvalidated |
| A-5 | Named ranges are few, tens at most | An exchange or a network has a handful of peering LANs and transit links | A long list needs an index | The lookup is a linear scan over a list whose length the schema bounds | unvalidated |
| A-6 | The end anchor is written `$` after the last AS number | The start anchor is `^`, and the page already allowed `$` before the fix spec | Another spelling changes two functions and the help | Owner review | unvalidated |
| A-8 | The peer simulator can be given the exact routes a peer announces, with their next hop and AS path | `SimulatorConfig` carries a seed and a route count, which reads as routes generated inside the simulator | The simulator gains a route-set input first, in this spec, before the network is built on it | Reading the sender in `internal/chaos/peer` before phase 6 | unvalidated |
| A-9 | Each internal router sends the collector its best path for every prefix, as it would to a route-reflector client | Proposed to the owner on 2026-11-18, not yet confirmed. A plain iBGP peer sends only its eBGP-learned routes, and the collector would then never learn which exit a non-border router uses | Under the other answer, only border routers appear in a trace, and need 6 shrinks to "which exits announce this prefix" | Owner review | unvalidated |
| A-10 | An exit router is recognized as the peer whose next hop for the prefix is itself, and its outside neighbour as the first AS of that route's path | `buildNextHopGraph` already marks that router as egress, and next-hop-self is what the simulated routers apply | A network that does not rewrite the next hop shows the neighbour's address as next hop, and the join has to match that address to an eBGP peer instead | A simulated border router without the rewrite, in phase 7 | unvalidated |
| A-11 | The simulated network and the joined trace belong in this post-release spec, not in the release fix | Proposed to the owner on 2026-11-18, not yet confirmed. The fix is proven by two small tests and is nearly written | If the owner wants the network as the fix's proof, phases 6 and 7 move to the fix spec with their criteria | Owner review | unvalidated |
| A-7 | The fix spec lands first | `Depends` in the header | The page-side AS path check does not exist yet in the form this spec extends | The fix spec's closure | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The per-route time grows the RIB's memory on a full-table collector | The size benchmark shows the record grew | AC-9 states the measured figure. If the record grows, the owner decides before phase 4 lands |
| R-2 | Writing the time on every UPDATE touches the hot path | The RIB insert benchmark slows | The clock is read once per received message, not per route |
| R-3 | A route that is re-announced unchanged looks new | Ages reset on every route refresh | The time moves only when the fingerprint test says the attributes changed. A test re-announces an equal route and expects the age to keep growing |
| R-4 | `description` changing meaning breaks a script | A user report | The change is named in the news fragment, and `group` carries the old value |
| R-5 | An operator names overlapping ranges | Two names for one next hop | The most specific range wins, and the commit refuses two entries with the same prefix |
| R-6 | The names are read only at start | A reload changes the list and the page keeps the old names | The reload migrator carries the list, with a functional test |
| R-7 | A last error text reaches a public page | Sensitive detail in a NOTIFICATION | `lastErrorString` already bounds this text for the public looking glass. The page shows it as given and adds nothing |
| R-8 | History columns make the peer table too wide for a phone | Horizontal scroll | The last error goes in the row's detail, not in a column. The stream error row's column span follows the new column count |
| R-10 | The generator and the looking glass share a mistake, so the test agrees with itself | A wrong trace that passes | The expected answers come from the topology model's own best-path computation, written apart from the looking glass and from the RIB. One hand-checked prefix is asserted literally beside the derived ones |
| R-11 | Many simulated sessions make the test slow or flaky on a loaded machine | Timeouts under repetition | The test waits for each speaker's end-of-RIB before it asks anything, and is proven under repeated runs before it is promoted |
| R-12 | A joined graph of a full path set is too large to read | The node cap is reached | The existing 100-node cap applies to the joined graph, with its message |
| R-13 | Removing the saved SVG comparisons loses a check on the drawing itself | A layout regression nobody sees | The unit captures of the two layouts stay. Only the lab's six saved files go, and the judge rules on that change |
| R-9 | A copied route record loses its time | An age of zero on a retained or cloned route | `RouteEntry.Clone` copies the field, with a test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Wrong ages or names on a read-only page, or more memory per stored route. No session or forwarding effect |
| How is it reverted? | A commit revert per phase. The config list is new, so a config that uses it stops validating after a revert |
| Who else touches this path? | The fix spec, `spec-bgp-per-peer-received-counter.md` and `spec-bgp-peer-state-fsm-and-reason.md` in `plan/release/fixes/` (peer summary), and `spec-bgp-filtered-route-storage.md` in `plan/post-release/features/` (the route record) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `GET /lg/peers` after a session dropped | → | `extractPeers` reading the history keys | `test/plugin/lg-peer-history.ci` |
| `POST /lg/search` with an end-anchored AS path | → | `matchASPath` | `test/plugin/lg-origin-search.ci` |
| `GET /api/looking-glass/routes/prefix` | → | the RIB's recorded time | `test/plugin/lg-route-age.ci` |
| A config with a named range, then `GET /lg/graph` in next-hop mode | → | `buildLGService` and the next-hop name lookup | `test/plugin/lg-named-next-hop.ci` |
| Simulated routers and neighbours open BGP sessions, then `GET /lg/graph` in the joined mode | → | the generated announcements, the RIB, and the joined graph builder | `test/plugin/lg-simulated-network.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A peer's session is closed by the remote side, then the peers page is read | Its row shows the state, the time of the change, a drop count of one, and its detail shows the last error | A peer that never dropped shows a count of zero, because zero is its true count |
| AC-2 | The same session comes back and drops again while the page stays open | The next message on the live event stream carries that peer's row with a drop count of two | Rows of other peers keep their own counts, because the stream sends every row as it is |
| AC-3 | The engine's summary carries no history keys for a row | The row renders with its other columns filled and the history cells empty | An empty cell is permitted and a zero is not, because the page never invents a count |
| AC-4 | AS path `65001$` | The result lists routes whose path ends with 65001 | A route crossing 65001 and originated elsewhere stays out, because the pattern is anchored at the end |
| AC-5 | AS path `^65010 65001$` | The result lists routes whose path is exactly those two AS numbers | A longer path containing the pair stays out |
| AC-6 | A path with the last AS prepended three times, searched with `65001$` | The route is listed | None: prepending does not change the origin |
| AC-7 | A route is learned, and thirty seconds later the page and the API are read | Both show an age of about thirty seconds | A difference of a few seconds is permitted, because the clock has one-second resolution |
| AC-8 | The same route is announced again with equal attributes | Its age keeps growing | An announcement that changes an attribute resets it, because the route changed |
| AC-9 | The route record size benchmark, before and after | Both sizes are stated in the spec's closure, and the growth is zero or eight bytes | Growth of eight bytes is permitted only with the owner's decision recorded here, because it is paid per stored route |
| AC-10 | A config names `198.51.100.0/24` as an exchange LAN, and a route has a next hop inside it | The route table shows the name beside the next hop, and the next-hop graph labels the node with it | A next hop in no named range shows its address alone |
| AC-11 | Two named ranges contain the next hop | The more specific one names it | None |
| AC-12 | Two entries name the same prefix | The commit is refused, naming both entries and the line to remove | Distinct prefixes commit, including nested ones |
| AC-13 | The named list changes and the daemon reloads | A page read after the reload shows the new name on the same next hop, served by the same listener | A page read before the reload completes shows the old name, because the list is replaced whole |
| AC-14 | A peer with a description and a group | The peers page shows the description, and the group in the peer's detail | A peer with no description shows an empty cell, because nothing is derived |
| AC-15 | The looking glass is started from environment variables with `enabled` unset in the config | The named ranges in the config are read and used | None |
| AC-16 | A topology of five routers in one AS and at least six external neighbours is given to the generator twice with the same seed | Both runs produce the same routers, neighbours, prefixes and per-speaker announcements | A different seed produces a different network, because the seed is the only source of variation |
| AC-17 | A border router learns a prefix from an external neighbour | The route it announces to the collector and to the other routers carries its own loopback as next hop and the neighbour's AS path unchanged | The route the external neighbour announces on its own eBGP session keeps the neighbour's address as next hop, because the rewrite is the border router's |
| AC-18 | A prefix is announced by two external neighbours attached to two different border routers | Each internal router announces one best path for it, whose next hop is the border router the model's best-path computation chose for that router | A router may choose either exit when the model ranks them equal, and the model states which one and why |
| AC-19 | The network is running against Ze | The peers page lists every router and every external neighbour as an established session, each with the route count the model computed for it | None: every simulated speaker holds a real session |
| AC-20 | The looking glass is searched for a prefix two exits announce | The result lists the external neighbours announcing it and the routers' paths toward each exit, and the model names the same set | Routers and neighbours that hold no route for the prefix stay out |
| AC-21 | The joined graph is asked for that prefix | It draws each router, an edge to the exit router it uses, an edge from each exit router to the external neighbour's AS, and the AS chain to the origin AS, matching the model's trace for every router | An AS that appears in no path for the prefix stays out |
| AC-22 | The joined graph is asked for a prefix only one neighbour announces | It draws one exit router, with every other router pointing to it, and one AS chain | None |
| AC-23 | The joined graph's text form | It lists nodes and edges in a fixed order, so two requests on the same state give the same text | The SVG form may differ in coordinates between versions, because the test reads the text |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens the peers page during an incident and sees which sessions dropped, when, and why | session event, peer summary, `extractPeers`, page | `test/plugin/lg-peer-history.ci` |
| 2 | Lists every route an AS originates | form, engine pattern, RIB, page | `test/plugin/lg-origin-search.ci` |
| 3 | Reads how long ago a route changed | UPDATE, RIB time, row, page and API | `test/plugin/lg-route-age.ci` |
| 4 | Names the exchange LANs once and reads next hops by name | config, hub, looking glass, table and graph | `test/plugin/lg-named-next-hop.ci` |
| 5 | Asks where a prefix leaves the network and what it crosses outside | simulated sessions, RIB, search, joined graph | `test/plugin/lg-simulated-network.ci` |
| 6 | Asks which external neighbours announce a prefix | simulated eBGP sessions, RIB, search | `test/plugin/lg-simulated-network.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractPeersReadsHistory` | `internal/component/lg/handler_ui_test.go` | The three keys reach the row, and a missing key stays empty | |
| `TestMatchASPathEndAnchor` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | End, both anchors, prepending | |
| `TestValidatePathPatternEndAnchor` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The validator and the matcher accept the same inputs | |
| `TestRouteAgeKeptOnEqualAnnounce` | `internal/component/bgp/plugins/rib/storage/peerrib_test.go` | Age survives an equal re-announcement and resets on a change | |
| `TestRouteEntryCloneKeepsTime` | `internal/component/bgp/plugins/rib/storage/routeentry_test.go` | A cloned record carries the time | |
| `TestRowCarriesAge` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The row member and its unit | |
| `TestRouteEntrySize` | `internal/component/bgp/plugins/rib/storage/routeentry_test.go` | Bytes per stored route record | |
| `TestNextHopNameMostSpecificWins` | `internal/component/lg/graph_nexthop_test.go` | Name lookup over nested ranges | |
| `TestSummaryCarriesDescriptionAndGroup` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | The two keys and their sources | |
| `TestTopologySameSeedSameNetwork` | `internal/chaos/scenario/topology_test.go` | The generator is deterministic | |
| `TestBorderRouterRewritesNextHop` | `internal/chaos/scenario/topology_test.go` | Next hop is the border router's loopback on what it announces inward, and the AS path is unchanged | |
| `TestRouterAnnouncesItsBestExit` | `internal/chaos/scenario/topology_test.go` | Each router's announced path for a prefix with two exits | |
| `TestExternalNeighbourKeepsItsNextHop` | `internal/chaos/scenario/topology_test.go` | The eBGP speaker's own announcement | |
| `TestTraceOfHandCheckedPrefix` | `internal/chaos/scenario/topology_test.go` | The model's trace for one prefix, asserted literally | |
| `TestJoinedGraphLinksExitToFirstAS` | `internal/component/lg/graph_joined_test.go` | The join of the router graph and the AS graph | |
| `TestJoinedGraphTextIsOrdered` | `internal/component/lg/graph_joined_test.go` | The fixed order of the text form | |
| `TestJoinedGraphRespectsNodeCap` | `internal/component/lg/graph_joined_test.go` | The cap and its message | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Named range prefix length, IPv4 | 0-32 | 32 | N/A | 33 |
| Named range prefix length, IPv6 | 0-128 | 128 | N/A | 129 |
| Named range count | 1-256 | 256 | N/A | 257 |
| Range name length | 1-64 | 64 | 0 | 65 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-peer-history` | `test/plugin/lg-peer-history.ci` | A session drops twice. The peers page shows the count, the time and the reason | |
| `lg-origin-search` | `test/plugin/lg-origin-search.ci` | A search anchored at the end lists only the routes that AS originates | |
| `lg-route-age` | `test/plugin/lg-route-age.ci` | The API answers a non-zero age for a route learned before the request | |
| `lg-named-next-hop` | `test/plugin/lg-named-next-hop.ci` | A named range labels the next hop in the table and the graph, and a reload renames it | |
| `lg-simulated-network` | `test/plugin/lg-simulated-network.ci` | Five simulated routers and their external neighbours hold sessions with Ze. For each prefix of the model, the peers page, the search and the joined graph answer what the model computed: the exits announcing it, each router's exit, and the AS chain outside | |

## Files to Modify
- `internal/component/lg/handler_ui.go` - history, age and group in the view rows
- `internal/component/lg/view.go` - the view fields
- `internal/component/lg/peers.templ`, `route_table.templ`, `route_detail.templ`, `help.templ` - the columns and their help
- `internal/component/lg/graph_nexthop.go`, `layout_nexthop.go`, `handler_graph.go` - the node name
- `internal/component/lg/server.go` - the named ranges held by the server, replaceable on reload
- `internal/component/lg/yang/ze-lg-conf.yang` - the named range list
- `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/listener_migrate.go` - extraction at start and on reload
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - the end anchor, the `age` member
- `internal/component/bgp/plugins/rib/storage/routeentry.go`, `peerrib.go`, `familyrib.go` - the route time
- `internal/component/bgp/plugins/cmd/peer/summary.go`, `peer.go` - description and group
- `docs/guide/looking-glass.md`, `docs/features/looking-glass.md`, `docs/architecture/web-interface.md` - the page as built
- `docs/architecture/api/birdwatcher-compat.md` - `age` and `description`
- `docs/architecture/plugin/rib-storage-design.md` - the route time and its cost
- `docs/architecture/api/commands.md` - the summary keys
- `docs/guide/configuration.md`, `docs/guide/cli.md` - the list and the anchor
- `internal/chaos/scenario/generator.go` - the topology as an input beside the seed
- `internal/chaos/peer/simulator.go` - a given route set per peer, if A-8 is broken
- `internal/le/interoplab/bgp/lg_lab.go`, `test/plugin/lg-graph-lab.ci` and its saved SVG files - replaced by the simulated network
- `docs/architecture/chaos-web-dashboard.md`, `docs/guide/chaos-testing.md` - the topology input and what the simulated routers announce

## Files to Create
- `internal/chaos/scenario/topology.go` - the topology model, its best-path computation and the per-speaker announcements
- `internal/component/lg/graph_joined.go` - the joined graph and its text form
- `test/plugin/lg-simulated-network.ci`
- `test/plugin/lg-peer-history.ci`
- `test/plugin/lg-origin-search.ci`
- `test/plugin/lg-route-age.ci`
- `test/plugin/lg-named-next-hop.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/lg/yang/ze-lg-conf.yang`: a list of named ranges under the looking glass container |
| YANG validation constraints | Yes | The prefix uses the shared prefix type, the name a length and a pattern, the list a maximum of 256 |
| YANG custom validators | Yes | One: no two entries with the same prefix, refused at commit with the line to remove |
| CLI commands/flags | No | No new command. `show bgp` gains a key, `show bgp rib` a row member and an anchor |
| CLI grammar (keyword before value) | N-A | No new keyword |
| Editor autocomplete | Yes | Automatic for the new leaves |
| Functional test for new RPC/API | Yes | The four `.ci` files under `test/plugin/` |
| Pipe completeness | N-A | No new command output path |
| Env var registration | No | A list has no environment form. The existing looking glass variables are unchanged |
| Doctor check for runtime dependencies | N-A | No new file, port or kernel dependency |
| Doctor code per problem class | N-A | The one refusal is at commit, where the editor shows it |
| Prometheus counters/metrics | No | No new metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/looking-glass.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the summary keys and the anchor |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, `docs/architecture/api/birdwatcher-compat.md` |
| 5 | Plugin added/changed? | No | No registration changes |
| 6 | Has a user guide page? | Yes | `docs/guide/looking-glass.md`, `docs/guide/looking-glass-howto.md` |
| 7 | Wire format changed? | No | No BGP bytes |
| 8 | Plugin SDK/protocol changed? | No | A row member is additive in the existing JSON |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | Four tests in an existing suite |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it has a looking glass row. Checked in phase 1 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/web-interface.md` |
| 13 | Route metadata keys added/changed? | No | The time is in the stored record, not a metadata key |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/features/spec-lg-peer-history-origin-search-and-named-nodes.md`. Declared pages already named above: `docs/architecture/web-interface.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/commands.md`. `ai/rules/plugins.md` is declared by `cmd/ze/hub/service_lg.go` and is unaffected: the looking glass stays a gated service built by its own factory |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The config example in `docs/guide/looking-glass.md` gains the list |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the four functional tests exist and fail
   - Tests: the four `.ci` files
   - Files: the schema list with no reader, the view fields with no source
   - Verify: each test fails on the missing value, not on a refusal
2. **Phase: Peer history** -- the page reads what the engine answers
   - Tests: `TestExtractPeersReadsHistory`
   - Files: `handler_ui.go`, `view.go`, `peers.templ`
   - Verify: `lg-peer-history` passes
3. **Phase: End anchor** -- engine first, then the page
   - Tests: `TestMatchASPathEndAnchor`, `TestValidatePathPatternEndAnchor`
   - Files: `rib_pipeline.go`, `handler_ui.go`, `help.templ`
   - Verify: `lg-origin-search` passes
4. **Phase: Route age** -- measured before it is kept
   - Tests: `TestRouteEntrySize` before the change, `TestRouteAgeKeptOnEqualAnnounce`, `TestRouteEntryCloneKeepsTime`, `TestRowCarriesAge`
   - Files: `routeentry.go`, `peerrib.go`, `familyrib.go`, `rib_pipeline.go`, `route_table.templ`
   - Verify: both record sizes are written into this spec before the phase is committed. `lg-route-age` passes
5. **Phase: Names** -- config, description and group
   - Tests: `TestNextHopNameMostSpecificWins`, `TestSummaryCarriesDescriptionAndGroup`
   - Files: the schema, `service_lg.go`, `listener_migrate.go`, `server.go`, the graph files, `summary.go`, `peer.go`
   - Verify: A-4 is settled first. `lg-named-next-hop` passes, including its reload step
6. **Phase: Simulated network** -- the model, then the sessions
   - Tests: the five `topology_test.go` tests, then `lg-simulated-network` up to the peers page and the search
   - Files: `topology.go`, `generator.go`, `simulator.go`
   - Verify: A-8 and A-9 are settled first. Every simulated speaker is an established session with the model's route count
7. **Phase: Joined trace** -- one graph from router to origin AS
   - Tests: the three `graph_joined_test.go` tests
   - Files: `graph_joined.go`, `handler_graph.go`, the graph buttons in `route_table.templ`, `help.templ`
   - Verify: A-10 is settled first. `lg-simulated-network` passes whole. The hand-written lab and its saved files are removed in this phase, after the judge's verdict

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each of the four needs is visible in the page, and age and description in the API |
| Correctness | Age resets only on a changed route. The most specific range names a next hop. An empty history cell is never a zero |
| Naming | JSON keys in kebab-case from the engine, snake_case only in the birdwatcher transform. The list and its leaves follow the looking glass schema's style |
| Data flow | The looking glass computes nothing about a route or a session. It reads and renders |
| Operator errors and doctor codes | The duplicate-prefix refusal names both entries and ends with the line to remove |
| Rule: explicit configuration | No name is derived from DNS, from a peer or from an address. A name exists only where the operator wrote one |
| Rule: measured cost | The record sizes are recorded figures from the same toolchain and architecture, before and after |
| Rule: gated service | No always-on file under `cmd/ze/hub` imports the looking glass package for the named ranges |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| History on the peers page | `./le test functional plugin` selecting `lg-peer-history` |
| End anchor | `./le test functional plugin` selecting `lg-origin-search` |
| Route age in the page and the API | `./le test functional plugin` selecting `lg-route-age` |
| Named next hops, live on reload | `./le test functional plugin` selecting `lg-named-next-hop` |
| Cost of the route time | `TestRouteEntrySize`, both figures in the closure |
| Pages state the built behavior | `./le doc check all` |
| No gated import from always-on code | `./le arch tier check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Range names are limited in length and characters by the schema. The end anchor adds one accepted character in one position |
| Output escaping | A range name and a peer description are operator text shown on a public page. Both go through templ escaping, and the graph builder escapes its labels as it does for AS names |
| Information exposure | The last error is the text the engine already bounds for this page. The peer description becomes public where the looking glass is public, and the guide says so |
| Resource exhaustion | The list is capped by the schema. The name lookup is bounded by that cap for each rendered next hop |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The peer summary already answers session history. The gap is entirely in what the page chooses to read.
- A mandatory API field with no producer reads as a measured zero. The contract page has a pattern for that, an availability flag, which this spec makes unnecessary for `age` by producing the value.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `constant-reported-as-measured-state.md` | 2026-11-18 | looking glass API `age` is always zero: the RIB keeps no route time |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The route time lives in the stored route record, at one-second resolution | A side table keyed by route. No age at all, with an availability flag in the API | A side table doubles the lookups and has to be kept in step on every insert and delete. The flag tells clients the truth and gives users nothing. The record has unused padding where the value fits, which AC-9 measures |
| Age resets only when the stored attributes change | Reset on every announcement | A route refresh would make a stable route look new, which is the opposite of what an age is for |
| Next-hop names come from a list the operator writes | Reverse DNS on the next hop. The name of the peer whose address equals the next hop | DNS names are slow to fetch and wrong for exchange LANs. A peer's name fits only when the next hop is the peer itself. A written list is explicit, and it covers transit links and exchange LANs alike |
| The list sits in the looking glass container | A system-wide list of named networks | Only the looking glass uses the names today. A wider home can be chosen when a second user exists |
| `description` carries the peer's description and `group` the group | Keep `description` as the group and add a new key for the label | The birdwatcher field and the schema both call the free-text label the description. Keeping the wrong meaning under the right name would keep the confusion in every consumer |
| The last error is in the row's detail | A column | The text is long, and the table must stay readable on a small screen |
| The simulated routers hold real BGP sessions | Keep injecting rows through the RIB command, as the hand-written lab does | Injection proves the query path only. Real sessions also prove reception, the per-peer counts and the session list, and they are what the owner asked for |
| The announcements are computed from a topology model | Type them per test | The hand-written lab holds 36 rows whose next-hop-self relationships a person had to get right. A model computes them, and the same model gives the expected answers |
| Proposed, awaiting the owner: internal routers send their best path for every prefix | Send only eBGP-learned routes, as a plain iBGP peer does | With the plain behaviour the collector cannot know which exit a non-border router uses, and no path can be traced inside the network |
| Proposed, awaiting the owner: a third graph mode joins the two existing graphs | Leave two graphs and let the reader match the exit router by eye. Replace both graphs with the joined one | Two graphs leave the join, which is the question asked, to the reader. Replacing them removes the two focused views operators already have |
| Proposed, awaiting the owner: this work sits in this post-release spec | Put it in the release fix as its proof | The fix is proven by two small tests and is nearly written. The network and the joined trace are larger and would hold it back |
| The trace stops at BGP: router, chosen exit, outside | Model the IGP to show hop-by-hop paths between routers | A collector sees BGP only. Drawing hops it cannot observe would publish a guess |
| No country or registry columns | Add them from the AS name source | They describe the AS, not the session, and an operator of a collector knows its peers. They can be added when asked for |

## Known Limitations

- Ages restart from zero when the daemon restarts, because the routes are relearned.
- A next hop is named only when an operator named its range. Nothing is guessed.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
