# ExaBGP Bridge Plugin

The bridge runs ExaBGP-API scripts against ze's BGP engine. It has two runners
with one behavior: an external SDK-mode process, and an internal in-process
runner. Both hand their scripts to one `bridgerun.Fleet`, so the fork, the
event fan-out, the ack and the respawn are written once.

<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- runInternalBridge, familyDecls -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- exabgp bridge config parsing -->
<!-- source: internal/plugins/exabgp/bridgerun/fleet.go -- Fleet, Script, Broadcast -->

## The bridge translates in two directions

An ExaBGP script writes text commands down to ze, and ze sends events up to the
script. Each direction has its own translator.

Down, `TranslateLine` reads one ExaBGP line and writes one ze CLI command.
`neighbor <address> announce route <prefix> next-hop <nh>` becomes
`send bgp <address> update text nhop <nh> nlri add <prefix>`.
Twelve sites in `bridge_command.go` build a command of the shape
`send bgp <selector> update text ...`, where the selector is one address or `*`.

`TranslateLine` answers a `Translation`, not a string. It carries the command,
the selector that command addresses, and whether the command puts an UPDATE on a
wire. The caller flushes with that selector. It does not read the selector back
out of the command, and the next section says what that cost.

Up, `bridge_event.go` reads one ze event into a `bridge.Event`. Each encoder
renders that. `ExabgpJSON` writes one JSON object per event, in envelope version
6.0.0. `AppendText` writes ExaBGP's one-event-per-line text. A script's own
`encoder` leaf decides which one it gets.

`MarshalExabgpJSON` serializes the JSON object in ExaBGP's own spacing,
`{ "key": value, "key": value }` and `[ a, b ]`, on both the fleet and the
single-event path. ExaBGP builds its JSON by hand that way, and scripts match on
substrings of it: upstream's api-rr script tests `'"nlri": "192.168.0.0/32"'`,
which Go's compact form never contains.

<!-- source: internal/exabgp/bridge/bridge_event_json.go -- MarshalExabgpJSON -->

An `announce route-refresh <afi> <safi>` line becomes `request peer <selector>
refresh <afi>/<safi>`, which sends the peer a ROUTE-REFRESH (RFC 2918 Section 3).
It is read before the route forms, whose `announce route` prefix would otherwise
take it as a route.

<!-- source: internal/exabgp/bridge/bridge_route_forms.go -- convertRouteRefresh -->

An `announce eor <afi> <safi>` line becomes `update text mp <family> eor`
(`update text nlri eor` for IPv4 unicast). A bare `announce eor` is IPv4 unicast
alone, because ExaBGP's `api_eor` answers `Family(AFI.ipv4, SAFI.unicast)` when
no family token follows; any other token count is refused. Each line sends its
marker, however many the session already carried, as ExaBGP does.

<!-- source: internal/exabgp/bridge/bridge_route_forms.go -- convertEOR -->

An `announce vpls` or `withdraw vpls` line becomes
`update text <attributes> mp l2vpn/vpls add|del rd <rd> ve-id <n>
ve-block-offset <n> ve-block-size <n> label-base <n>`, the RFC 4761 Section 3.2.2
NLRI. ExaBGP names the fields `rd`, `endpoint`, `offset`, `size` and `base`, and
writes them among the attributes in any order, so they are taken out by name and
every other word goes to the attribute parser. All five are required: a line
missing one is refused by name, never sent with a zero in its place. A trailing
`withdraw` flag on an announce deletes the route, as it does for every family.

<!-- source: internal/exabgp/bridge/bridge_vpls.go -- convertVPLSRoute -->

ExaBGP 6.0's `announce ipv4 mpls-vpn <prefix> ... rd <rd> label <n>` is the
family-stating spelling of `announce route <prefix> ... rd <rd> label <n>`, and
both become one `mp ipv4/mpls-vpn` section with the same bytes (SAFI 128).
`announce ipv4 rtc origin-as <asn> route-target <rt> next-hop <ip>` and
`announce ipv4 rtc default next-hop <ip>` become an `mp ipv4/rtc` section, which
the rtc plugin encodes as the RFC 4684 Section 4 membership; `withdraw ipv4 rtc`
sends the same words under `del`. Both SAFI words are rows of `bridgeSAFI`.

<!-- source: internal/exabgp/bridge/bridge_family.go -- bridgeSAFI -->


The read happens ONCE for the whole fleet, and each format is rendered once. So
the cost of a fan-out grows with the number of formats in use, not with the
number of scripts. Both numbers matter here, because the fan-out runs for every
UPDATE of every peer.

The ExaBGP side is an external contract. An operator writes a script against
ExaBGP, so a change to ze's own CLI does not reach that script through the
translated forms.

<!-- source: internal/exabgp/bridge/bridge_command.go -- TranslateLine, Translation, convertRoute -->
An End-of-RIB is rendered as ExaBGP renders it, `{"eor": {"afi": ..., "safi": ...}}`
inside `message`, and never as an update with the member missing. `ExabgpJSON`
used to write `message` only when the converted update was non-empty, and an
End-of-RIB converts to nothing, so the marker reached a script as an event with
no message at all. A ported ExaBGP script reads that line, matches none of its
keywords and takes its failure branch.

<!-- source: internal/exabgp/bridge/bridge_event.go -- ReadEvent, Event, ExabgpJSON, the eor arm of the update case -->
<!-- source: internal/exabgp/bridge/bridge_event_text.go -- Event.AppendText, TextForm -->

## The text encoder is ExaBGP's own, ported

A process declaring `encoder text` gets ExaBGP's line format, ported from
`Response.Text` in `src/exabgp/reactor/api/response/text.py`. The v4 encoder in
`response/v4/text.py` answers byte-identical lines, so there is one format
rather than two.

```
neighbor 127.0.0.1 up
neighbor 127.0.0.1 down - hold timer expired
neighbor 127.0.0.1 receive update start
neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100
neighbor 127.0.0.1 receive update withdrawn 10.0.0.0/24
neighbor 127.0.0.1 receive update end
neighbor 127.0.0.1 send keepalive
neighbor 127.0.0.1 send notification code 6 subcode 2 data 0x53687574646F776E20436F6D6D756E69636174696F6E3A202262796522
neighbor 127.0.0.1 send open version 4 asn 65001 hold_time 90 router_id 1.1.1.1 capabilities [ ... ]
neighbor 127.0.0.1 receive route-refresh afi 1 safi 1 0
```

One event is one line. So every value a PEER chose is escaped before it is
written. A shutdown reason carrying a newline would otherwise forge a whole
event on the script's stdin. That is the CWE-116 ExaBGP's own `oneline()` exists
to close.

`negotiated`, `fsm` and `signal` get NO line, which is what ExaBGP's own text
encoder answers for them. The encoder names that answer apart from "this kind is
unknown to me", so a gap reaches a log rather than a silence.

An attribute whose value is an EMPTY LIST is written not at all. ExaBGP renders
a list attribute through `str(attribute)` and skips it when that answers the
empty string, which `ASPath.string` does for a path with no segments. A route
this speaker originated has an empty AS_PATH and ze reports it as
`"as-path": []`, so without the skip every announced line carried an
` as-path [ ]` ExaBGP never writes. `api-check` compares the whole line, so that
one token failed the case.

