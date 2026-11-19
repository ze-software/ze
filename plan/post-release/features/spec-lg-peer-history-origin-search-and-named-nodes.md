# Spec: lg-peer-history-origin-search-and-named-nodes

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | config |
| Priority | P3 |
| Release reason | New looking glass capabilities for a route collector: session history, origin and exact AS path search, route age, and operator names on peers and next hops. None repairs shipped behavior; the repairs are in the companion fix spec |
| RFC gaps | - |
| Depends | - |
| Phase | 7/7 |
| Handoff | verify |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The design below was written as a proposal, without an approval gate. On
2026-11-18 the owner ordered the whole spec implemented, which accepts the
choices it marked as proposed: assumptions A-1, A-3, A-6, A-9 and A-11, and the
third graph mode. The work is developed in the worktree
`.claude/worktrees/lg-collector`, on the branch `lg-collector`, because the
session that implements it runs isolated from the shared checkout. The companion
fix spec, spec-lg-search-matches-what-the-pages-promise, closed on 2026-11-18:
the looking glass now sends `covering` for a bare address and `exact` for a
prefix with its length, and its AS path field takes AS numbers separated by
spaces with an optional leading `^` (`docs/guide/looking-glass.md`,
`docs/architecture/api/birdwatcher-compat.md` Section 3.3).

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
| 5 | A simulated network: about five routers of one AS, each a fake peer holding a real BGP session to Ze, plus more external neighbours on eBGP sessions. Code generates the topology and its routes, and computes for each speaker what that router would announce: to its iBGP neighbours, with the next hop rewritten where the topology says (a border router on what it learned outside, a route reflector on what it reflects), and to the collector, its chosen route, with the next hop rewritten to itself on what it learned outside and unchanged on what it learned inside | `lg-graph-lab` describes twelve routers by hand: thirty-six typed rows of peer, prefix, next hop and AS path, injected into the RIB through a command. No session exists for those routers, and the next-hop-self relationships are written by hand, not computed |
| 6 | One trace of a prefix, router by router: from a router, through each router of the AS that its BGP next hop names in turn, to the exit router, then through every AS of the path to the origin. And the list of exits announcing a prefix | The next-hop graph links a router to the peer whose address is its next hop, and stops at a router whose next hop is its own address. The AS-path graph draws the AS chain and starts at the first AS. Nothing joins the two, so the routers a path crosses inside cannot be read beside the path it takes outside |

Every router of the AS holds a BGP session to Ze, so Ze holds each router's
chosen route for the prefix and the BGP next hop that router uses. The trace
follows that chain. It starts at a router and takes its chosen route and next
hop. It finds the router of the AS whose own address is that next hop: its
session address with Ze, which the simulated topology makes the loopback the
router writes as next hop. It takes that router's chosen route for the same
prefix, and repeats until it reaches the exit router. The trace then continues
through every AS of the exit route's AS path to the origin. Where next hops are
rewritten inside the network (next-hop-self on iBGP, a route reflector), the
trace shows the real path through the core, router to router. What stays
invisible is only the IGP's hops between two consecutive BGP next hops, and the
page says so.

Each step of the walk reads one router's chosen route, and its next hop decides
what comes next. The page states every end, never silently.

| Next hop, or condition | Meaning | What the trace does |
|------------------------|---------|---------------------|
| The address of another router of the AS, not yet visited | The traffic crosses that router | Moves to it and takes its chosen route |
| The router's own address | It rewrote the next hop toward Ze, the normal next-hop-self case: this router is the exit | Leaves the AS at this router and continues with the AS path. The existing next-hop graph marks the same case as egress |
| The address of no router of the AS | The external neighbour at that address | Leaves the AS at this router toward that address and continues with the AS path, whose first AS names that neighbour. The neighbour needs no session with Ze; if it holds one, the page may show that session |
| The address of a router the walk already visited | A loop | Ends, and states the router and that the walk would loop |
| The address of a router of the AS that sent Ze no route for the prefix | The chain is broken at that router | Ends, and states that Ze holds no route from it for the prefix |
| The exit route's AS path is empty | The prefix originates inside the AS at the exit router | Ends at that router, and states that the prefix originates there |
| Ze holds several different routes from the router for the prefix (added in phase 7, end `several-routes`) | No row says which route that router chose, so its next hop is not known | Ends at that router, and states "Ze holds several routes from it for the prefix and no row says which one it chose" |
| The row Ze holds from the router has no next hop or AS path the trace can read (added in phase 7, end `route-unreadable`) | Skipping the row would draw a chain the data does not support | Ends at that router, and states "Ze holds a route from it, and its next hop or its AS path cannot be read" |

The visited-router guard bounds every walk by the number of routers of the AS,
so no separate hop limit is needed.

### Added 2026-11-18: small fixes found on the way

Each was found while implementing the needs above and is fixed in this work,
with its criterion, as `ai/META-DECISIONS.md`, "How agents work", requires of a
small problem.

| Fix | Where it was found | What was wrong | Criterion |
|-----|--------------------|----------------|-----------|
| The `aspath` spelling of the RIB filter is validated like `path` | Phase 3, the end anchor in `rib_pipeline.go` | The value written after `aspath` was never validated, so a malformed pattern answered no route with no refusal. `isPathStage` now sends both spellings through `parsePathPattern` | AC-39 |
| The peer `description` leaf reaches the peer record | Phase 5, tracing assumption A-4 | `parsePeerSettings` read no `description`, so the label was dropped at config parsing and never reached `show bgp`. Committed as 4e7f74f51f | AC-31 |
| `ExitConfirmsCleanup` is declared on the plugin registration | The first build of this branch | The branch's base did not build without it. The repair is commit 35d6480c53, which carries no behavior of this spec | None: a build repair, proven by the packages compiling |
| An empty trace says why | Phase 7, the first run against real sessions | The graph answered "No route found from a router of the AS" whatever the cause | AC-40 |
| The API says when it does not know a route's age | Phase 4, the page half | `transformRoutes` read a missing `age` as zero, which is the defect of the owned journal row under another producer | AC-41 |
| A connection held before the peers start follows its peer | The stress runs of `lg-simulated-unchanged-next-hop`: a simulated speaker read EOF where it expected an OPEN | A neighbor that connects before `StartPeers` waits in its peer's inbound slot. The rebuild that runs before the peers start (`RebuildRunningPeers`, `reconcilePeersJournaled`) replaced that peer with a new one, and `Peer.Stop` on a peer that never ran emptied nothing, so the socket was neither handed over nor closed. A product fix in the reactor, committed as 7d1a7b193f | AC-50, AC-51, AC-52 |
| A best-path row carries no age | The review of phase 4 | `bestSource.Next` took a route clock reading, so `show bgp rib best json` wrote `age` while the other form of the same row wrote none. The RIB records when a route last changed, never when it became best | AC-49 |
| An IPv4-mapped next-hop range is refused | The review of phase 5 | The looking glass reads an IPv4-mapped next hop as its IPv4 address, so a range written in the mapped form named nothing, and sat beside the IPv4 range of the same addresses without being refused as its duplicate | AC-44 |
| A rolled-back reload puts the previous next-hop names back | The review of phase 5 | The reload replaced the names before its last step that can fail, and its rollback did not put them back | AC-45 |
| The trace bounds the walks it lists | The review of phase 7 | The node cap does not bound the walks: a lookup by address answers every covering prefix, each walked from every router, so a graph under the cap could list thousands of lines | AC-46 |
| A recording names a commit that holds its code | The review of the replay | The recorder wrote a recording from a tree with uncommitted changes, and the site published it under the commit it named | AC-47 |
| The replay's frame document runs nothing when opened directly | The review of the replay | The frame document read the recording wherever it was opened, outside the sandbox its page gives it | AC-48 |

### Added 2026-11-18: a replay of the looking glass for the website

The owner asked: "provide me a way to share how it looks on the site not a
video a replay (JS HTML perhaps ??)", and answered the proposal below with "yes
build the replay for the site".

| # | Need | Today |
|---|------|-------|
| 7 | A way to share how the looking glass looks, on the website, as a replay and not a video | The site replays terminal sessions from captured events (`internal/le/site/terminaldemo`), and shows the web interface only as static WebP screenshots. The `labs/looking-glass-graph` page describes the lab in prose and shows no looking glass |