Three places diverge from ExaBGP. Each one does because the ze event carries
less than ExaBGP's own objects do.

| What | ze writes | Why |
|---|---|---|
| An attribute with no registered JSON key | ` attribute [ 0xCC <value> ]` | the event carries no attribute flags on this path |
| The capability list of an `open` line | ze's name and value for each capability | the event carries a code, a name and a value, not ExaBGP's `Capabilities` object |
| A structured NLRI | its `nlri` field, or compact JSON | `extensive()` is per family and is not reachable from the object ze sends |

<!-- source: internal/exabgp/bridge/bridge_event_text.go -- appendUpdateText, appendOneline, textAttributes -->

## A line that names no neighbor goes to the peers that feed the script

`splitNeighborSelector` reads a destination off the front of the line. A line
that carries none names no destination. ExaBGP sends such a line to the peers
whose `api { processes [ ... ] }` list names the process that wrote it
(`Reactor.peers(service)`, `src/exabgp/reactor/loop.py`). The bridge reads it the
same way. `Translator.Peers` carries those addresses, so `announce route
<prefix>` becomes
`send bgp 127.0.0.1,192.0.2.1 update text nlri add <prefix>`.

A translator with no peers sends to every peer, `send bgp * ...`. That is what
one script and one neighbor means, and it is what the bridge did for every
script until 2026-09-06. Until then a two-process config put each script's
routes on both sessions. `api-multiple-api` is the case for it.

## The destination keyword is `neighbor` or `peer`