The looking glass itself is recorded, not its pixels. A recorder runs the
simulated network of need 5 against a real `ze`, asks the looking glass a
declared tour of questions over HTTP, and writes every answer (pages, htmx
fragments, a bounded sample of the live peer event stream, the graphs as SVG
and as text) with the looking glass's own stylesheet and scripts into one JSON
file. The website embeds that file in a sandboxed frame, where a small script
answers the requests the looking glass's own pages and scripts make. A visitor
clicks through the genuine interface on real markup and real data, and a tour
control replays the declared questions with a caption each. Only the recorded
questions have an answer: any other request shows a notice that names it.

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
| The page's AS path check | `internal/component/lg/handler_ui.go`, as the fix spec leaves it | Accepts the engine's grammar | Extends: `parseASPathPattern` (`aspath_pattern.go`) follows the engine for the new anchor. No shared test table exists (Mistake Log), so the page's table in `validate_test.go` restates the engine's accepted and refused forms |
| The stored route record | `internal/component/bgp/plugins/rib/storage/routeentry.go`, `RouteEntry` | Per-route state beside the pooled attributes | Extends: one time value per route |
| Equal re-announcement detection | Same file, `AttrFingerprint` and `AttrLen` | Recognizes an announcement that changes nothing | Extends: the same test decides that the route's time stays |
| RIB row output | `internal/component/bgp/plugins/rib/rib_pipeline.go`, `showRows` and `serializeRouteItem` | Writes each route as JSON | Extends: one `age` member |
| The API route transform | `internal/component/lg/handler_api.go`, `transformRoutes` | Already copies `age` from the row | Does not apply: unchanged. It starts carrying a real value |
| The AS name decorator | `internal/component/lg/server.go`, `ASNDecorator` | Names an AS from a DNS lookup | Does not apply to next hops: an address range has no public name source. The next-hop name comes from config |
| The next-hop node name | `internal/component/lg/graph_nexthop.go`, `NextHopNode.Name`; `layout_nexthop.go`, `formatNextHopNodeLabel` | A field and its rendering, never filled | Extends: this spec fills it |
| Looking glass config | `internal/component/lg/yang/ze-lg-conf.yang`; `cmd/ze/hub/service_lg.go`, `buildLGService` | Schema and extraction for the listener settings | Extends: one list in the same container. As built, the extraction is not in `buildLGService` (Mistake Log): `ExtractLGNextHopRanges` (`internal/component/config/lg_next_hop_ranges.go`) reads the list beside `ExtractLGConfig` (`loader_extract.go`), `runYANGConfig` (`cmd/ze/hub/main.go`) puts it in `serviceDeps`, and `buildLGService` hands it to `lg.LGConfig` |
| Certificate rotation on reload | `cmd/ze/hub/listener_migrate.go`, `updateLGCertificate`; `server.go`, the atomic certificate pointer | Applies one changed looking glass setting to the running server | Extends: the named ranges reach the running server the same way, behind their own small interface and their own atomic pointer |
| The hand-written graph lab | `internal/le/interoplab/bgp/lg_lab.go`, `runLGLab` and `lgInjections`; `test/plugin/lg-graph-lab.ci` | Injects 36 typed rows through `request bgp rib inject` and compares six graphs with saved SVG files | Does not replace it, extends beside it: the lab stays. Its proof that an AS name reaches a graph through the real decorator and a DNS answer (`le mock cymru`) has no other test, so removing it would lose that proof. The generated network adds what the lab cannot give: routes over real sessions, and checks computed by the model for the AS path graph, the next-hop graph and the trace graph |
| The scenario generator | `internal/chaos/scenario/generator.go`, `Generate` and `GeneratorParams` | Builds a deterministic set of peer profiles from a seed: iBGP or eBGP, AS number, router id, address, port, route count | Extends: a topology is a new input beside the seed. The profiles keep their shape, and each one gains the routes the model computed for it |
| The peer simulator | `internal/chaos/peer/simulator.go`, `RunSimulator` and `SimulatorConfig` | Holds one real BGP session per simulated peer and announces generated routes | Extends: it announces a given route set per peer. It announced only seed-generated routes (A-8, broken), so phase 6 added the route set: `scenario.RouteSet`, `SimProfile.RouteSet`, `(*Sender).BuildAnnouncement` |
| The scripted test peer | `le test peer`, driven by a `.ci` peer block | One scripted speaker with hand-written expectations | Does not apply: its routes are typed per test as hex or options, which is the hand-writing this work removes |
| The two graphs | `internal/component/lg/handler_graph.go`, `handleGraph`, `handleASPathGraph`, `handleNextHopGraph`; `graph.go`, `buildGraph`; `graph_nexthop.go`, `buildNextHopGraph` | One query for a prefix, rendered as an AS graph or as a router graph, chosen by `mode`. `buildNextHopGraph` links a router to the peer whose address is its next hop, and marks a router whose next hop is its own address as egress | Extends: a third mode joins them over the same query answer. Its walk uses the same address match, router by router, keeps the egress mark as the exit, and adds the visited-router guard and the stated ends. No second query and no new command |
| Graph text output | `handler_graph.go`, `format=text`; `renderGraphText`, `renderNextHopGraphText` | A deterministic text form of each graph, for tests | Extends: the trace graph has one too, and the model-derived checks read it |
| The peer `description` leaf | The BGP peer schema, "Basic peer identification" | A free-text label the schema says the CLI, the web UI and the logs show | Extends: the looking glass shows it. The label was dropped at config parsing (A-4), so this spec first carries it to the peer record (AC-31) |
| The terminal replay of the website | `internal/le/site/terminaldemo` (`./le site terminal-demo render`), `internal/le/site/demo.go` `playerMount`, `website/assets/js/site.js` `initTerminalDemoPlayers`; `website/AI.md`, "Site design and content rules" | An `le` action records a real session as text events with its provenance, the site build reads the recording and never runs it, and a player script replays it with a transcript link, a no-script fallback and no autoplay | Extends (need 7): the same three parts for a web interface. `./le site lg-replay record` is the recorder, in the same `le site` family. The `lg-replay` producer reads the committed recording, and `website/assets/js/lg-replay.js` is the player. It does not reuse the asciicast format or the container renderer, because a looking glass answers HTTP requests and is not a byte stream of a terminal |
| Recorded site data | `website/data/repo-facts.json`, written by `./le site facts update` and read by the build (`website/AI.md`, "Site build tooling") | A committed JSON file an action records and the build renders | Extends (need 7): `website/data/lg-replay.json` is recorded the same way, so a site build renders it and starts no daemon |
| Authored pages | `internal/le/site/authored.go`, `publishAuthoredPage` | Publishes each hand-authored page, `labs/looking-glass-graph/` included, through the shared shell | Extends (need 7): a page names a generated block with a `site-block` comment, and the block's owner registers what it expands to. The terminal marker (`demoMarker`, `demo.go`) is read only by the Markdown guide renderer, so it does not reach an authored page |
| The simulated network fixture | `internal/test/fixture/lg_simulated_network.go`, `lgSimulatedNetworkFixture` | Builds the model, starts `ze`, runs the speakers and asks the looking glass the model's questions | Extends (need 7): its network-starting part becomes `RunLGSimulatedNetwork`, which the fixture and the recorder both call, so the recording comes from the run the test proves |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A browser or API client asks the looking glass for peers, routes or a graph
- An operator commits a config with named next-hop ranges
- A BGP UPDATE reaches `bgp-rib` and changes a stored route
- A browser asks the looking glass for the trace of a prefix

### Transformation Path
1. On each stored change of a route, `bgp-rib` records the time on that route
2. A RIB query writes each row with its age in seconds, computed when the row is written
3. The peer summary answers each peer row with its history keys, its description and its group
4. The looking glass reads those keys and renders them, and passes `age` to the API transform
5. At start and on reload, the hub reads the named ranges from the config tree and hands them to the looking glass server
6. When the looking glass renders a next hop, in the table or the graph, it looks the address up in the named ranges and shows the name beside it
7. For the trace, the looking glass makes the one RIB query for the prefix and reads the peer list it already reads: each session's address and its `peer-type`. A peer the engine types internal is a router of the AS. The first design read the summary's global `local-as`, which is zero when the operator states the local AS per peer (Mistake Log)
8. From each router it follows the chosen route's next hop to the router whose session address it is, until the next hop is the router's own address or no router's address, then appends the AS path of the exit route. The table under need 6 gives each end and its statement

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
| Same | reader | As built: `internal/component/config/lg_next_hop_ranges.go`, `ExtractLGNextHopRanges`, called from `runYANGConfig` (`cmd/ze/hub/main.go`) at start, from `main_reload.go` on reload, and from the tree validator `lg-next-hop-range` (`internal/component/lg/register.go`). The spec first named `buildLGService`, which reads no config (Mistake Log) | New reader beside `ExtractLGConfig` | Change: read the list |
| Peer summary row, `peer-type` | writer | `internal/component/bgp/plugins/cmd/peer/summary.go`, `handleBgpSummary` | Writes `internal` or `external` for each peer, from the peer's own session | Preserve: unchanged by this spec |
| Peer summary row, `peer-type` | reader | `internal/component/lg/graph_trace.go`, `traceRoutersFrom` | New reader of an existing key | Preserve the key. Added in phase 7: it tells the routers of the AS |
| Looking glass route row, `age_available` | writer | `internal/component/lg/handler_api.go`, `transformRoutes` | New member beside `age` | New, additive: contract Section 6.2 |
| Looking glass route row, `age_available` | reader | A birdwatcher client outside the repository, Alice-LG among them | Upstream birdwatcher has no such member, so a client that does not know it ignores it and reads `age` as before | New, additive: a client that wants to tell an unknown age from zero reads it (`docs/architecture/api/birdwatcher-compat.md`, Section 6.2) |

### Integration Points
- `extractPeers` and `peersTableBody` - history columns
- `matchASPath` - the anchor
- The route insert path of `bgp-rib` - the time write
- `ExtractLGNextHopRanges`, `runYANGConfig`, `buildLGService` and the reload migrator - the named ranges
- `handleGraph` - the trace mode

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | History and age arrive in command answers. Names arrive from config through the hub |
| No unintended coupling (components stay isolated) | Yes | The looking glass gains no import. `bgp-rib` learns nothing about the looking glass. The hub reaches the server through an interface |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every item extends a mechanism in the table above. The unused node name field is filled, not replaced |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The time is a value in the route record. Rows are written as today |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The list is in the looking glass's own schema. As built it is read by `ExtractLGNextHopRanges` in the config package, beside the other looking glass extractors, and its refusal is a tree validator the looking glass registers (`lg-next-hop-range`) |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | The CLI column list in `peer.go` gains `group` by hand, as it holds every other column by hand. Searched: the summary row, that column list, the looking glass readers |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A time of one-second resolution per route is enough | A looking glass shows ages in seconds, and the API field is seconds | A finer clock costs more bytes per route | Owner review | confirmed by the owner on 2026-11-18. `storage.RouteTime` holds whole seconds |
| A-2 | A four-byte time fits in the route record's existing padding, so the record does not grow | `RouteEntry` opens with three one-byte fields followed by an eight-byte field, which leaves five unused bytes | The record grows by eight bytes per route, and AC-9 reports it | The size benchmark of AC-9, run before and after | confirmed on 2026-11-18: `TestRouteEntrySize` passes in the main thread's run of the storage package. `ChangedAt` sits at offset 4, `MsgID` stays at 8, and the record is 64 bytes before and 64 after, so the growth is zero |
| A-3 | The time survives nothing: after a daemon restart every route is new | Routes are relearned from the peers at restart | Persisting it needs a store and is not wanted | Owner review | confirmed by the owner on 2026-11-18. A reading is relative to the epoch the RIB takes when it starts (`routeClock`), so it cannot outlive the process |
| A-4 | The peer's `description` leaf does not reach the peer summary today | `summary.go` fills `description` from the group name, and the search found no other producer of the label | If it reaches it under another key, only the looking glass reader changes. If the label is dropped before the peer record, that part is a defect and moves to the fix spec | Tracing the leaf from the schema to the peer record before phase 5 | confirmed in its worse form on 2026-11-18: `parsePeerSettings` read no `description`, so the label was dropped before the peer record. The fix spec the "If wrong" column names had closed, so this spec carries the fix as AC-31 |
| A-5 | Named ranges are few, tens at most | An exchange or a network has a handful of peering LANs and transit links | A long list needs an index | The lookup is a linear scan over a list whose length the schema bounds | confirmed as built, with one difference: the lookup (`(*nextHopRanges).nameOf`) is a linear scan bounded by 256 entries, and the bound is enforced by the reader, `ExtractLGNextHopRanges` (`LGNextHopRangesMax`), not by the schema walk, which does not reach `environment` (`validatedSections`). `TestExtractLGNextHopRangesCountBoundary` and `TestNewNextHopRangesBounds` are written for 256 and 257, and their run is not yet recorded |
| A-6 | The end anchor is written `$` after the last AS number | The start anchor is `^`, and the page already allowed `$` before the fix spec | Another spelling changes two functions and the help | Owner review | confirmed: the owner's order of 2026-11-18 accepts it, and the engine reads it in `parsePathPattern` (`rib_pipeline.go`) |
| A-8 | The peer simulator can be given the exact routes a peer announces, with their next hop and AS path | `SimulatorConfig` carries a seed and a route count, which reads as routes generated inside the simulator | The simulator gains a route-set input first, in this spec, before the network is built on it | Reading the sender in `internal/chaos/peer` before phase 6 | broken on 2026-11-18: `(*Sender).BuildRoute` built every route from one fixed next hop and a path derived from the AS number. Phase 6b added the route set (`scenario.RouteSet`, `SimProfile.RouteSet`, `(*Sender).BuildAnnouncement`), which the "If wrong" column already planned, so the design holds. The chaos packages are green and the work is committed as d2c5426511. Mistake Log row 1 |
| A-9 | Each internal router sends the collector its best path for every prefix, as it would to a route-reflector client | Proposed to the owner on 2026-11-18, not yet confirmed. A plain iBGP peer sends only its eBGP-learned routes, and the collector would then never learn which exit a non-border router uses | Under the other answer, only border routers appear in a trace, and need 6 shrinks to "which exits announce this prefix" | Owner review: confirmed by the owner on 2026-11-18 | confirmed |
| A-10 | An exit router is recognized as the peer whose next hop for the prefix is itself, and its outside neighbour as the first AS of that route's path. A router is matched to a next hop by its session address with Ze | `buildNextHopGraph` already marks that router as egress, and next-hop-self is what the simulated routers apply. The simulated routers open their session to Ze from the loopback they write as next hop | A border router that does not rewrite the next hop shows the neighbour's address, which matches no router, and the walk leaves the AS there as well. A router whose next-hop address is not its session address would be read as an external neighbour, which is R-14 | A simulated border router without the rewrite toward Ze, in phase 7 | confirmed on 2026-11-18 by the draft `lg-simulated-unchanged-next-hop`, which passes against a real daemon: a hand-written network of three routers and two neighbours, one border router with `NextHopPolicyUnchanged`, whose walk ends `exit-external-address` at the neighbour's address. It passed on the second run. The first timed out with another session's process listening in the port range, which the fixture now refuses by name (`lgSimulatedPortsIdle`). `TestTraceWalkLeavesAtExternalNextHop` proves the same walk on RIB-shaped rows. One change to the first sentence as built: a router of the AS is a peer whose `peer-type` is internal (Mistake Log) |
| A-11 | The simulated network and the trace belong in this post-release spec, not in the release fix | Proposed to the owner on 2026-11-18, not yet confirmed. The fix is proven by two small tests and is nearly written | If the owner wants the network as the fix's proof, phases 6 and 7 move to the fix spec with their criteria | Owner review | confirmed: the owner's order of 2026-11-18 to implement the whole spec keeps the network and the trace here |
| A-7 | The fix spec lands first | `Depends` in the header | The page-side AS path check does not exist yet in the form this spec extends | The fix spec's closure | confirmed: spec-lg-search-matches-what-the-pages-promise closed on 2026-11-18, before phase 3 extended `parseASPathPattern` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The per-route time grows the RIB's memory on a full-table collector | The size benchmark shows the record grew | AC-9 states the measured figure. If the record grows, the owner decides before phase 4 lands |
| R-2 | Writing the time on every UPDATE touches the hot path | The RIB insert benchmark slows | The clock is read once per received message, not per route |
| R-3 | A route that is re-announced unchanged looks new | Ages reset on every route refresh | The time moves only when the fingerprint test says the attributes changed. A test re-announces an equal route and expects the age to keep growing |
| R-4 | `description` changing meaning breaks a script | A user report | The change is named in the news fragment, and `group` carries the old value |
| R-5 | An operator names overlapping ranges | Two names for one next hop | The most specific range wins, and the commit refuses two entries with the same prefix. As built: `nameOf` takes the longest matching prefix, and the tree validator `lg-next-hop-range` (`internal/component/lg/register.go`, over `ExtractLGNextHopRanges`) refuses one prefix written twice in masked form, such as `198.51.100.1/24` beside `198.51.100.0/24`, naming both entries and the line to delete. The same key written twice is refused earlier, by the parser's duplicate list key check |
| R-6 | The names are read only at start | A reload changes the list and the page keeps the old names | The reload migrator carries the list, with a functional test. As built: `main_reload.go` reads the list and `updateLGNextHopRanges` (`listener_migrate.go`) swaps it on the running server behind the `nextHopRangesUpdatable` interface, into an atomic pointer (`(*LGServer).UpdateNextHopRanges`). The draft `lg-named-next-hop` passes with its reload step on the connection opened before the reload |
| R-7 | A last error text reaches a public page | Sensitive detail in a NOTIFICATION | `lastErrorString` already bounds this text for the public looking glass. The page shows it as given and adds nothing |
| R-8 | History columns make the peer table too wide for a phone | Horizontal scroll | The last error goes in the row's detail, not in a column. The stream error row's column span follows the new column count |
| R-10 | The generator and the looking glass share a mistake, so the test agrees with itself | A wrong trace that passes | The expected answers come from the topology model's own best-path computation, written apart from the looking glass and from the RIB. One hand-checked prefix is asserted literally beside the derived ones |
| R-11 | Many simulated sessions make the test slow or flaky on a loaded machine | Timeouts under repetition | The test waits for each speaker's end-of-RIB before it asks anything, and is proven under repeated runs before it is promoted |
| R-12 | A trace graph of a full path set is too large to read | The node cap is reached | The existing 100-node cap applies to the trace graph, with its message |
| R-13 | Removing the saved SVG comparisons loses a check on the drawing itself | A layout regression nobody sees | Did not occur as built: the lab and its six saved files stay, because nothing else proves an AS name reaching a graph through the real decorator and DNS. `TestLGGraphGoldenOutput` (`graph_golden_test.go`) adds six whole-SVG captures of the three modes under `testdata/graph/`, which await the judge with the other captures |
| R-14 | A router writes as next hop an address other than its session address with Ze | Another router's walk leaves the AS at a router of the AS, read as an external neighbour | The simulated topology uses one address for both. The trace shows the address it left the AS by, and the guide states that a router is matched by its session address |
| R-9 | A copied route record loses its time | An age of zero on a retained or cloned route | `RouteEntry.Clone` copies the field, with a test |
| R-15 | The test files of this branch's base are stale: at the committed HEAD, the test files of the BGP packages do not compile against the committed production code | A scoped package test fails to compile on symbols this work never touched | The main thread ran the BGP package tests through a compiler overlay that reads the shared checkout's uncommitted test files, which match the committed production code. Every new test of this spec is in a file of its own (`routeentry_age_test.go`, `rib_pipeline_age_test.go` and the others the TDD plan names), so the overlay cannot drop one. A green recorded here is a green under that overlay, and the committed tree's own compile is CI's |
| R-16 | The branch was developed and tested in the worktree `.claude/worktrees/lg-collector`, not in the main checkout, because the session ran isolated. The rules ask for tests in the main checkout | A result that differs once the branch lands on `main` | Stated here and in the header so the closing session knows. Nothing of this spec has run in the main checkout. The site build is the one check that could not run on the branch at all: it refuses there for a reason that is not this spec's (the committed `data/dependencies.json` disagrees with the committed `go.mod`), so the replay page is first rendered when the site is built from the main checkout after the branch lands |
| R-17 | The reactor fix of AC-50 leaves one window open: a connection that arrives between the removal and the addition of the same peer, about two milliseconds in the evidence log, finds no peer | A simulated speaker reads EOF where it expected an OPEN, at a much lower rate than before the fix | The existing miss path closes that connection, so it is not left open, and a real neighbor retries. The simulator does not retry, so a stress run can still show the red. Closing the window needs the reactor to park arrivals for a peer under restart, a wider change that is not written |
| R-18 | A rolled-back reload may not put the looking glass certificate back either. The reviewer and the fix agent both read `runReloadContext` that way: its rollback restores the PKI store, and nothing hands the previous chain back to the listener. Unverified, and not this spec's change | After a reload that fails late, the looking glass serves the certificate of the refused config | Not fixed here. It is verified by a test that drives a reload with a changed certificate and a later failing step, then reads the certificate the listener serves. If it is confirmed it is a defect of the certificate update, fixed like the ranges of AC-45 |
| R-19 | On macOS a fixture could lose its last stderr line, the verdict, when it asked the daemon to stop: the stop kills the plugin's group at once there, and the line was still in the fixture's own stderr pump. Seen once in 35 runs of the `lg-aspath-search` draft | A functional test that ends with no `OK:` line and no failure line | `fixture.Run` calls `crashlog.Flush()` before the driver, so every fixture writes straight to the stderr its parent owns. Linux gives the plugin one second to exit, so the window was macOS's. No unit test can cover it, so its proof is the stress rerun. If the red returns, the loss is on the daemon's side |

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
| A config with a named range, then `GET /lg/graph` in next-hop mode | → | `ExtractLGNextHopRanges`, `buildLGService` and the next-hop name lookup | `test/plugin/lg-named-next-hop.ci` |
| Simulated routers and neighbours open BGP sessions, then `GET /lg/graph` in the trace mode | → | the generated announcements, the RIB, and the trace graph builder | `test/plugin/lg-simulated-network.ci` |
| A simulated border router that does not rewrite the next hop, then `GET /lg/graph` in the trace mode | → | the walk's exit at an address of no router (A-10) | `test/plugin/lg-simulated-unchanged-next-hop.ci` |