ExaBGP spells the keyword twice. Its v4 API writes `neighbor <address>` and its
v6 API writes `peer <address>`, its own selector parser takes either ("Accept
both 'neighbor' (v4) and 'peer' (v6) prefixes", `extract_neighbors`,
`src/exabgp/reactor/api/command/limit.py`), and the bridge reads both. One
script is held to one of them: its first `neighbor` line holds it to API 4 and its
first `peer` line to API 6, and from then on a line in the other spelling is
answered with its reason and `error`, and reaches no peer ("`encoder` says which format a script
reads" below).

v6 also holds every address in ONE bracket rather than repeating the keyword
after each comma, so `peer [A router-id X, B] announce route ...` names the two
sessions `neighbor A router-id X, neighbor B announce route ...` names. A list
holding `*` is `*`, because ExaBGP unions the entries and ze's own selector
parser reads a comma list as addresses alone.

The bridge read `neighbor` alone until 2026-09-13, and ExaBGP's healthcheck
application is what measured it. That application writes
`peer * announce route <ip> next-hop <nh> med <n>`, so every line it wrote was
refused by name, nothing was announced, and the script read the refusal as a
failed check. `api-healthcheck-module` is the case.

The rest of the v6 vocabulary is read in its v4 spelling. v6 is target-first,
so it writes `daemon shutdown`, `session ack disable`, `rib flush out`,
`rib clear in`, `peer create <ip> ...` and `peer delete <ip>` where v4 wrote
`shutdown`, `disable-ack`, `flush adj-rib out`, `clear adj-rib in`,
`create neighbor <ip> ...` and `delete neighbor <ip>`. ExaBGP translates each
v4 line into its v6 form (`translate_v4_to_v6`, `dispatch/v4.py`); the bridge
reads the same table the other way, so the two spellings of a word reach one
translation. A word whose v4 spelling the bridge does not translate
(`daemon reload`, `session reset`, `system version`, `rib show in`, ...) is
refused in both spellings with the same reason. `api-v6-vocabulary` is the case.

<!-- source: internal/exabgp/bridge/bridge_selector.go -- bridgeSelectorKeyword, splitBracketSelector, selectorForAddresses -->
<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- convertNeighborControl -->
<!-- source: internal/exabgp/bridge/bridge_v6_control.go -- v6ControlSpellings, v4ControlSpelling -->

## Clearing and flushing the adj-rib-out

| ExaBGP line | ze command | What ExaBGP does at the pin |
|-------------|------------|-----------------------------|
| `clear adj-rib out` | `clear bgp rib advertised <sel>` | `OutgoingRIB.withdraw`: withdraws every route the adj-rib-out holds |
| `flush adj-rib out` | `clear bgp rib out <sel>` | `OutgoingRIB.resend`: sends them all again, between a BoRR and an EoRR when Enhanced Route Refresh was negotiated |
| `clear adj-rib in` | `clear bgp rib in <sel>` | clears the adj-rib-in |

The selector is the line's neighbor, or every peer the process serves. A
selector that matches no peer is answered `error`, as ExaBGP answers "no
neighbor matching the command".

`clear adj-rib out` also nets the batch it is read in. ExaBGP's withdraw pops
every route still queued for the peer, so a route the script wrote for the
cleared peers earlier in the same read never reaches the wire, and a route
written after the clear is queued after it. `Net` cancels the earlier route
commands whose selector the clear covers, and nets each run between clears on
its own, so the clear dispatches where it was written. A route aimed wider
than the clear (written for `*`, cleared for one neighbor) is kept: it reaches
the wire and the clear then withdraws it from the cleared peer
(`docs/exabgp/exabgp-differences.md`). `api-rib` and `api-rr-rib` are the cases.

<!-- source: internal/exabgp/bridge/bridge_command.go -- convertControl -->
<!-- source: internal/exabgp/bridge/bridge_batch.go -- Net, netRun, adjRIBOutClearCovers -->

## A bare address is a host route

ExaBGP writes a host route as an address alone. Its `prefix` parser splits on
`/` and takes 32 on failure, or 128 when the address holds a colon, and the API
reuses that parser. So `announce route 1.2.3.4 next-hop 5.6.7.8` means
1.2.3.4/32.

Ze reads a prefix, and answered `invalid prefix: 1.2.3.4`. The bridge therefore
completes the token before it builds the command. It does so only for the SAFIs
whose NLRI is a plain prefix: `unicast`, `multicast`, `mpls-label` and
`mpls-vpn`. The set is keyed by ze's family words, because the bridge has
already built the family for update text: a route with a bare `label` and no
`rd` is sent under `mpls-label`, ze's name for SAFI 4, which ExaBGP calls
`nlri-mpls`. A family whose NLRI is a field list is left alone, because `mvpn`
writes `shared-join rp 10.99.199.1 group 239.251.255.228` and those bare
addresses are values rather than prefixes.

The set is an allow-list rather than a subtraction, so a SAFI added later gets
no prefix length until somebody decides it should.

<!-- source: internal/exabgp/bridge/bridge_route_forms.go -- defaultPrefixLengths, bridgePrefixSAFI -->

`convertRoute` is the one place that decides what the bridge translates, and it
answers the same set of forms for a line that names a neighbor and for a line
that does not. The set is the ExaBGP vocabulary: `announce route`,
`withdraw route`, `announce ipv[46] <safi>` and `withdraw ipv[46] <safi>`, with
the SR-Policy shape of each, and `announce vpls` and `withdraw vpls`.

An announce and its withdraw differ in ONE token, the NLRI verb, so each pair is
one function taking that verb: `convertFamilyRoute` for a family-stating route
and `convertFlowSpec` for `flow route { ... }`. Both pairs were two functions and
both had diverged, dropping on the withdraw side what the announce side read. The
FlowSpec pair discarded the `then { ... }` block, so
`withdraw flow route { match { ... } then { rate-limit 1; } }` reached ze with no
rate-limit extended community.
<!-- source: internal/exabgp/bridge/bridge_command.go -- convertFamilyRoute, convertFlowSpec -->

## `help` is the only line that passes through

A line the translator reads becomes a ze command. Every other line is REFUSED,
and the refusal quotes the line the script wrote. `help` is the one exception: it
is the bridge's own word rather than a route, ze declares it as `ze-meta:bgp-help`,
and it is spelled the same on both sides.

The passthrough used to be wider, and what it carried is gone. ze declared a bare
form for each `announce` and `withdraw` command, a script reached those forms
through the passthrough, and no ExaBGP spelling collided with one of them because
ExaBGP spells its own forms `route` or a family. Those six forms are translator
output now: `announce route <prefix>` becomes `send bgp * unicast ...` and the
family forms follow, so a genuine ExaBGP script reaches them by writing ExaBGP.
That is a gain rather than a break, because the bare ExaBGP form did not work at
all before the translator learned it.

What remained of the passthrough after that was an untyped path from a script's
stdout to ze's dispatcher, by which a script could type any ze command. A line
sent down it died as an unknown command with no mention of the bridge.

Removing it is a behavior change, and this page is where it is recorded: the
bridge no longer carries a way for a script process to reach an arbitrary ze
command. Nothing in the repository used that path
(`spec-fixit-send-names-its-destination` enumerated the lines a script can send
before the narrowing landed).

| Wire method | How a script reaches it | What a rename costs |
|---|---|---|
| `announce-unicast`, `announce-blackhole`, `announce-flowspec`, `withdraw-tag`, `withdraw-id`, `withdraw-all` | translator output, from `announce route`, `withdraw route` and the family forms. An operator types the ze spelling at ze's own CLI | nothing for a script |
| `help` | passthrough, with one spelling on both sides | a break for a script that asks for help |
| `peer-update` | translator output, from `neighbor <address> announce` | one edit in the translator, done: the translator writes `send bgp <selector> update text ...` |
| `peer-raw` | neither, because the translator never writes the word `raw` | one edit, done |

`ze-meta:bgp-help` is the one member `bridgeSurface` keeps, so the eight BGP methods
above it are checked by the verb-first grammar gate rather than exempted from it.

## `ping` is answered by the bridge

ExaBGP 6.0's health check, `ping` (v6 spelling `session ping`), reaches no ze
command: the bridge answers it, as it answers the ack words. The answer is
ExaBGP's bytes, then `done`:

| Line | Answer line |
|------|-------------|
| `ping`, `session ping`, `ping <client-uuid> <client-start-time>` | `{"pong": "<uuid>", "active": true}` |
| any of these with the word `text` | `pong <uuid> active=true` |

The UUID is a version 4 UUID drawn once per process, as ExaBGP draws its
`daemon_uuid` once per reactor, so every script of one ze daemon reads the same
value. `active` is always true: ExaBGP keeps no list of clients and answers
true. The answer line is written whatever the ack state, and only the `done`
follows it, because ExaBGP writes the pong with `processes.write` and the
`done` with `answer_done_sync`.

<!-- source: internal/exabgp/bridge/bridge_ping.go -- writePong, pingAction, daemonUUID -->
<!-- source: internal/exabgp/bridge/bridge_ack.go -- AnswerLocal -->

## `status` is answered by the bridge

`status` (v6 spelling `daemon status`), the other half of ExaBGP's health pair,
is answered by the bridge too, with ExaBGP's bytes, then `done`. The answer is
JSON when the script is held to API 6 or the line ends in `json`, and text
otherwise, as ExaBGP's `_answers_in_json` decides:

```
ExaBGP Daemon Status
====================
Version: 6.0.0
UUID: <the ping UUID>
PID: <ze's pid>
Uptime: 0h 1m 5s
Peers: 1
  - neighbor 127.0.0.1 local-ip 127.0.0.1 local-as 65535 peer-as 65535 router-id 127.0.0.1 family-allowed in-open: ESTABLISHED
```

```
{"version": "6.0.0", "uuid": "<uuid>", "pid": 4242, "uptime": 65, "start_time": 1760960000.2499998, "peers": {"<name>": "ESTABLISHED"}}
```

The peer list is read from `show bgp peer * detail`. Each name is ExaBGP's
`Neighbor.name()`, with `auto` for an absent local address or AS, and each state
is the peer's `fsm-state`, the RFC 4271 Section 8.2.2 state as ExaBGP prints it
(`IDLE`, `ACTIVE`, `CONNECT`, `OPENSENT`, `OPENCONFIRM`, `ESTABLISHED`). ExaBGP
lists its peers in configuration order; ze's answer is a map keyed by address,
so the bridge lists them by address. A row without its state answers the line
with `error`, never with a guessed state.

Only the exabgp-bridge plugin answers it. The standalone runner, `ze exabgp
plugin`, reads ok or an error from each dispatch and only the answer's first
record, never ze's peer list, so it refuses `status` by name in its log and
answers `error`.

<!-- source: internal/exabgp/bridge/bridge_status.go -- StatusPeers, StatusReply, StatusInJSON, errStatusStandalone -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.status -->

<!-- source: internal/component/command/grammar/checker.go -- bridgeSurface, ExemptCategory -->

## A neighbor command is one of three things

ExaBGP's API drives a neighbor as well as its routes. The three verbs do not
share one answer, and `convertNeighborControl` decides which answer each one
gets.

| ExaBGP line | Answer |
|---|---|
| `neighbor <ip> teardown [<code>] [<subcode>] [<text>]` | `request peer <ip> notify <code> <subcode> [hex <octets>]` |
| `neighbor <ip> disable [<message>]` | `request peer <ip> disable [<message>]` |
| `neighbor <ip> enable` | `request peer <ip> enable` |
| `create neighbor <ip> ...` | `create bgp peer <ip> asn <asn> ...` |
| `delete neighbor <ip>` | `delete bgp peer <ip>` |
| `neighbor <ip> receive update ...` | refused: this is an event, not a command |

The teardown grammar is the pinned ExaBGP's `teardown_notification`
(`neighbor.py`). The words after the verb are split the way ExaBGP splits them
(quotes group words). Up to two leading all-digit words are numbers, each from
0 to 255: none means Cease, Administrative Shutdown (6, 2), one is a Cease
subcode, two are a code and a subcode. The words left, joined with one space,
are the text. Every form reaches ze's `notify`, with the code and the subcode,
and the Data field in its `hex` form. ze's own `teardown` is not used, because
it sends a Cease only and names the subcode when no text is given.

The bridge builds the Data field the pin's `Notify._wire_text` builds
(`teardownWireData`), so for an ASCII text the peer receives the octets
ExaBGP 6 sends. ExaBGP 6 reads a script's stdout as ASCII and drops the process
on any other octet, while the bridge sends the NOTIFICATION
(`docs/exabgp/exabgp-differences.md`, "Refusals and keep-first"). Under
Cease 2 and 4 it is the RFC 9003 Shutdown Communication: a length octet, then
the text's UTF-8 octets cut to the pin's 128 (`SHUTDOWN_COMM_MAX_LEGACY`, RFC
9003 Section 3), back to the last whole character. Under every other pair it is
the text encoded as Python's `encode('ascii', 'backslashreplace')` encodes it:
an ASCII character as it is, any other as `\xNN`, `\uNNNN` or `\UNNNNNNNN`. No
text sends an empty Data field, and every Data field is cut to the pin's 4075
octets (`DATA_MAX_OCTETS`). The hex form sends those octets as they are, so the
text never passes through ze's command tokenizer: a `"`, a backslash and a run
of spaces reach the peer as the script wrote them. The refusals carry ExaBGP's
own text: `expected a code or a subcode, got '<word>'`, `'<n>' is not a value
from 0 to 255`, and `No closing quotation`. The `api-teardown` compatibility
test passes against ze and against the pinned ExaBGP alike.
<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- convertTeardown, teardownWireData -->
<!-- source: internal/component/bgp/plugins/cmd/peer/notify.go -- handlePeerNotify, notifyData -->

ExaBGP 6 `disable` and `enable` (`neighbor.py`) map one to one: `disable` holds the
peer down and sends Cease subcode 2 with the rest of the line, split the way
ExaBGP splits it (quotes group words), as the RFC 9003 Shutdown Communication;
`enable` releases the hold and takes nothing after it.
<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- convertDisable, convertEnable -->

`create neighbor` reaches `create bgp peer`, the command `ze-bgp:peer-create`
answers. The peer ze builds lives in the running daemon, and like ExaBGP's own
dynamic peer it is not written to the configuration file. A commit or a reload
whose configuration does not declare it keeps it running. `delete neighbor`
reaches `delete bgp peer`, the counterpart `ze-bgp:peer-delete` answers.

Each parameter maps to the `create bgp peer` keyword that means the same thing.

| ExaBGP parameter | ze keyword |
|---|---|
| `local-address`, `local-ip` | `local-address` |
| `local-as` | `local-as` |
| `peer-as` | `asn` |
| `router-id` | `router-id` |
| `family-allowed ipv4-unicast/ipv6-unicast` | `family ipv4/unicast,ipv6/unicast` |
| `graceful-restart <seconds>` | `graceful-restart <seconds>` |
| `group-updates true\|false` | `group-updates true\|false` |
| `api <process>`, repeated | `attach exabgp-bridge`, once |

Every ExaBGP process is a script the `exabgp-bridge` plugin runs, never a ze
process of its own, so `api <process>` attaches that plugin whatever the process
is called. Attaching it under the script's name named a process ze does not run:
the created peer then attached nothing and refused every route the script sent
it, which is what `api-peer-lifecycle` caught.

Four lines are refused, and each refusal names what it refused rather than
dropping it. `family-allowed in-open` is ExaBGP's "let the OPEN decide the
families", and ze states the families its OPEN offers, so the word names no ze
behavior. A parameter the table does not carry is refused by name. A line
missing `local-address`, `local-as` or `peer-as` is refused, because ExaBGP
refuses it too, so the script reads the same answer from either daemon. And
`delete neighbor <ip> <filter>` is refused, because `delete bgp peer` takes the
selector alone and a filter would reach a peer the script did not name.

`receive` is not a command. It is ExaBGP's own EVENT vocabulary, written by the
text response encoder, and it travels UP from the daemon to the script. The
`api-check` script reads such a line on its stdin. The refusal names the
direction, so that a reader opens the event encoder rather than the translator.

<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- convertNeighborControl, convertTeardown -->

## One write is one batch, and a batch nets before it reaches the wire

An operator migrating an ExaBGP script writes several API commands in ONE
`write(2)`. ExaBGP reads its processes once per reactor cycle and holds what
that read returned in its outgoing RIB, where the commands cancel each other
before anything is encoded. A script that writes four lines in one write
therefore puts the frames of ONE netted set on the wire, not four.

The bridge is the same unit for ze. Its reader takes the complete lines of one
read, carries any partial line into the next batch, and hands the batch to a
dispatcher. Five rules then decide what leaves.

Each row states what the batch DISPATCHES. What a dispatched withdrawal then
puts on the wire is the reactor's answer, not the netting's: a session that has
advertised nothing is written nothing at all
(`docs/architecture/update-building.md`, "A Withdrawal Names a Route This
Connection Advertised"). The rules together are what make `api-fast` batch 1
produce one frame.

| Rule | What it does |
|------|--------------|
| A route is withdrawn at most ONCE per batch | `withdraw X`, `announce X`, `withdraw X` dispatches one withdrawal, and it answers the line that wrote it first |
| A withdrawal cancels an announce of the same route EARLIER in the batch | `announce X` then `withdraw X` dispatches the withdrawal alone, and the announce never reaches ze |
| An announce cancels nothing | `withdraw X` then `announce X` dispatches both |
| Withdrawals dispatch before announces | whatever order the script wrote them in |
| A command that carries no route keeps its write order | an End-of-RIB names no route, so it cancels nothing and nothing cancels it |

The first rule is upstream's dict: a withdrawal is queued under the NLRI index,
so a second write of the same line replaces the first rather than adding to it.
`api-flow` is the recording. Its script withdraws one flow route, announces it,
withdraws it again and announces it again, all in one write, and reads back one
withdrawal and one announce.

The route each command carries is stated by the translator that WROTE the
command, on `Command.Key`. No consumer reads it back out of the command text: a
second reading of the `send bgp` grammar drifts the moment that grammar moves,
with no test red.

The reader does no wire work. It hands each batch to the dispatcher goroutine
and returns to the pipe at once, so the next write a script makes becomes its
own batch rather than joining the one still being dispatched. That is what keeps
ze's batches no coarser than ExaBGP's, which is the standard the compatibility
fixtures were recorded against. `api-fast` is the recording: its first write of
four lines produces ONE frame, and its second write of three produces two, with
the withdrawal first.

<!-- source: internal/exabgp/bridge/bridge_batch.go -- BatchReader, Net -->
<!-- source: internal/exabgp/bridge/bridge_command.go -- Command, RouteKey -->

## A group is applied at its end

ExaBGP 6.0 lets a script send several routes as one set. `group start` opens a
block; every line after it that starts with `announce` or `withdraw`, and an
API 6 `peer <selector> announce|withdraw` of a route kind (`route`, `ipv4`,
`ipv6`, `flow`, `vpls`, `attributes`), is held and answered `done` alone.
`group end` applies them. `group <command> ; <command>` is a group of its own
on one line, for every peer of the script unless `peer <selector> group ...`
names them. The line is split where ExaBGP's lexer ends a statement: at a `;`
outside quotes and braces, read with its backslash escapes resolved, so a `;`
inside a quoted word cannot start a command. An End-of-RIB or a route-refresh
written as API 6 is sent as it comes.

The first `attributes ...` command of a group gives its attributes to every
route after it that does not write them, and its next-hop to every route that
gives none. The routes of a closed group are translated, netted and sent as one
set, and route commands that differ only in their one prefix are sent as ONE ze
command, so they leave in one UPDATE as they do from ExaBGP's RIB. A route
whose NLRI is several words (a flow rule, an RD and a label) is never run
together with another.

The answers are ExaBGP's own (`command/group.py`), in text, or in JSON for an
API 6 script or a line ending in `json`:

| Line | Answer |
|------|--------|
| `group start` | `group started`, then `done`; inside an open group `error: already in group block (nested groups not allowed)`, then `error` |
| `group end` | `group processed: <n> announced, <m> withdrawn`, then `done`; with `, <k> errors` and `error` when a command could not be applied (the others are); `group ended (0 commands)` for an empty group; `error: not in group block` with none open |
| `group a ; b` | as `group end`; `error: empty group` when it holds no command |

A group holds at most 100000 commands and 100 MiB, ExaBGP's bounds, and is
discarded past either. Each script has its own group, emptied when its child
exits. Every runner reads its lines through one shared path,
`bridge.ScriptLines.Batch`, and differs only in how it hands ze a command, so the
group, the netting and the answers cannot drift between them. `api-group` and
`api-group-inline` are the cases.

<!-- source: internal/exabgp/bridge/bridge_group.go -- Group.read, SplitCommands, Translator.group, mergeShared, groupApply.answer -->
<!-- source: internal/exabgp/bridge/bridge_lines.go -- ScriptLines.Batch, ScriptLines.applyGroup, ScriptLines.dispatch, mergeHead -->

## The selector travels with the command

After a batch's route commands, the bridge injects one flush per selector the
batch reached and blocks until each forward pool drains. The peer it flushes is
`Translation.Selector`, the selector the translator used when it built the
command. A list such as `<a>,<b>` drains every peer it names
(`handleBgpPeerFlush`). A flush ze refuses or does not answer fails every route
line of that selector, so the script reads `error`, never `done` for a barrier
that did not hold (`failSelectorLines`).

It was read back out of the finished command until 2026-09-05, by
`ExtractPeerAddress`, which required a literal leading token and answered the
empty string for anything else. Its three callers read that empty string as
"nothing to flush" and continued, while `IsRouteCommand` answered on the
substring `update text`, which survives any change of leading token. So the two
disagreed the moment the token moved: every route was still recognized as one,
and every flush after it was skipped, with no error and no log line. Both helpers
are gone. The fact is stated once, by the builder that already knew it.

<!-- source: internal/exabgp/bridge/bridge_command.go -- Translation, TranslateLine -->
<!-- source: internal/exabgp/bridge/bridge.go -- pluginToZebgp -->

## The bridge runs EVERY process the config declares

An ExaBGP config declares any number of `process <name> { run ...; }` blocks,
and a neighbor names the ones it wants in `api { processes [ a b ] }`. The ze
side declares the same set:

```
exabgp {
    bridge {
        process public {
            run "./run/api-multiple-public.run"
            encoder text
            feed 127.0.0.1 {
                event [ receive-update send-update ]
            }
        }
        process private {
            run "./run/api-multiple-private.run"
            respawn disable
            feed 192.168.0.1 {
                event [ ]
            }
        }
    }
}
```

The block name is the ExaBGP process name, because that is the name a neighbor's
`api { processes [ ... ] }` list refers to. A block with no `run` command is
REFUSED and the refusal names it.

The bridge forks one child for each block and reads commands from EVERY child's
stdout. It fans each event out to the children the event's peer FEEDS. The ack is
per child: each script writes a line and blocks for its own `done`, and
`disable-ack` from one script silences that script alone.

Until 2026-09-06 the config carried one `run` leaf and the bridge forked one
child, so a config declaring two processes ran one of them and said nothing.
That is the silently-wrong answer `ai/rules/principles.md` exists to prevent,
and the three ported compatibility tests that name two processes
(`api-no-respawn`, `api-multiple-api`, `api-api`) are what measured it.

## `feed` is the relation ExaBGP models on the neighbor

An ExaBGP `api` block names BOTH the processes a neighbor feeds and the message
kinds it feeds them. The relation is therefore per neighbor AND per process. Two
neighbors can grant one process different events. A per-process event list could
not say that, and the union of two grants over-delivers to the neighbor that
granted less. So the ze side writes one `feed` block per peer, inside the
process block.

The event names are ExaBGP's own api keys: `receive-update`, `send-open`,
`send-keepalive` and their siblings, each built from the direction and
`Message.CODE.SHORT`. The four standalone words `neighbor-changes`,
`negotiated`, `fsm` and `signal` join them. `bridge.Event.APIKey` answers the key
for one event, and the fan-out compares the two, so there is no second table.

The two empties are DIFFERENT and the difference is load-bearing.

| Config | What the script is fed |
|---|---|
| no `feed` block at all | every event of every peer, which is what one script and one neighbor means |
| a `feed` block with an empty `event` list | nothing from that peer |

The second row is ExaBGP's own answer to `api { processes [ p ]; }` with no
direction block. Its `flatten` reads an absent message kind as a refusal, not as
a default.

Until 2026-09-06 every script got every event of every peer. `api-check` is the
case that measured it. Its neighbor grants `receive-update` and `send-update`
alone. Its script exits 1 on the first line that is not the announce it waits
for, so the session's OPEN was enough to fail it.

<!-- source: internal/exabgp/migration/migrate_api_events.go -- apiBlockEvents -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.receives, grantSet -->

## A route is acked after its flush

A route command owes the script two answers, and their order is the contract.
`done` means the command is done. A route is not done until it is on the wire.
So the bridge dispatches the per-peer flush FIRST and acks after it. ExaBGP
orders them the same way: `announce_route` awaits every peer's flush event and
calls `answer_done` after it.

Acking first let a script send its next command while the previous route's flush
was still running. The two then reached the wire in whichever order they
finished. `api-ipv4`, `api-ipv6`, `api-mvpn` and `api-vpnv4` each announce and
withdraw one NLRI and read the frames in order. Each of them caught it.

A batch keeps that rule. Every command of the batch dispatches, then one flush
for each selector the batch reached, then one answer for each LINE the script
wrote. The answers run in write order for a second reason: `disable-ack` is
itself acked and silences what FOLLOWS it, so a batch answered in dispatch order
would silence the wrong lines. One line whose dispatch failed is answered
`error` on its own and the rest of the batch keeps its answers, because a script
blocked for an answer that never comes stops there.

A refused line, whether ze's dispatcher refused it, the translator could not
read it, or the script's API version does not know it, is answered with its
reason and then the `error` marker, as ExaBGP's `answer_error_sync` writes it:
`{"error": "<why>"}` (quoted as Python's `json.dumps` quotes it) to a script
held to API 6, `error: <why>` to an API 4 or undecided one. The form is the
version the script was held to when it wrote that line (`BatchLine.Version`),
because ExaBGP answers a line before it reads the next. With ack disabled
neither line is written. ExaBGP writes the marker alone for an unknown command
and a selector matching no peer; Ze gives the reason there too
(`docs/exabgp/exabgp-differences.md`, "A refused line names its reason").

<!-- source: internal/exabgp/bridge/bridge_batch.go -- AnswerBatch, BatchSelectors, BatchLine.Version -->
<!-- source: internal/exabgp/bridge/bridge_ack.go -- AckMode.WriteError -->
<!-- source: internal/exabgp/bridge/bridge_lines.go -- ScriptLines.Batch, ScriptLines.answer -->

## FlowSpec value lists

ExaBGP 6.0 reads `destination-port [ =80 =443 ]` as 80 OR 443, and
`>8080&<8088` as one AND term (configuration/grammar/types/flow.py,
`_bracketed` and `_expression`). Ze's update text reads successive values as
OR and `&` as AND (`docs/architecture/api/architecture.md`, "FlowSpec NLRI"),
so the bridge drops the brackets and passes every term through unchanged, in
the family form (`announce ipv4 flow ... destination-port [ =80 =443 ]`) and in
`flow route`: both reach ze as `destination-port =80 =443`. Before 2026-10-21
ze's update text ANDed successive numeric values, so such a route was sent as
80 AND 443, which matches no packet, and the family form sent the brackets,
which ze refused.
<!-- source: internal/exabgp/bridge/bridge_command.go -- parseFlowSpecBridgeRoute -->
<!-- source: internal/exabgp/bridge/bridge_flow.go -- flowRouteList, readComponent -->

## FlowSpec redirect and copy

A `flow route` line's redirect and copy actions follow the ExaBGP 6.0 grammar
(configuration/grammar/types/flow.py), the same table the migration follows
(`docs/architecture/config/exabgp-syntax.md`): `redirect <ip>`,
`redirect-to-nexthop <ip>` and `redirect-to-nexthop-ietf <ip>` send
`redirect-to-nexthop <ip>`, `copy <ip>` sends `copy-to-nexthop <ip>`, and the next
hop is left alone; `redirect-simpson <ip>` and `copy-simpson <ip>` set the next hop
and send the 0x0800 `redirect-to-nexthop-draft` or `copy-to-nexthop`; a bare
`redirect-to-nexthop` and `redirect-to-nexthop-simpson` send
`redirect-to-nexthop-draft`. An IPv6 address goes in IPV6_EXTENDED_COMMUNITIES
(`docs/architecture/api/update-syntax.md`).
`test/exabgp-compat/api/api-flow-redirect.ci` holds `redirect <ipv4>`,
`copy <ipv4>`, `redirect [<ipv6>]` and `redirect-simpson <ipv4>` to the octets
the ExaBGP pin sends.
<!-- source: internal/exabgp/bridge/bridge_flow.go -- readRedirect, readIETFRedirect, readSimpson -->

## What the bridge refuses, and what it keeps

ExaBGP 6.0 refuses some lines 5.0 took, and keeps 5.0's leniency on the API
for two others. The bridge answers each the way ExaBGP 6.0 does.

| Line | Answer |
|------|--------|
| A route no selected peer has the family of | refused with ze's reason |
| A selector that names no peer this process feeds | refused, nothing dispatched |
| `flow route` with no `match` | refused |
| A flow route, braced or flat, announce or withdraw, matching an ICMP type or code and a port | refused before ze sees it, answered `error` as ExaBGP 6.0 answers it (its `propagated` drops the route as the line is parsed, so `api_flow` returns none), with a warning naming the route: RFC 8955 Section 4.2 says no packet matches it and it SHOULD NOT be propagated |
| `flow route` with `mark` or `action`, or `redirect [<ipv6>]:<nn>` (the IPv6 route-target redirect) | refused, naming the word: `update text` has no spelling for it |
| `copy`, `redirect-simpson`, `copy-simpson` or `redirect-to-nexthop-ietf` given something that is not an address | refused, naming the value |
| An `interface-set:` item inside `extended-community`, on a flow route or any other route | refused, naming the `interface-set` keyword to write it with: ExaBGP has no such extended community type |
| An interface-set whose transitivity is neither `transitive` nor `non-transitive`, or whose AS number holds a period | refused, transitivity judged first as ExaBGP does |
| An attribute written with no value (`path-information`) | refused, naming the value to write |
| `aggregator` with more than one word, or with a bracket left open, doubled or unmatched (`aggregator ( 65001:192.0.2.1`, `aggregator 65001:192.0.2.1)`) | refused, naming the form to write: one `<asn>:<router-id>`, bare or in brackets, as ExaBGP's `AggregatorType` reads it |
| A prefix whose mask is no number (`10.0.0.0/x`) | refused, naming the mask |
| `announce operational` with an unknown kind, or none | refused, naming the command and the kinds |
| `announce operational` with a known kind | refused: Ze does not send OPERATIONAL |
| An attribute that is not a list written twice (`next-hop 1.1.1.1 next-hop 2.2.2.2`) | the first is sent, the line is done, a warning names the dropped value |
| `community`, `extended-community` or `large-community` written twice | the second list is merged into the first, the line is done, no warning |
| `label`, `rd` or `path-information` written twice | the last is sent, the line is done, no warning |
| A route line holding a `\u` escape without four hexadecimal digits | refused, naming the escape and the two ways to write it |
| `split /<n>` shorter than the prefix | the prefix is sent whole, the line is done, a warning says so |
| `watchdog` or `name` as the last word of a route line | the route is sent with neither, as ExaBGP reads the empty word there; for `watchdog` a warning says so and names the fix, `watchdog <name>` |

The route with no carrying peer is refused by Ze itself, as ExaBGP refuses it:
`update text` asks the reactor whether any selected peer is configured for each
announced family (`SelectedPeersCarry`, the families Ze's OPEN offers that
peer) and answers an error before any route of the command is sent. A route a
selected peer is configured for is accepted whatever its session state, which
is ExaBGP's answer too; when no session has the family negotiated Ze answers
status `warning`, and the bridge answers `done`. The standalone runner releases
a dispatch at the answer's terminator, whose message is the failure.
`api-family-carry-v4` covers both directions on a down peer.

Keep-first is the bridge's own work. Ze's update-text parser keeps the LAST of
a repeated attribute, so the bridge forwards only the first, keyed on the ze
keyword. The warnings go to the bridge's log as `exabgp-bridge route line`.
Keep-first covers only the attributes that are not lists. ExaBGP 6.0 merges a
list given twice (`LIST_ATTRIBUTES` and `add_attribute`,
`configuration/grammar/tree/static.py`), so the bridge merges the communities,
the extended communities and the large communities into the first value. A
repeated large community stays in the merged list, which ExaBGP drops as it
merges: Ze drops it on the way out, as RFC 8092 Section 3 requires
("Duplicate BGP Large Community values MUST NOT be transmitted"). A label, an
RD and a path identifier are fields of the NLRI, which ExaBGP sets as it reads
them, so the last one written is the route's. ExaBGP 4.2 merged no list and
5.0 merged only the extended communities. The bridge follows 6.0, so a 4.2 or
5.0 script meets what it would meet on upgrading ExaBGP.

A route line's backslash escapes are resolved before it is split into words,
as every ExaBGP release reads a route command (`unescape` in
`configuration/grammar/lexer.py`; `tokens` in 4.2 and 5.0). `\b`, `\f`, `\n`,
`\r` and `\t` are those characters, `\uXXXX` is the character it names, and
any other escaped character is itself, so `med 100\n` sends MED 100. Only a
route, flow, vpls or attributes line is read that way. A neighbor control, a
refresh or an operational line reaches its converter with its backslashes as
written, as ExaBGP reads those with `shlex.split` on the raw text.

An SRv6 service group that names a SID and no endpoint behavior
(`bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: )`) is sent with the
behavior 0xFFFF, ExaBGP's default for that group. Ze's update text reads an
omitted behavior as 0, so the bridge writes it out. RFC 9252 Section 3.1: "The
opaque SRv6 Endpoint Behavior (i.e., value 0xFFFF) MAY be used when the
advertising router wishes to abstract the actual behavior of its locally
instantiated SRv6 SID."

The label-index Prefix-SID, `bgp-prefix-sid [ <index> ]` or
`bgp-prefix-sid [ <index>, [ ( <base>,<range> ) ... ] ]`, is passed through
unchanged: update text reads the same keyword in the same spelling and writes
the Label-Index and Originator SRGB TLVs (RFC 8669 Sections 3.1 and 3.2).

`api-refusals-v4`, `api-repeated-attributes-v4` and `api-route-forms-v4` are
the cases.

<!-- source: internal/exabgp/bridge/bridge_dispatch_answer.go -- terminatorResult -->
<!-- source: internal/component/bgp/reactor/reactor_api_carry.go -- SelectedPeersCarry -->
<!-- source: internal/exabgp/bridge/bridge_attribute.go -- parseRouteAttributes, warnRouteLine, attrValueName, mergedListValue, srv6WithBehavior -->
<!-- source: internal/exabgp/bridge/bridge_escape.go -- resolveLineEscapes, errBadEscape -->
<!-- source: internal/exabgp/bridge/bridge_route_forms.go -- splitPrefix, refuseMaskNotNumber -->
<!-- source: internal/exabgp/bridge/bridge_operational.go -- convertOperational -->
<!-- source: internal/exabgp/bridge/bridge_command.go -- Translator.servedSelector, errSelectorMatchesNoPeer -->

## `encoder` says which format a script reads

`encoder` is ExaBGP's leaf. Ze declares it in the same place, inside the process
block, so one bridge can feed a text script and a JSON script at once. An absent
leaf is `text`, ExaBGP's own default (`configuration.get('encoder', 'text')`).

The leaf is read together with the script's API version, as ExaBGP main reads it
since e2142a435. A script is held to the API of the first command that tells
(`command_api_version` in ExaBGP's `dispatch/version.py`, `bridge.CommandAPIVersion`
here): a first word in `daemon`, `session`, `system`, `rib`, `peer` is API 6;
`neighbor`, or a word ExaBGP's `translate_v4_to_v6` translates (`announce`,
`withdraw`, `teardown`, `help`, `api version`, `show adj-rib in`, ...), is API 4;
anything else, `group` and a typing mistake included, decides nothing. The hold
lasts until the script exits: a respawned child starts from the setting again.

| API version | `encoder text` (or absent) | `encoder json` |
|-------------|----------------------------|----------------|
| undecided | 5.x text | 6.0 JSON, envelope `6.0.0` |
| 4 | 5.x text | 5.x JSON, envelope `4.0.1` |
| 6 | 6.0 JSON, and one warning that text is ignored | 6.0 JSON |

A held script writing the other API's command is answered with the reason and
`error`, and the line reaches no peer, as ExaBGP answers its `UnknownCommand`. `environment { exabgp {
api { version auto|4|6; } } }` is ExaBGP's `exabgp.api.version` (read under
ExaBGP's spellings, `exabgp.api.version` before `exabgp_api_version`): auto detects per
script, 4 or 6 holds every script from its start, and a value ExaBGP refuses
stops the bridge at start. A script's `api version` line answers `API version:
<setting> (this helper: <version>)`, and `api version <v>` moves the setting for
the scripts started afterwards, ExaBGP's answers byte for byte. The fan-out still
renders each event once per format in use, three at most.

The version state is one type, `bridge.ScriptVersion`, and every runner holds
one per script, so no runner answers a script differently from another. That
includes the standalone runner, `ze exabgp plugin <command>` started with no hub
token, which speaks MuxConn on its own stdin and stdout (`bridge.Bridge`). It
reads `exabgp.api.version` from its environment at each start, detects and
refuses as above, and answers `api version` itself. Like the external runner it
has no process block, so its script reads JSON: 6.0 JSON while undecided or held
to 6, 5.x JSON (envelope `4.0.1`) once held to 4.

The two APIs read an OPEN and a NOTIFICATION differently, as ExaBGP main writes
them (`response/json.py`, `Notification.v4_data`):

| Field | API 4 (JSON, and the text line) | API 6 JSON |
|-------|---------------------------------|------------|
| `open.capabilities` | keyed by code, `"1"`, `"65"`, body without `code` | keyed by name, `"multiprotocol"`, `"asn4"`, `code` inside the body |
| `notification.data` | hex of the Shutdown Communication decoded (`Shutdown Communication: "bye"`), else of the wire | hex of the wire, length octet included |
| `notification.message` | the Data field made readable | the same |
| `label` of a labeled or VPN route | `[[100]]`, the label alone | `[[100, 1601]]`, with the raw label entry |
| `string` of a flow route | ends with ` next-hop <ip>` when the route has one | the rules alone |
| a labeled or VPN route on a text line | `10.0.0.0/24 label 100 next-hop 1.2.3.4 rd 65000:1` (labels without raw value, next hop before the RD) | (no text) |

Hex is ExaBGP's `hexstring`, `0x` and uppercase. ze's OPEN event carries each
capability as a code, a name and a value, and the bridge rebuilds ExaBGP's body
per code from it: the value ze structures for six capabilities, the payload in hex
for every other one. `asn` is the two-octet My AS field, so a four-octet AS reads
23456, AS_TRANS, as in ExaBGP. ze's event omits an ADD-PATH family whose mode RFC
7911 does not define, which ExaBGP shows as `disabled` or `invalid (<n>)`.

A word that names neither format is REFUSED, by the config parser and by the
migration. Each of them names the process and the word. Until 2026-09-06 the
leaf was parsed by nothing at all: a config asking for text got JSON, with no
error and no log line.

<!-- source: internal/exabgp/bridge/bridge_encoder.go -- Encoder, ParseEncoder -->
<!-- source: internal/exabgp/bridge/bridge_version.go -- CommandAPIVersion, FormatFor, APIVersionRequest.Answer -->
<!-- source: internal/exabgp/bridge/bridge_event_open.go -- Event.openJSON, openCapabilitiesJSON, capabilityBody -->
<!-- source: internal/exabgp/bridge/bridge_event_notification.go -- notificationData.v4Data, notificationData.readable -->
<!-- source: internal/exabgp/bridge/bridge_event.go -- exabgpNLRIObject, labelValuesOnly, appendFlowStringNextHop -->
<!-- source: internal/exabgp/bridge/bridge_event_text.go -- labeledText.appendAnnounced, labeledText.appendQualifier -->
<!-- source: internal/exabgp/bridge/bridge_script_version.go -- ScriptVersion.Detect, ScriptVersion.Reset, Event.Line -->
<!-- source: internal/exabgp/bridge/bridge.go -- Bridge.Start, Bridge.dispatchBatch, Bridge.translateAndForward -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- parseEncoder, parseFeeds -->

<!-- source: internal/plugins/exabgp/bridgerun/fleet.go -- Fleet.Start, Fleet.Broadcast -->
<!-- source: internal/exabgp/bridge/bridge_lines.go -- ScriptLines.Batch, LineRunner -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.batch, scriptRunner, script.writeLoop -->

## A script that exits is started again, unless `respawn` says otherwise

`respawn` is ExaBGP's leaf and ze keeps its behavior. The default is true: a
script that exits is forked again the moment ze reads the EOF, with no backoff.
`respawn disable` leaves the script stopped, which is what a script that does
its work once and exits needs.

The restart is rate-limited exactly as ExaBGP rate-limits it. ExaBGP buckets the
clock into 64-second windows and refuses the sixth fork of one process inside
one window, counting the first. A script that dies at once therefore forks five
times, and then stays stopped with an error line naming it.

<!-- source: internal/plugins/exabgp/bridgerun/respawn.go -- respawnLimiter, respawnWindow, respawnMax -->

What becomes of the routes an exited script announced is ExaBGP 6.0's
`on-exit` leaf. `keep` leaves them and `withdraw` withdraws them. Left unset, a
script held to API 4 keeps them, as ExaBGP 4.x and 5.x did, and any other has
them withdrawn (ExaBGP's `Processes._on_exit`). Each exit logs
`exabgp-bridge process exited` with `routes=keep|withdraw` and
`by=on-exit|API 4|API 6|API undecided`. The withdrawal is settled before the
script is started again, so it reaches the wire ahead of anything the new child
announces.

Which routes are the script's is ExaBGP's owner rule: the last script to
announce a route owns it, and a withdraw of it from any script clears it. The
fleet keeps that record for every route command it dispatched, keyed by the
selector and the route, and an exit withdrawing its routes sends each one the
withdrawal its builder wrote beside the announce (`Command.Withdrawal`), then
flushes. Each such withdrawal carries `restore-configured`, so a route that
replaced one the peer's static config holds gives the configured route back
rather than leaving the prefix unannounced, as ExaBGP's `withdraw_owner` does
(`docs/architecture/api/update-syntax.md`); `api-on-exit-configured` is its
case. `api-on-exit` is the case for an API 6 script, and
`api-on-exit-v4-keep` for an API 4 one, whose route is still standing when a
sibling script announces after it exited. `ze exabgp migrate` carries the leaf as
written and leaves it unset when the ExaBGP block did.

The standalone runner (`ze exabgp plugin <command>`, `bridge.Bridge`) follows
the same rule with the same ledger: it notes each route command ze accepted,
and when its one child exits it settles the exit after the child's last batch
is dispatched. It has no process block, so `on-exit` is unset there and the API
version the child was held to decides. It does not respawn the child.

<!-- source: internal/exabgp/bridge/bridge_owner.go -- exitPolicy, OwnerLedger, OwnerLedger.Exited, restoringWithdrawal -->
<!-- source: internal/exabgp/bridge/bridge.go -- Bridge.Run, Bridge.exited, muxRunner.Dispatched -->
<!-- source: internal/component/bgp/reactor/peer_static_restore.go -- restoreConfigured -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.exited -->
<!-- source: internal/exabgp/migration/migrate.go -- exabgpOnExit -->

That is the bridge restarting a SCRIPT. The bridge itself is a ze plugin, and it
declares `failure-policy: restart` in its own Stage-1 registration, so a bridge
process that dies is started again by the engine and its scripts come back with
it. The two are separate mechanisms with the same intent, and both exist because
ExaBGP respawns a process that exits.

<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- bridgeRegistration -->
<!-- source: internal/component/plugin/server/failure_policy.go -- (*Server).applyFailurePolicy -->

## One script cannot stall another

Each child has a queue of at most 1000 lines and one goroutine draining it into
that child's stdin. A child that stops reading fills its own pipe and its own
queue, and the lines that no longer fit are dropped and counted. Without the
queue that child would block the write, which is the SDK event loop for an event
and the fan-out to every sibling for the rest of the fleet.

The queue is also what makes each child's stdin have exactly one writer, so an
event and an ack never interleave inside one line.

<!-- source: internal/plugins/exabgp/bridgerun/script.go -- queueDepth, script.send -->

## The internal runner cannot read the `run` line

The external runner takes its script command from the process-manager `run`
line, so it carries ONE script. A `RunEngine(conn net.Conn)` runner never sees
that line, so the internal runner reads the whole `process` list from the
`exabgp { bridge { ... } }` config root, delivered by the SDK `OnConfigure`
callback at stage 2. This is the one structural difference between the two
runners; everything after it is shared.

`ze exabgp migrate` writes the internal form, so a migrated ExaBGP config always
reaches the runner that carries every process.

The external runner carries no `encoder` and no `feed` either, for the same
reason: a process-manager command line states neither. Its one script is written
in JSON and is fed by every peer. Both are stated at the construction site
rather than left to a zero value.

## Stage ordering constrains the family declaration

Config is not available at stage-1 registration. `familyDecls` therefore
declares the CLI-default family at stage 1, and the `family` leaf refines the
ADD-PATH capability encoding at stage 3, once the config has arrived.

The stage-3 declaration then holds for the bridge's life. A reload that changes
`route-refresh` or `add-path` is refused at config-verify naming `every peer`,
plugin `exabgp-bridge` and the leaf, with the restart steps: ze would otherwise
keep sending the old capabilities in its OPEN.
<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- bridgeCapabilities, capabilityDecls -->
<!-- source: pkg/plugin/sdk/capability_declaration.go -- DeclareCapabilitiesFrom -->

Every other bridge leaf is refused too while the bridge runs, because the
bridge has no live reconfigure path: its scripts keep the config they started
with. A reload that adds or removes a `process` block, changes anything inside
one, or edits the `family` list is refused at config-verify with one line per
changed leaf path (`exabgp bridge process <name>`, `exabgp bridge family`)
saying it cannot change while ze runs, then the restart steps. That check runs
before the capability comparison, so a family edit is refused under the
`family` path even while `add-path` is set. Apply and rollback install
nothing: a candidate verify accepted is one the running bridge already serves.
<!-- source: internal/plugins/exabgp/bridgeplugin/config_tx.go -- verifyReload, changedBridgeLeaves -->
<!-- source: pkg/plugin/sdk/capability_declaration.go -- composeCapabilityVerify -->

## Config root

The settings nest under a top-level `exabgp` root, on an owner decision of
2026-07-09, so the root stays available for later ExaBGP-related
configuration. The registry plugin name stays `exabgp-bridge`.