Every test of this table passes against a real daemon as a draft under
`test/draft/plugin/` (main thread, 2026-11-18). None is promoted to
`test/plugin/` yet: the stress runs come first.

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
| AC-18 | A prefix is announced by two external neighbours attached to two different border routers | Each internal router announces to the collector one best path for it, whose next hop is the one the model computed for that router: the border router of the exit the model's best-path computation chose, or the route reflector that rewrote the next hop on the way | A router may choose either exit when the model ranks them equal, and the model states which one and why |
| AC-19 | The network is running against Ze | The peers page lists every router and every external neighbour as an established session, each with the route count the model computed for it | None: every simulated speaker holds a real session |
| AC-20 | The looking glass is searched for a prefix two exits announce | The result lists the external neighbours announcing it and the routers' paths toward each exit, and the model names the same set | Routers and neighbours that hold no route for the prefix stay out |
| AC-21 | The trace graph is asked for that prefix | For each router it draws the chain of routers its BGP next hops cross, an edge from the exit router to the first AS of the exit route's path, and the AS chain to the origin AS, matching the model's trace for every router | An AS that appears in no path for the prefix stays out |
| AC-22 | The trace graph is asked for a prefix only one neighbour announces | It draws one exit router, every router's chain ends at it, and one AS chain | None |
| AC-23 | The trace graph's text form | It lists nodes and edges in a fixed order, so two requests on the same state give the same text | The SVG form may differ in coordinates between versions, because the test reads the text |
| AC-24 | The topology of AC-16 | At least one router is the client of a route reflector that rewrites the next hop on the routes it reflects, so its trace crosses the reflector before the exit router: three routers of the AS, then the AS path. The trace graph's text form for a prefix holds that chain as the model states it | None: the generator refuses a topology with no such router, because one-hop chains alone would not prove the walk |
| AC-25 | The trace graph is built from RIB rows where two routers name each other as next hop | Each walk ends at the revisited router, and the page states that router and that the walk would loop | None |
| AC-26 | A router's next hop is its own address | The walk leaves the AS at that router as the exit and continues with the AS path | None: this is the next-hop-self case, and it is drawn as an exit |
| AC-27 | A router's next hop is the address of no router of the AS, and the neighbour at that address holds no session with Ze | The walk leaves the AS at that router toward that address, and continues with the AS path, whose first AS names the neighbour | Where the neighbour holds a session with Ze, the page may also show that session, and nothing else changes |
| AC-28 | A router's next hop names a router of the AS that sent Ze no route for the prefix | The walk ends at the named router and states that Ze holds no route from it for the prefix | None |
| AC-29 | The trace graph page and its help | Both carry a note that the trace follows each router's BGP next hop, and that the IGP's hops between two consecutive next hops lie outside what it draws | None: the note is what tells a reader the chain is BGP's view |
| AC-30 | The exit route for a prefix has an empty AS path | The trace ends at the exit router and states that the prefix originates inside the AS at that router | None |
| AC-31 | A peer's `description` leaf is set, on the peer or on its group, then changed and the daemon reloads | The label reaches the peer record and the `description` key of `show bgp`, and the reload delivers the changed label to the running peer with the session kept | The peer object of a BGP event carries no description, because no event format has a member for it |
| AC-32 | `./le site lg-replay record` runs on a checkout with a built `ze` | It starts the simulated network of AC-19 against that `ze`, runs the fixture's checks, asks every question of the tour over HTTP, and writes `website/data/lg-replay.json` with the seed, the commit, the commit date, the tour, every answer, the looking glass's assets and a sample of the event stream | None: no answer in the file is written by hand |
| AC-33 | The replay page is open and each step of the tour is played | Every request the looking glass's pages and scripts make for a tour step is answered from the recording, and the page shows the recorded answer | None |
| AC-34 | A visitor searches for a prefix the recording does not hold | The page shows a notice that names the request and says it is not in the recording | The rest of the replay keeps working, because the notice replaces one answer only |
| AC-35 | The replay page is open and used | Every looking glass request the page makes is answered inside the page from the recording, and the tour plays to its last step with the recorded answers. The frame's content security policy allows no connection, and the web server's log holds no `/lg/` request | The frame document and the player script are fetched once, because they are the page |
| AC-36 | The recorder runs twice on the same commit | The two files are byte-identical. Session uptimes, state-change times and the listener address are replaced by the fixed values the file's `normalized` list declares, and an answer that still changed between two reads ten seconds apart is named in the file's `varies` list | A file with a non-empty `varies` list is permitted and says exactly which answers differ run to run |
| AC-37 | The replay page is read with JavaScript disabled | It shows the tour as a list and a link to the text transcript, and no frame | None |
| AC-38 | The site is built on a checkout that holds no recording | The replay block says the looking glass has not been recorded yet, and the build succeeds | None: a site build never starts a daemon to make one |
| AC-39 | `show bgp rib` with the filter written `aspath` and a malformed pattern, then with a valid one | The malformed pattern is refused with the refusal `path` gives for it, and the valid one filters as `path` does (`TestParsePipelineValidatesASPathAlias`) | None: the two spellings are one filter |
| AC-40 | The trace graph is asked for a prefix and no walk can start | The answer is one line that says which of three causes applies: the peer summary names no router of the AS, the query answered no route for the prefix, or no answered route came from a router of the AS. It ends with the three counts it was decided on: peers, routers of the AS, routes answered (`TestTraceGraphSaysWhyItHasNoTrace`) | None: an empty graph with no reason reads as "no route", which is one cause of three |
| AC-41 | The API answers a route row whose age the engine does not know: a sent row, a best-table row, or a route stored with no reading | The row carries `age` 0 and `age_available` false. A row with a known age carries its seconds and `age_available` true, a known zero included (contract Section 6.2, `TestTransformRoutesAgeAvailability`) | `age` stays present, because the birdwatcher contract makes it mandatory. The flag beside it carries the truth |
| AC-42 | The trace graph is built from RIB rows where Ze holds several different routes from one router for the prefix | The walk ends at that router with the end `several-routes`, and states that Ze holds several routes from it for the prefix and no row says which one it chose (`TestTraceWalkStopsOnSeveralRoutes`) | None: choosing one of them would draw a guess |
| AC-43 | The trace graph is built from RIB rows where the row from one router has no next hop or AS path the trace can read | The walk ends at that router with the end `route-unreadable`, and states that Ze holds a route from it, and that its next hop or its AS path cannot be read (`TestTraceWalkStopsOnUnreadableRoute`) | None: skipping the row would state that the router holds no route, which is false |
| AC-44 | A config names a next-hop range as an IPv4-mapped IPv6 prefix, such as `::ffff:198.51.100.0/120` | The commit is refused: the message names the entry, says the looking glass reads an IPv4-mapped next hop as its IPv4 address, and names the IPv4 range to write in its place, `198.51.100.0/24` for that entry (`TestExtractLGNextHopRangesRefusesMappedRange`). `newNextHopRanges` refuses the same range | A mapped range shorter than 96 bits is refused with no IPv4 range named, because it is wider than the whole IPv4 space |
| AC-45 | A reload replaces the next-hop names, and a later step of the same reload fails and rolls it back | The looking glass shows the names it showed before the reload (`TestUpdateNextHopRangesRestoresTheReplacedList`, `TestUpdateLGNextHopRangesRestoresOnRollback`) | None |
| AC-46 | The trace graph is asked for an address whose covering prefixes give more than 100 walks | The text form writes 100 `trace` lines, then one `unlisted traces=<n>` line with the count left out and the sentence the drawing shows under its lines. Every walk still gives its nodes and edges to the graph (`TestTraceGraphBoundsTheWalksItLists`) | A graph of 100 walks or fewer writes no such line |
| AC-47 | `./le site lg-replay record` runs on a checkout with uncommitted changes, without and then with `trial`. Then the site is built from a recording whose `tree-dirty` is true | Without `trial`, nothing is recorded: the refusal counts the changes, names the first of them, and ends with the command that records a trial. With `trial`, the file is written with `tree-dirty` true. The site build refuses to publish that file and names `./le site lg-replay record` (`TestUncommittedChangesAreRefusedUnlessTrial`, `TestRecordDeclaresTheTrialKeyword`, `TestLGReplayRefusesARecordingOfADirtyTree`) | None: a published recording names a commit, and that commit holds the code that answered |
| AC-48 | The replay's frame document is opened directly in a browser, outside the page that frames it, or framed without the sandbox | It decides before it reads the recording: no recorded markup is written and no recorded script runs, and its root carries `data-lg-replay-frame="refused: ..."` with the reason (`TestLGReplayFrameOpenedDirectlyRunsNothingRecorded`) | The document keeps its one paragraph, which names the page that plays it |
| AC-49 | `show bgp rib best`, in its table form and with `json` | A best-path row carries no `age` member in either form (`TestBestWalkRowCarriesNoAge`, `TestBestJSONTerminalRowCarriesNoAge`) | None: the RIB records when a route last changed, never when it became best |
| AC-50 | A neighbor connects after the listeners open and before the peers start, and the rebuild before the start replaces its peer with a new one | The connection moves to the replacement peer, which runs the session on it (`TestHeldInboundMovesToTheReplacedPeer`) | A connection that arrives between the removal and the addition of the same peer finds no peer and is closed by the existing miss path (R-17) |
| AC-51 | The same held connection, and the rebuild removes its peer for good | Ze sends a NOTIFICATION, Cease with subcode Connection Rejected (RFC 4486), then closes the connection (`TestHeldInboundIsRejectedWhenThePeerIsRemoved`) | None |
| AC-52 | The same held connection, and the reactor stops before its peers start | The neighbour's read on that connection ends with end of file, within the test's bound: Ze closed the TCP connection, and the neighbour received no NOTIFICATION bytes before the close (`TestHeldInboundIsClosedWhenTheReactorStopsBeforeItsPeers`) | The close is silent because the same path serves a stop for restart, which owes silence |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens the peers page during an incident and sees which sessions dropped, when, and why | session event, peer summary, `extractPeers`, page | `test/plugin/lg-peer-history.ci` |
| 2 | Lists every route an AS originates | form, engine pattern, RIB, page | `test/plugin/lg-origin-search.ci` |
| 3 | Reads how long ago a route changed | UPDATE, RIB time, row, page and API | `test/plugin/lg-route-age.ci` |
| 4 | Names the exchange LANs once and reads next hops by name | config, hub, looking glass, table and graph | `test/plugin/lg-named-next-hop.ci` |
| 5 | Asks which routers of the network a prefix's traffic crosses, where it leaves, and what it crosses outside | simulated sessions, RIB, search, trace graph | `test/plugin/lg-simulated-network.ci` |
| 6 | Asks which external neighbours announce a prefix | simulated eBGP sessions, RIB, search | `test/plugin/lg-simulated-network.ci` |

## Goal Validation

The evidence each goal of the Task section owes. The closure records each named test's output beside its row.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Need 1: session history on the peers page | functional | `test/plugin/lg-peer-history.ci`: a real session dropped twice, and the page read for its drop count, time of change and last error (AC-1, AC-2) |
| Need 2: origin and exact AS path search | functional | `test/plugin/lg-origin-search.ci`: an end-anchored search through the page lists only the routes that AS originates, with a crossing route kept out (AC-4 to AC-6) |
| Need 3: route age in the page and the API | functional, with explicit JSON | `test/plugin/lg-route-age.ci`: the API's `age` is non-zero for a route learned before the request, and keeps growing over an equal re-announcement (AC-7, AC-8). `TestRouteEntrySize` gives the record size before and after (AC-9) |
| Need 4: names an operator recognizes | functional | `test/plugin/lg-named-next-hop.ci`: a named range labels the next hop in the table and the graph, and a reload renames it on the same listener (AC-10, AC-13). The peers page shows the description and the group (AC-14) |
| Need 5: a simulated network over real sessions | functional | `test/plugin/lg-simulated-network.ci`: every simulated router and neighbour is an established session on the peers page, each with the route count the model computed (AC-19). `TestTopologySameSeedSameNetwork` shows the generator is deterministic (AC-16) |
| Need 6: one trace from router to origin | functional, with explicit text output | `test/plugin/lg-simulated-network.ci`: for each prefix of the model, the trace graph's text form equals the model's trace for every router, including a chain across the route reflector (AC-21, AC-24). `TestTraceOfHandCheckedPrefix` asserts one trace literally, so the model and the page cannot agree on a shared mistake (R-10). The `graph_trace_test.go` tests cover each end of the walk (AC-25 to AC-28, AC-30) |
| Need 7: a replay of the looking glass on the website | recorded run, then a browser test | `./le site lg-replay record` writes the recording from the simulated network (AC-32, AC-36); `TestLGReplayAnswersTheTourFromTheRecording` in `internal/le/site/browsertest/lgreplay_test.go` plays the tour in a browser and reads the web server's request log (AC-33, AC-34, AC-35) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractPeersReadsHistory` | `internal/component/lg/handler_ui_test.go` | The three keys reach the row, and a missing key stays empty | passing |
| `TestPeerStreamCarriesHistory`, `TestPeersTableRowsMatchTheHeader`, `TestPeerInfoFromReadsHistory` | `internal/component/lg/handler_ui_test.go` | The live stream carries the history cells (AC-2), every row has as many cells as the header (R-8), and the peer header shows the history only when present (AC-3) | passing |
| `TestMatchASPathEndAnchor` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | End, both anchors, prepending | passing |
| `TestValidatePathPatternEndAnchor` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The validator and the matcher accept the same inputs | passing |
| `TestParsePipelineValidatesASPathAlias`, `TestShowPipelinePathEndAnchor` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | The `aspath` spelling is validated like `path` (AC-39), and the anchor filters the rows of `show bgp rib` | passing |
| `TestParseASPathPattern`, `TestSearchSendsTheEndAnchor`, `TestSearchRefusesUnsupportedPathCharacters` | `internal/component/lg/validate_test.go`, `handler_ui_test.go` | The page accepts one `$` after the last AS number, sends it to the engine, and refuses it anywhere else. The changed refusal rows of the two existing tests await the judge | passing |
| `TestRouteAgeKeptOnEqualAnnounce` | `internal/component/bgp/plugins/rib/storage/routeentry_age_test.go` (planned in `peerrib_test.go`; a file of its own because of R-15) | Age survives an equal re-announcement and resets on a change | passing |
| `TestRouteEntryCloneKeepsTime` | `internal/component/bgp/plugins/rib/storage/routeentry_age_test.go` (planned in `routeentry_test.go`) | A cloned record carries the time | passing |
| `TestRowCarriesAge` | `internal/component/bgp/plugins/rib/rib_pipeline_age_test.go` (planned in `rib_pipeline_test.go`) | The row member and its unit | passing |
| `TestRouteEntrySize` | `internal/component/bgp/plugins/rib/storage/routeentry_age_test.go` (planned in `routeentry_test.go`) | Bytes per stored route record: 64, with `ChangedAt` at offset 4 and `MsgID` at 8 (AC-9) | passing |
| `TestRouteAgeOnEveryInsertPath`, `TestRouteAgeKeptWhenAttributeBytesDifferAndValuesAreEqual`, `TestRouteStoredWithoutReadingHasNoAge`, `TestRouteTimeSince`, `TestRouteTimeAgeAt` | `internal/component/bgp/plugins/rib/storage/routeentry_age_test.go` | Every insert path stamps the route, equal values under different bytes keep the time, and a route stored with no reading has no age | passing |
| `TestInjectedRouteAgeKeptOnEqualInject`, `TestReceivedUpdateStampsItsRoutes`, `TestPoolReceivedRouteHoldsTheReading`, `TestUnparsedLabeledRouteHoldsTheReading`, `TestRouteAgeWrittenOnlyWhenKnown`, `TestManagerBuiltWithoutConstructorHasNoRouteClock` | `internal/component/bgp/plugins/rib/rib_pipeline_age_test.go` | Each way a route reaches the RIB holds the reading, and a row with no reading has no `age` member | passing |
| `TestRouteRowsReadTheAge`, `TestRouteRowsKeepAZeroAge`, `TestRouteRowsLeaveAMissingAgeEmpty`, `TestFormatElapsed`, `TestTransformRoutesAgeAvailability` | `internal/component/lg/route_age_test.go` | The pages show the age, a known zero stays a zero, a missing age stays empty, and the API states `age_available` (AC-7, AC-41) | passing |
| `TestNextHopNameMostSpecificWins` | `internal/component/lg/graph_nexthop_test.go` | Name lookup over nested ranges | passing |
| `TestNewNextHopRangesBounds`, `TestUpdateNextHopRangesReplacesTheList`, `TestRouteTableShowsNextHopName`, `TestPeersPageShowsDescriptionAndGroup`, `TestNextHopRangeValidatorRefusesOneRangeTwice`, `TestNextHopRangeValidatorIsRegistered` | `internal/component/lg/nexthop_ranges_test.go` | The 256 bound, the swap on reload (AC-13), the name in the route table (AC-10), the description and group on the peers page (AC-14), and the refusal reaching the registered validator (AC-12) | passing |
| `TestExtractLGNextHopRangesIgnoresEnabled`, `TestExtractLGNextHopRangesNoList`, `TestExtractLGNextHopRangesRefusesOneRangeTwice`, `TestExtractLGNextHopRangesAcceptsNestedRanges`, `TestExtractLGNextHopRangesCountBoundary`, `TestExtractLGNextHopRangesPrefixLengthBoundary`, `TestExtractLGNextHopRangesRefusesEntryWithoutName`, `TestLGNextHopRangeSchemaBoundaries` | `internal/component/config/lg_next_hop_ranges_test.go` | The reader ignores `enabled` (AC-15), refuses one range written twice and accepts nested ones (AC-12), and holds the boundaries of the table below | passing |
| `TestUpdateLGNextHopRangesReplacesTheList`, `TestUpdateLGNextHopRangesWithoutLookingGlass`, `TestUpdateLGNextHopRangesReportsRefusal` | `cmd/ze/hub/listener_migrate_ranges_test.go` | The reload seam hands the list to the running server (AC-13, R-6) | passing |
| `TestSummaryCarriesDescriptionAndGroup` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | The two keys and their sources | passing |
| `TestParsePeerFromTreeDescriptionReachesPeerSettings` | `internal/component/bgp/reactor/config_test.go` | The `description` leaf reaches `PeerSettings`, and an absent leaf leaves it empty (AC-31) | passing |
| `TestReloadDescriptionReachesRunningPeerWithoutRestart` | `internal/component/bgp/reactor/peer_settings_swap_test.go` | A changed label reaches the running peer and the peer snapshot with the session kept (AC-31) | passing |
| `TestTopologySameSeedSameNetwork` | `internal/chaos/scenario/topology_test.go` | The generator is deterministic | passing |
| `TestBorderRouterRewritesNextHop` | `internal/chaos/scenario/topology_test.go` | Next hop is the border router's loopback on what it announces inward and to the collector, and the AS path is unchanged | passing |
| `TestReflectorRewritesReflectedNextHop` | `internal/chaos/scenario/topology_test.go` | The reflector's client receives the reflector as next hop and passes it to the collector unchanged, and the reflector passes the collector the border router's next hop | passing |
| `TestRouterAnnouncesItsBestExit` | `internal/chaos/scenario/topology_test.go` | Each router's announced path for a prefix with two exits | passing |
| `TestExternalNeighbourKeepsItsNextHop` | `internal/chaos/scenario/topology_test.go` | The eBGP speaker's own announcement | passing |
| `TestTopologyHasMultiHopTrace` | `internal/chaos/scenario/topology_test.go` | Over several seeds, every generated topology holds a router whose trace crosses an intermediate router, and the generator refuses one that does not (AC-24) | passing |
| `TestTraceOfHandCheckedPrefix` | `internal/chaos/scenario/topology_test.go` | The model's trace for one prefix, asserted literally, including one chain of three routers | passing |
| `TestTraceEndsWhereThePrefixOriginates`, `TestTraceLeavesTowardExternalAddress`, `TestNewTopologyRefusesInvalidSpec`, `TestTopologyRefusesUnknownQuestion` | `internal/chaos/scenario/topology_test.go` | The model's answer for a prefix originated inside (AC-30) and for an unchanged next hop (A-10), and its refusal of a spec or a question it cannot answer | passing |
| `TestRouteSetUpdateBytesEBGP`, `TestRouteSetUpdateBytesIBGP` | `internal/chaos/peer/sender_routeset_test.go` | Every byte of an UPDATE built from a given next hop and AS path: nothing prepended, LOCAL_PREF on iBGP only (AC-17) | passing |
| `TestRouteSetRefusesPathTheSenderWouldChange`, `TestRouteSetBuildRouteAnswersForItsPrefixes` | `internal/chaos/peer/sender_routeset_test.go` | A path the builder would change is refused, and a re-announcement stays inside the set | passing |
| `TestSimulatorAnnouncesItsRouteSet`, `TestSimulatorRefusesRouteSetWithAnotherFamily` | `internal/chaos/peer/sender_routeset_test.go` | A simulated session sends exactly the set, then End-of-RIB, and a set beside another family is refused before any session | passing |
| `TestGenerateFromTopologyProfiles`, `TestGenerateFromTopologyRefusesUnspecifiedSession` | `internal/chaos/scenario/generator_topology_test.go` | One profile per speaker, at the speaker's address, carrying its announcements | passing |
| `TestConfigGenCollectorForTopology` | `internal/chaos/scenario/generator_topology_test.go` | The collector config holds one peer per speaker of the model, with its AS, its address and its listen port, and an import policy on each external neighbour and on no other peer | passing |
| `TestLGSimulatedReadTableReadsByColumn`, `TestLGSimulatedPeersProblemNamesTheDifference`, `TestLGSimulatedHandCheckedRefusesAWrongNetwork` | `internal/test/fixture/lg_simulated_network_test.go` | The fixture's page reader and its comparisons refuse a page that differs from the model | passing |
| `TestLGSimulatedTraceReadsTheFields`, `TestLGSimulatedTraceProblemNamesTheDifference`, `TestLGSimulatedTraceEndNames`, `TestLGSimulatedTraceRefusesWhatIsNotTheGraph` | `internal/test/fixture/lg_simulated_trace_test.go` | The fixture reads the trace graph's text form field by field, names every end of the model, and refuses an answer that is not the graph | passing |
| `TestLGSimulatedReadGraphReadsBothModes`, `TestLGSimulatedReadGraphRefusesWhatIsNotTheGraph`, `TestLGSimulatedGraphWantFollowsTheModel`, `TestLGSimulatedGraphProblemNamesTheDifference`, `TestLGNextHopUnchangedHandCheckedAgreesWithTheModel`, `TestLGNextHopUnchangedTraceProblemRefusesTheRewrittenNextHop` | `internal/test/fixture/lg_simulated_graphs_test.go` | The fixture's comparison of the AS path and next-hop graphs with the model, and the hand-written network of A-10 against the model | passing |
| `TestTraceWalkGoesRouterToRouter` | `internal/component/lg/graph_trace_test.go` | From RIB rows in the shape the RIB query answers, a walk crosses two intermediate routers to the exit router | passing |
| `TestTraceGraphLinksExitToFirstAS` | `internal/component/lg/graph_trace_test.go` | The join of the exit router and the first AS of the exit route's path | passing |
| `TestTraceWalkExitsOnOwnAddress` | `internal/component/lg/graph_trace_test.go` | A next hop equal to the router's own address makes it the exit, never a loop (AC-26) | passing |
| `TestTraceWalkLeavesAtExternalNextHop` | `internal/component/lg/graph_trace_test.go` | A next hop that is no router's address leaves the AS toward it, with no session needed for that neighbour (AC-27) | passing |
| `TestTraceWalkStopsOnRevisitedRouter` | `internal/component/lg/graph_trace_test.go` | Two routers naming each other end with the loop statement (AC-25) | passing |
| `TestTraceWalkStopsOnRouterWithoutRoute` | `internal/component/lg/graph_trace_test.go` | A named router with no route for the prefix ends with its statement (AC-28) | passing |
| `TestTraceWalkOriginInsideAS` | `internal/component/lg/graph_trace_test.go` | An empty AS path at the exit states that the prefix originates there (AC-30) | passing |
| `TestTraceGraphTextIsOrdered` | `internal/component/lg/graph_trace_test.go` | The fixed order of the text form | passing |
| `TestTraceGraphRespectsNodeCap` | `internal/component/lg/graph_trace_test.go` | The cap and its message | passing |
| `TestTraceWalkStopsOnSeveralRoutes`, `TestTraceWalkStopsOnUnreadableRoute` | `internal/component/lg/graph_trace_test.go` | The two ends added in phase 7, each with its statement (AC-42, AC-43) | passing |
| `TestTraceRoutersAreTheInternalPeers`, `TestTraceRoutersRefusesUnreadableSummary`, `TestTraceGraphSaysWhyItHasNoTrace` | `internal/component/lg/graph_trace_test.go` | A router of the AS is a peer whose `peer-type` is internal, a summary row the walk cannot read is refused, and a graph with no walk says which of three causes applies with its counts (AC-40) | passing |
| `TestTraceGraphModeAnswersText`, `TestTraceGraphModeAnswersSVG`, `TestTraceGraphModeStatesWhatItDoesNotDraw`, `TestTraceGraphModeRefusals`, `TestTraceLayoutFollowsTheWalk`, `TestTraceGraphSVGOfOneExit`, `TestTraceGraphSVGStatesEveryEnd`, `TestGraphModeControlOffersTheTrace`, `TestHelpCarriesTheTraceNote` | `internal/component/lg/graph_trace_mode_test.go` | The third mode through the handler: its text form (AC-23), its drawing with every walk's end and the note (AC-29), its cap message, its refusals, the mode button and the help | passing |
| `TestLGGraphGoldenOutput` | `internal/component/lg/graph_golden_test.go`, captures under `testdata/graph/` | Six whole-SVG captures: the AS path, next-hop and trace graphs, each for two exits and for one exit | written; the captures await the judge |
| `TestLGGoldenOutput` and the handler captures | `internal/component/lg/golden_test.go`, `testdata/golden/`, `testdata/handler/` | The page markup this spec changed on purpose: the history cells, the Age column, the named next hop, the third graph button, the help | awaiting recapture and the judge |
| `TestNormalizeFixesWhatVariesRunToRun` | `internal/le/site/lgreplay/recording_test.go` | Uptimes, state-change times and the listener address become the declared fixed values, each with its count (AC-36). The third pass added the `route-age` rule to this file's tests | passing |
| `TestNormalizeLeavesStableTextAlone` | `internal/le/site/lgreplay/recording_test.go` | A route count, an AS number and a prefix are untouched | passing |
| `TestExchangeKeyIgnoresOrderEncodingAndEmptyFields` | `internal/le/site/lgreplay/recording_test.go` | The key the player computes for a browser's request equals the recorder's (AC-33) | passing |
| `TestFollowUpRequestsOfAnAnswer` | `internal/le/site/lgreplay/recording_test.go` | Every `hx-get` of a recorded answer is found once, in order | passing |
| `TestRecordingBytesAreStable` | `internal/le/site/lgreplay/recording_test.go` | The same recording encodes to the same bytes, and reads back equal (AC-36) | passing |
| `TestRecordAsksTheTourAndDeclaresWhatVaries` | `internal/le/site/lgreplay/record_test.go` | Against a test HTTP server: the tour, the follow-up requests, the assets and the stream sample are recorded, and an answer that changes between the two passes is named in `varies` (AC-32, AC-36) | passing |
| `TestRecordRefusesAnAnswerThatIsNotText` | `internal/le/site/lgreplay/record_test.go` | A binary answer is refused by name and nothing is written | passing |
| `TestLGReplayFrameCarriesTheRecordingAndAllowsNoConnection` | `internal/le/site/lgreplay_test.go` | The frame document embeds the recording and the player, and its policy allows no connection (AC-35) | passing |
| `TestLGReplayBlockListsTheTourAndTheTranscript` | `internal/le/site/lgreplay_test.go` | The block carries the mount, the tour list, the provenance and the transcript link (AC-37) | passing |
| `TestLGReplayBlockWithoutARecording` | `internal/le/site/lgreplay_test.go` | The block states the looking glass is not recorded, and the producer writes nothing (AC-38) | passing |
| `TestAuthoredPageExpandsARegisteredBlock` | `internal/le/site/lgreplay_test.go` | A `site-block` comment is replaced by its registered block, and an unknown block is refused by name | passing |
| `TestLGReplayAnswersTheTourFromTheRecording` | `internal/le/site/browsertest/lgreplay_test.go` | In a browser: the tour plays from the recording, an unrecorded search shows the notice, and the server sees no `/lg/` request (AC-33, AC-34, AC-35) | passing |
| `TestExtractLGNextHopRangesRefusesMappedRange` | `internal/component/config/lg_next_hop_ranges_test.go` | An IPv4-mapped range is refused with the IPv4 range to write (AC-44). `TestNewNextHopRangesBounds` holds the same refusal at the looking glass, and `TestLGNextHopRangeSchemaBoundaries` gained the name's refused characters | passing |
| `TestUpdateNextHopRangesRestoresTheReplacedList` | `internal/component/lg/nexthop_ranges_test.go` | The restore call puts back the list the update replaced (AC-45) | passing |
| `TestUpdateLGNextHopRangesRestoresOnRollback` | `cmd/ze/hub/listener_migrate_ranges_test.go` | The reload seam hands the restore call to the reload's rollback (AC-45). No test drives a whole reload whose promotion fails | passing |
| `TestTraceGraphBoundsTheWalksItLists` | `internal/component/lg/graph_trace_test.go` | 100 lines, then the count of the walks left out and its sentence (AC-46) | passing |
| `TestBestWalkRowCarriesNoAge`, `TestBestJSONTerminalRowCarriesNoAge` | `internal/component/bgp/plugins/rib/rib_pipeline_best_age_test.go` | A best-path row has no `age` in either form (AC-49) | passing |
| `TestTraceEndsOriginInsideOnAnEmptyASPath` | `internal/chaos/scenario/topology_trace_end_test.go` | The model ends a trace origin-inside on an empty AS path before it asks for the next hop, as the looking glass does (AC-30) | passing |
| `TestUncommittedChangesAreRefusedUnlessTrial`, `TestRecordDeclaresTheTrialKeyword` | `internal/le/site/lgreplay/actions_test.go` | The recorder refuses uncommitted changes unless `trial` is given, and declares the keyword (AC-47) | passing |
| `TestEachFailedStageNamesItsOwnFix`, `TestAStoppedRunSaysItWasStopped` | `internal/le/site/lgreplay/actions_test.go` | A failed stage of the recorder's network names the fix of that stage, and a stopped run says it was stopped | passing |
| `TestTourStepsAreDistinctQuestions` | `internal/le/site/lgreplay/record_test.go` | No two steps of the tour ask the same question | passing |
| `TestLGReplayRefusesARecordingOfADirtyTree`, `TestLGReplayProvenanceSaysTheNetworksAreNotReal` | `internal/le/site/lgreplay_test.go` | The site refuses to publish a recording made from an uncommitted tree (AC-47), and the page's provenance says the networks are simulated | passing |
| `TestLGReplayFrameOpenedDirectlyRunsNothingRecorded`, `TestLGReplayKeepsABoundedListOfFrameErrors` | `internal/le/site/browsertest/lgreplay_test.go` | In a browser: the frame document opened directly runs nothing recorded (AC-48), and the page keeps the first 20 frame errors and counts the rest | passing |
| `TestHeldInboundMovesToTheReplacedPeer`, `TestHeldInboundIsRejectedWhenThePeerIsRemoved`, `TestHeldInboundIsClosedWhenTheReactorStopsBeforeItsPeers` | `internal/component/bgp/reactor/peer_inbound_held_test.go` | A connection held before the peers start follows its replaced peer, is refused with Cease Connection Rejected when the peer is removed, and is closed silently when the reactor stops first (AC-50, AC-51, AC-52). The tests give the peer its connection with `setInboundConnection` and do not start the peer, so the hand-over is proven up to `takeInboundConnection` | passing |
| `TestLGSimulatedKeepSurvivesTheWorkDirectory`, `TestLGSimulatedKeepSaysWhenItCannotWrite`, `TestLGSimulatedTailIsBounded` | `internal/test/fixture/lg_simulated_failure_test.go` | A failed run of the simulated network keeps its evidence outside the directory the runner removes, says when it cannot, and bounds the tail it prints | passing |

Status as of 2026-11-19. `passing` means the main thread ran the test and it
passed: the RIB packages whole, the reactor package whole, the chaos packages,
the looking glass package, the config package, the looking glass tests of the
hub and fixture packages, the recorder's package, the replay's site tests and
its three browser tests. The RIB, reactor and hub packages ran through the
overlay of R-15. Every functional test of the table below passes from its
promoted place: `lg-simulated-network`, `lg-peer-history`, `lg-origin-search`,
`lg-route-age` and `lg-named-next-hop` after 80 stressed runs each with no
failure, `lg-aspath-search` after 120, and `lg-simulated-unchanged-next-hop`
after 160.

One review round ran (an independent reader over the whole branch, then the
fixes and a naming pass), and an independent judge ruled three times on the
changed test expectations and captures, with no refusal. Lint reports no
finding in a file this branch wrote, in the packages it can analyse; it cannot
analyse the RIB, reactor and hub packages on this branch (R-15), where this
branch's lines were read against the same rules.

The recording `website/data/lg-replay.json` was made at commit `af2c9677aa`
from a clean tree, and a second run at that commit wrote the same bytes
(AC-36). The site page of AC-38 has not been built: a full site build refuses
on this branch (R-16), so it renders once the branch is on `main` and the site
is built there.

What remains before closure: the Review Gate and the closure sections of
`/ze-close`, in a session that did not write this work; the site build of
AC-38 from `main`; and the open product window of R-17.

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
| `lg-peer-history` | `test/plugin/lg-peer-history.ci` | A session drops twice. The peers page shows the count, the time and the reason | passing as a draft |
| `lg-origin-search` | `test/plugin/lg-origin-search.ci` | A search anchored at the end lists only the routes that AS originates | passing as a draft |
| `lg-route-age` | `test/plugin/lg-route-age.ci` | The API answers a non-zero age for a route learned before the request, and the page shows an age between two API answers | passing as a draft, API and page |
| `lg-named-next-hop` | `test/plugin/lg-named-next-hop.ci` | A named range labels the next hop in the table and the graph, and a reload renames it | passing as a draft, with the reload |
| `lg-simulated-unchanged-next-hop` | `test/plugin/lg-simulated-unchanged-next-hop.ci` | A hand-written network of three routers and two neighbours, where one border router does not rewrite the next hop. The trace graph leaves the AS at that router toward the neighbour's address (A-10, AC-27) | passing as a draft, on the second run (A-10) |
| `lg-aspath-search` | `test/plugin/lg-aspath-search.ci`, replaced by the draft of the same name | The existing test of the AS path search, whose refused input changes from `65001$` to `65001$$` now that one end anchor is accepted | the draft replacement passes; the changed expectation awaits the judge |
| `lg-covering-lookup` | `test/plugin/lg-covering-lookup.ci` (existing, unchanged) | Its shared row reader now reads nine cells, the Age column last | passing; the reader's change awaits the judge |
| `lg-simulated-network` | `test/plugin/lg-simulated-network.ci` | Five simulated routers and their external neighbours hold sessions with Ze. For each prefix of the model, the peers page, the search and the trace graph answer what the model computed: the exits announcing it, each router's chain of BGP next hops to its exit, with at least one chain crossing the route reflector that rewrites the next hop (AC-24), and the AS chain outside | passing as a draft, with five OK lines: 11 speakers with the model's route counts; 9 prefixes searched; the AS path graphs; the next-hop graphs; the trace graph, 45 traces, 18 across three routers or more, 4 prefixes with one exit, 5 ending at an origin inside the AS |

"Passing as a draft" means the test passed against a real daemon from
`test/draft/plugin/` in the main thread's run of 2026-11-18. The Location
column is where each lands on promotion. Still owed for every draft: the stress
runs, then the promotion, then `docs/functional-tests.md`.

## Files to Modify
- `internal/component/lg/handler_ui.go` - history, age and group in the view rows
- `internal/component/lg/view.go` - the view fields
- `internal/component/lg/peers.templ`, `route_table.templ`, `route_detail.templ`, `help.templ` - the columns and their help
- `internal/component/lg/peer_routes.templ`, `search.templ` - the history and the group in the peer header, the Age column and the named next hop in a peer's routes, the form of the AS path field
- `internal/component/lg/aspath_pattern.go` - the page's reading of the end anchor
- `internal/component/lg/handler_api.go` - `age` and `age_available` in `transformRoutes`
- `internal/component/lg/render.go` - `formatElapsed`, one wording for uptime and age
- `internal/component/lg/graph_nexthop.go`, `layout_nexthop.go`, `handler_graph.go` - the node name. `handler_graph.go` also holds the trace mode, and `layout_nexthop.go` the drawing helpers the trace drawing shares
- `internal/component/lg/server.go` - the named ranges held by the server, replaceable on reload
- `internal/component/lg/register.go` - the tree validator `lg-next-hop-range`
- `internal/component/lg/yang/ze-lg-conf.yang` - the named range list
- `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/listener_migrate.go` - the list handed to the server at start, and the `nextHopRangesUpdatable` seam on reload
- `cmd/ze/hub/main.go`, `service_registry.go`, `main_reload.go`, `register_lg.go` - where the list is read at start and on reload and carried in `serviceDeps`. The extraction lives in `internal/component/config/` (Files to Create), beside `loader_extract.go`
- `internal/component/bgp/plugins/rib/rib_pipeline.go`, `rib_pipeline_best.go` - the end anchor, the `aspath` spelling validated like `path`, the `age` member
- `internal/component/bgp/plugins/rib/rib.go`, `rib_structured.go`, `rib_commands.go`, `rib_inject.go` - the reading taken where a route is received or injected
- `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` - the path pattern in the descriptions of `show` and `best`
- `internal/component/bgp/plugins/rib/storage/routeentry.go`, `peerrib.go`, `familyrib.go` - the route time
- `internal/component/bgp/plugins/cmd/peer/fields.go` - the `description` field name
- Existing tests whose expectations changed, each for the judge: `internal/component/lg/validate_test.go`, `handler_ui_test.go`, `ribfake_test.go`, `golden_test.go`, `handler_api_test.go`, `view_test.go`, `graph_nexthop_test.go`, the captures under `internal/component/lg/testdata/`, `internal/test/fixture/lg_aspath_search.go`, `lg_covering_lookup.go`, and `test/web/lg-stream-error.wb` (as a draft, for the twelve-column span)
- `docs/guide/looking-glass-howto.md`, `features/looking-glass.md`, `ai/digests/rib.md` - the pages and the digest as built
- `plan/journal/constant-reported-as-measured-state.md` - the owned row's Fix cell
- `website/changes/fragments/lg-peer-history-origin-search-and-named-nodes.md` (created) - the news fragment
- `internal/component/bgp/plugins/cmd/peer/summary.go`, `peer.go` - description and group
- `internal/component/bgp/reactor/peer_settings.go`, `config.go`, `peer.go`, `peer_settings_apply.go`, `reactor_api.go`, `internal/component/plugin/types_bgp.go` - the peer's `description` leaf carried to the peer record and the peer snapshot, delivered on reload without a session restart (AC-31, the fix A-4 found)
- `docs/guide/command-reference.md`, `docs/features/formatting.md`, `docs/architecture/core-design.md` - the two summary keys, the column order, the reload swap set
- `docs/guide/looking-glass.md`, `docs/features/looking-glass.md`, `docs/architecture/web-interface.md` - the page as built
- `docs/architecture/api/birdwatcher-compat.md` - `age` and `description`
- `docs/architecture/plugin/rib-storage-design.md` - the route time and its cost
- `docs/architecture/api/commands.md` - the summary keys
- `docs/guide/configuration.md`, `docs/guide/cli.md` - the list and the anchor
- `internal/chaos/scenario/generator.go` - the topology as an input beside the seed
- `internal/chaos/peer/simulator.go`, `sender.go`, `simulator_actions_v2.go` - a given route set per peer (A-8 is broken)
- `internal/chaos/scenario/profile.go`, `config.go` - the route set a profile carries, and the collector config written from the profiles
- `internal/chaos/orchestrator/run.go`, `internal/chaos/inprocess/runner.go` - both runners hand a profile's route set to its simulator
- `internal/test/fixture/plugin_fixture_09.go` - the registrations of `plugin/lg-peer-history`, `plugin/lg-origin-search`, `plugin/lg-route-age`, `plugin/lg-named-next-hop`, `plugin/lg-simulated-network` and `plugin/lg-simulated-unchanged-next-hop`
- `internal/component/bgp/reactor/peer_connection.go`, `reactor.go`, with `peer.go` and `reactor_api.go` above - the connection held before the peers start (AC-50 to AC-52): `setInboundConnection`, `Peer.Stop`, `stopWithCease`, `reconcilePeersJournaled`, `abortStartup`
- `docs/architecture/behavior/peer-lifecycle.md`, `ai/digests/bgp-reactor.md` - what happens to a held connection
- `internal/test/fixture/fixture.go` - `Run` flushes the stderr pump before the driver (R-19)
- Not modified, against the first plan: `internal/le/interoplab/bgp/lg_lab.go`, `test/plugin/lg-graph-lab.ci` and its saved SVG files. The lab stays (Existing Mechanisms, R-13)
- `docs/architecture/chaos-web-dashboard.md`, `docs/guide/chaos-testing.md` - the topology input and what the simulated routers announce

## Files to Create
- `internal/chaos/scenario/topology.go` - the topology model, its best-path computation and the per-speaker announcements
- `internal/chaos/scenario/topology_generate.go` - the topology generated from a seed, split from the model to keep each file to one concern
- `internal/component/lg/graph_trace.go` - the trace graph and its text form
- `internal/component/lg/layout_trace.go` - the trace graph's layout and drawing
- `internal/component/lg/nexthop_ranges.go` - the named ranges: the lookup, the swap on reload, the validator's body
- `internal/component/config/lg_next_hop_ranges.go` - `ExtractLGNextHopRanges`, the one reader of the list, and its refusals
- `internal/component/bgp/plugins/rib/storage/routetime.go` - `RouteTime` and its age
- `internal/component/bgp/plugins/rib/rib_route_age.go` - the RIB's route clock and the `age` member of a row
- `internal/test/fixture/lg_simulated_network.go` - the fixture that builds the model, starts Ze on the config written from it, runs one simulator per speaker and compares the looking glass with the model
- `internal/test/fixture/lg_simulated_trace.go`, `lg_simulated_graphs.go`, `lg_simulated_next_hop_unchanged.go` - the comparison of the trace graph, of the AS path and next-hop graphs, and the hand-written network of A-10
- `internal/test/fixture/lg_peer_history.go`, `lg_origin_search.go`, `lg_route_age.go`, `lg_named_next_hop.go` - the fixtures of the four first functional tests
- Test files, each new: `internal/component/bgp/plugins/rib/storage/routeentry_age_test.go`, `internal/component/bgp/plugins/rib/rib_pipeline_age_test.go`, `internal/component/lg/route_age_test.go`, `nexthop_ranges_test.go`, `graph_trace_test.go`, `graph_trace_mode_test.go`, `graph_golden_test.go` with `testdata/graph/`, `internal/component/config/lg_next_hop_ranges_test.go`, `cmd/ze/hub/listener_migrate_ranges_test.go`, `internal/chaos/scenario/topology_test.go`, `generator_topology_test.go`, `internal/chaos/peer/sender_routeset_test.go`, `internal/test/fixture/lg_simulated_network_test.go`, `lg_simulated_trace_test.go`, `lg_simulated_graphs_test.go`
- `internal/component/bgp/reactor/peer_inbound_held.go` - the hand-over, the refusal and the close of a connection held before the peers start (AC-50 to AC-52)
- Test files the review round added, each new: `internal/component/bgp/reactor/peer_inbound_held_test.go`, `internal/component/bgp/plugins/rib/rib_pipeline_best_age_test.go`, `internal/chaos/scenario/topology_trace_end_test.go`, `internal/le/site/lgreplay/actions_test.go`, `internal/test/fixture/lg_simulated_failure_test.go`
- `test/plugin/lg-simulated-network.ci`
- `test/plugin/lg-simulated-unchanged-next-hop.ci`
- Until promotion every `.ci` file of this list is a draft under `test/draft/plugin/`, with the draft replacement of `lg-aspath-search.ci`, and `test/draft/web/lg-stream-error.wb`
- `test/plugin/lg-peer-history.ci`
- `test/plugin/lg-origin-search.ci`
- `test/plugin/lg-route-age.ci`
- `test/plugin/lg-named-next-hop.ci`
- `internal/le/site/lgreplay/` (need 7) - the recorder: `register.go` and `actions.go` (`./le site lg-replay record`), `tour.go` (the tour, one list), `recording.go` (the file format, the request key, the normalization), `record.go` (the run and the two passes)
- `internal/le/site/lgreplay.go` (need 7) - the `lg-replay` producer (frame document and transcript) and the `lg-replay` block of the labs page
- `website/assets/js/lg-replay.js` (need 7) - the player: the host page's controls and, inside the frame, the answers from the recording
- `website/data/lg-replay.json` (need 7) - the recording, written by the recorder
- `internal/le/site/lgreplay/recording_test.go`, `record_test.go`, `internal/le/site/lgreplay_test.go`, `internal/le/site/browsertest/lgreplay_test.go` (need 7)

Modified for need 7: `internal/test/fixture/lg_simulated_network.go`
(`RunLGSimulatedNetwork`, shared with the recorder), `internal/le/site/authored.go`
(the `site-block` registry), `internal/le/register.go` (one blank import),
`website/labs/looking-glass-graph/index.html` (the block),
`website/assets/css/30-components.css` (the replay's frame and controls),
`website/AI.md` and `docs/contributing/gh-pages.md` (how the replay is recorded
and published).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/lg/yang/ze-lg-conf.yang`: a list of named ranges under the looking glass container |
| YANG validation constraints | Yes | The prefix uses the shared prefix type, the name a length and a pattern, the list a maximum of 256. As built, the schema walk does not reach `environment`, so the reader `ExtractLGNextHopRanges` enforces the 256 (`LGNextHopRangesMax`) |
| YANG custom validators | Yes | One, registered as `lg-next-hop-range`: no two entries with the same prefix once masked, refused at commit with both entries and the line to delete |
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
   - Tests: the seven `topology_test.go` tests, then `lg-simulated-network` up to the peers page and the search
   - Files: `topology.go`, `generator.go`, `simulator.go`
   - Verify: A-8 and A-9 are settled first. Every simulated speaker is an established session with the model's route count
7. **Phase: Trace** -- one graph from router to origin AS
   - Tests: the `graph_trace_test.go` tests, fifteen as built
   - Files: `graph_trace.go`, `handler_graph.go`, the graph buttons in `route_table.templ`, `help.templ`
   - Verify: A-10 is settled first. `lg-simulated-network` passes whole. As built, the hand-written lab and its saved files are not removed: the lab stays, because its proof of AS names through the real decorator and DNS has no other test, and the generated network adds to it

Where the work stands on 2026-11-18, by the main thread's runs. Phases 1 to 7
are written. Committed: cc90420075 (the engine end anchor and the route age),
d2c5426511 (the topology model and the route sets), 4e7f74f51f (the peer
description), the build repair 35d6480c53, and 7d1a7b193f (the connection held
before the peers start). The looking glass, hub, config,
fixture and replay code is in the tree and not yet committed. Not yet done:
the stress runs and the promotion of the drafts, the judge's verdicts on the
changed expectations and the captures, lint, the review rounds, a second
recording to confirm AC-36, and the site build (R-16).

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
- A mandatory API field with no producer reads as a measured zero. The contract page has a pattern for that, an availability flag, which this spec makes unnecessary for `age` by producing the value. As built this held only for received rows: sent rows and best-table rows still have no age, so the API carries the flag after all (`age_available`, AC-41).
- AC-9, measured on 2026-11-18 by `TestRouteEntrySize`: the route record is 64 bytes before and 64 bytes after, growth zero. The time took four of the five padding bytes after the three one-byte fields.
- A unit fixture written from one config shape hid a zero. The trace passed every unit test on a summary with a global local AS and found no router on real sessions, where the local AS is stated per peer. The simulated network found it on its first run, which is what it was built for.

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
| The hand-written lab stays beside the generated network | Remove the lab and its six saved files once the network passes, as first planned | Found in phase 7: only `test/plugin/lg-graph-lab.ci` proves an AS name reaching a graph through the real decorator against a DNS answer. Removing it would delete that proof, so the network adds to the lab and replaces nothing |
| A router of the AS is a peer whose `peer-type` the engine answers as internal | Compare each peer's `remote-as` with the summary's global `local-as`, the first design. Write a global local AS into the generated collector config | The global `local-as` is zero when the operator states the local AS per peer, so the first design found no router on real sessions. The config change would have hidden that from the test and left every such operator with an empty trace. The engine already decides the type per peer |
| The API keeps `age` and adds `age_available` | Omit `age` when the engine does not know it | The birdwatcher contract makes `age` mandatory. Section 7.2 of the contract page already carries a count the same way: the member stays, and a flag beside it says whether it is known (now Section 6.2 and divergence 6) |
| Accepted by the owner's order of 2026-11-18: internal routers send their best path for every prefix | Send only eBGP-learned routes, as a plain iBGP peer does | With the plain behaviour the collector cannot know which exit a non-border router uses, and no path can be traced inside the network |
| Accepted by the owner's order of 2026-11-18: a third graph mode joins the two existing graphs | Leave two graphs and let the reader match the exit router by eye. Replace both graphs with the trace graph | Two graphs leave the join, which is the question asked, to the reader. Replacing them removes the two focused views operators already have |
| Accepted by the owner's order of 2026-11-18: this work sits in this post-release spec | Put it in the release fix as its proof | The fix is proven by two small tests and is nearly written. The network and the trace are larger and would hold it back |
| The trace follows each router's BGP next hop, router to router, then the AS path | Show only each router's chosen exit. Model the IGP to draw the hops between two next hops | The owner's decision of 2026-11-18. Ze holds every router's chosen route and next hop, so where the network rewrites next hops the chain through the core is observed, not guessed. A collector does not see the IGP's hops between two consecutive next hops, and drawing them would publish a guess, so the page says they are absent |
| A walk ends early only on a revisited router or at a router with no route, and says which. A next hop equal to the router's own address, or to no router's address, is the exit | Stop silently. Draw the partial chain as if it were complete. A fixed hop limit | A partial chain drawn as complete is a silently wrong answer. The visited-router guard bounds the walk by the number of routers, so a hop limit would add nothing |
| No country or registry columns | Add them from the AS name source | They describe the AS, not the session, and an operator of a collector knows its peers. They can be added when asked for |
| Need 7: the website replays recorded looking glass answers inside a sandboxed frame | An encoded video. Screenshots. A live public demo instance | A video is refused by the website rules (no encoded video published or stored), cannot be clicked, and is not text a diff can review. Screenshots are what the site has today: they show one state and nothing a visitor can explore. A live instance is a daemon somebody has to run, secure and keep in step with `main`, and the site is static hosting. A recording is text, is regenerated from the deterministic simulated network, and serves the genuine markup, stylesheet and scripts |
| Need 7: the frame is a generated document loaded in an `iframe` with `sandbox="allow-scripts allow-forms"`, never `srcdoc` and never a shadow root | A `srcdoc` frame. A shadow root in the site page. A same-origin frame | The looking glass's htmx resolves every request against `location.href`, which in a `srcdoc` document is `about:srcdoc` and cannot be a base. A shadow root isolates styles only: the looking glass's scripts listen on `document` and would run in the site page. A same-origin frame would share the site's storage and could reach its page. The sandboxed document has an origin of its own, and its policy (`default-src 'none'`) lets the browser refuse any request the player does not answer |
| Need 7: the recorder follows every `hx-get` of a recorded answer, up to a declared cap | Record the tour's questions only | With the tour alone, a visitor who clicks a row the tour did not click gets the notice on most of the page. The follow-up requests of the simulated network are a few dozen small fragments, so every row a recorded page shows has its answer. Free-text searches are the questions that stay unrecorded |
| Need 7: values that change run to run are replaced by fixed values the file declares, and every question is asked twice, ten seconds apart | Record the values as they are. Strip them from the page | Recorded as they are, two runs of one commit never give the same file, so a diff of the recording says nothing. Stripped, the page no longer looks like the looking glass. The second pass names any answer no rule covered, so the file states what still differs |

## Known Limitations

- Ages restart from zero when the daemon restarts, because the routes are relearned.
- A next hop is named only when an operator named its range. Nothing is guessed.
- Only received rows carry an age. A sent row and a row of the best table (`show bgp rib best`) carry none, so the API answers `age` 0 with `age_available` false for `routes/table/{family}` and `routes/export/{name}`. The CSV download of a peer's routes has no age column.
- A change of the label stack alone, under equal attributes, does not move a route's time, and neither does an entry modified in place. The time follows the stored attributes.
- The trace takes as a router of the AS every peer the engine types internal, and the engine judges that against each peer's own session local AS. Where peers carry a per-peer local-AS override, peers internal to different local AS numbers are walked as routers of one AS: the trace does not tell them apart.
- A lookup by a bare address answers every covering prefix, and the trace walks each of them on its own rows, so the answer holds one trace per prefix and router. Asking with the prefix and its length gives one prefix.
- The trace asks the peer summary after the RIB query, so the two answers are not of one moment.
- An AS path the engine truncated is written with no `as-path` member, as an empty path is, so the trace reads it as a prefix originated inside the AS.
- The replay on the website answers only the questions the recorder asked: the tour and the requests its answers link to. Any other request, a free-text search included, shows a notice that names it.
- The hand-written graph lab stays Linux-only. The simulated network runs on Linux and macOS, with one listen port per speaker and no loopback alias.
- `le chaos run` has no option for a topology: only the functional fixtures and the recorder build one.

## Mistake Log

The table of `plan/templates/TEMPLATE-CLOSURE.md`, started early because
`ai/rules/planning.md` and `/ze-implement` ask for the row when an assumption
breaks. The rest of the closure template is appended at closure, not here.

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-8 read `SimulatorConfig` as possibly able to announce a given route set | `(*Sender).BuildRoute` built every route from one fixed next hop and an AS path derived from the AS number. The simulator could only announce seed-generated routes | The audit before phase 6 read the sender (`internal/chaos/peer/sender.go`) | Phase 6b added the route set first, as the assumption's "If wrong" column planned: `scenario.RouteSet`, `SimProfile.RouteSet`, `(*Sender).BuildAnnouncement`. Committed as d2c5426511 |
| assumption | The spec said the looking glass config is extracted in `buildLGService` (Existing Mechanisms, the census, Integration Points, the wiring table) | `buildLGService` (`cmd/ze/hub/service_lg.go`) reads no config: it reads `serviceDeps`, which `runYANGConfig` (`cmd/ze/hub/main.go`) fills from `ExtractLGConfig` and `ExtractLGSettings` in `internal/component/config/loader_extract.go` | The audit before phase 5 read the function | The named ranges follow the real path: `ExtractLGNextHopRanges` in `internal/component/config/`, `serviceDeps.LGNextHopRanges`, then `buildLGService`. The rows of this spec that named the function are corrected |
| assumption | The spec said the page follows the engine's AS path grammar "through the shared test table the fix spec adds" | No shared table exists. The engine's table is in `rib_pipeline_test.go` and the page's in `validate_test.go`, and the engine reads commas where the page writes spaces | The audit read both test files. Phase 3a confirmed it | The page's table restates the engine's accepted and refused forms, and the looking glass test fake validates and matches `$` as the engine does (`ribfake_test.go`). The two tables are still kept in step by hand |
| approach | The trace's first design called a peer a router of the AS when its `remote-as` equalled the summary's global `local-as` | That member is the global local AS, and it is zero when the operator states the local AS per peer, as the generated collector config does. No peer matched, so the trace was empty | Only when the trace met real sessions: the first run of `lg-simulated-network` answered "No route found from a router of the AS" on eleven established sessions. The unit fixture had written `"local-as":65000`, the shape of one kind of config. Phase 7a's notes had flagged a per-peer override and missed the zero | `traceRoutersFrom` reads each row's `peer-type`. The fixture now writes `"local-as":0`, `TestTraceRoutersAreTheInternalPeers` pins the rule, and an empty trace says why (AC-40) |
| assumption | The generated collector config was written with no import policy, on the assumption that a collector stores what its peers send | Ze applies RFC 8212 to an eBGP peer with no import policy and stores nothing from it. All eleven sessions established and every external neighbour showed zero routes received | The first run of `lg-simulated-network`: `Received "0"` for 203.0.113.1, with the RFC 8212 warning for each eBGP peer | `writeFullPeerBlock` writes an accepting import policy on each eBGP peer when `ConfigParams.Collector` is set, and `TestConfigGenCollectorForTopology` fails without it. The seed-generated config is unchanged |

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
