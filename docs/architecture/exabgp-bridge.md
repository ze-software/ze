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
`send bgp <address> update text nhop <nh> nlri ipv4/unicast add <prefix>`.
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

The read happens ONCE for the whole fleet, and each format is rendered once. So
the cost of a fan-out grows with the number of formats in use, not with the
number of scripts. Both numbers matter here, because the fan-out runs for every
UPDATE of every peer.

The ExaBGP side is an external contract. An operator writes a script against
ExaBGP, so a change to ze's own CLI does not reach that script through the
translated forms.

<!-- source: internal/exabgp/bridge/bridge_command.go -- TranslateLine, Translation, convertRoute -->
<!-- source: internal/exabgp/bridge/bridge_event.go -- ReadEvent, Event, ExabgpJSON -->
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
neighbor 127.0.0.1 send notification code 6 subcode 2 data 41424344
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

`TranslateLine` matches `neighbor <address> <rest>` with a regular expression. A
line that does not match names no destination. ExaBGP sends such a line to the
peers whose `api { processes [ ... ] }` list names the process that wrote it
(`Reactor.peers(service)`, `src/exabgp/reactor/loop.py`). The bridge reads it the
same way. `Translator.Peers` carries those addresses, so `announce route
<prefix>` becomes
`send bgp 127.0.0.1,192.0.2.1 update text nlri ipv4/unicast add <prefix>`.

A translator with no peers sends to every peer, `send bgp * ...`. That is what
one script and one neighbor means, and it is what the bridge did for every
script until 2026-09-06. Until then a two-process config put each script's
routes on both sessions. `api-multiple-api` is the case for it.

## A bare address is a host route

ExaBGP writes a host route as an address alone. Its `prefix` parser splits on
`/` and takes 32 on failure, or 128 when the address holds a colon, and the API
reuses that parser. So `announce route 1.2.3.4 next-hop 5.6.7.8` means
1.2.3.4/32.

Ze reads a prefix, and answered `invalid prefix: 1.2.3.4`. The bridge therefore
completes the token before it builds the command. It does so only for the SAFIs
whose NLRI is a plain prefix: `unicast`, `multicast`, `nlri-mpls` and
`mpls-vpn`. A family whose NLRI is a field list is left alone, because `mvpn`
writes `shared-join rp 10.99.199.1 group 239.251.255.228` and those bare
addresses are values rather than prefixes.

The set is an allow-list rather than a subtraction, so a SAFI added later gets
no prefix length until somebody decides it should.

<!-- source: internal/exabgp/bridge/bridge_route_forms.go -- defaultPrefixLengths, bridgePrefixSAFI -->

`convertRoute` is the one place that decides what the bridge translates, and it
answers the same set of forms for a line that names a neighbor and for a line
that does not. The set is the ExaBGP vocabulary: `announce route`,
`withdraw route`, `announce ipv[46] <safi>` and `withdraw ipv[46] <safi>`, with
the SR-Policy shape of each.

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
is the bridge's own word rather than a route, ze declares it as `ze-bgp:help`,
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
sent down it died as an unknown command with no mention of the bridge. Removing
it is a behavior change, and it is recorded in Known Limitations of
`plan/immediate/spec-fixit-send-names-its-destination.md`.

| Wire method | How a script reaches it | What a rename costs |
|---|---|---|
| `announce-unicast`, `announce-blackhole`, `announce-flowspec`, `withdraw-tag`, `withdraw-id`, `withdraw-all` | translator output, from `announce route`, `withdraw route` and the family forms. An operator types the ze spelling at ze's own CLI | nothing for a script |
| `help` | passthrough, with one spelling on both sides | a break for a script that asks for help |
| `peer-update` | translator output, from `neighbor <address> announce` | one edit in the translator, done: the translator writes `send bgp <selector> update text ...` |
| `peer-raw` | neither, because the translator never writes the word `raw` | one edit, done |

`ze-bgp:help` is the one member `bridgeSurface` keeps, so the eight BGP methods
above it are checked by the verb-first grammar gate rather than exempted from it.

<!-- source: internal/component/command/grammar/checker.go -- bridgeSurface, ExemptCategory -->

## A neighbor command is one of three things

ExaBGP's API drives a neighbor as well as its routes. The three verbs do not
share one answer, and `ConvertNeighborControl` decides which answer each one
gets.

| ExaBGP line | Answer |
|---|---|
| `neighbor <ip> teardown <subcode>` | `request peer <ip> teardown <subcode>` |
| `create neighbor <ip> ...` | `create bgp peer <ip> asn <asn> ...` |
| `delete neighbor <ip>` | `delete bgp peer <ip>` |
| `neighbor <ip> receive update ...` | refused: this is an event, not a command |

The teardown carries a BGP cease subcode, and that subcode is what goes on the
wire. ze sends Cease, which is RFC 4271 error code 6, and it supplies the RFC
8203 shutdown communication itself when the command gives none. So the ExaBGP
grammar, which carries a subcode and nothing else, needs no message added to it.

`create neighbor` reaches `create bgp peer`, the command `ze-bgp:peer-add`
answers. The peer ze builds lives in the running daemon alone, which is what
ExaBGP's own dynamic peer does: neither writes the configuration file, so a
reload removes the peer. `delete neighbor` reaches `delete bgp peer`, the
counterpart.

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
| `api <process>`, repeated | `attach <process,process>` |

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

<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- ConvertNeighborControl, convertTeardown -->

## One write is one batch, and a batch nets before it reaches the wire

An operator migrating an ExaBGP script writes several API commands in ONE
`write(2)`. ExaBGP reads its processes once per reactor cycle and holds what
that read returned in its outgoing RIB, where the commands cancel each other
before anything is encoded. A script that writes four lines in one write
therefore puts the frames of ONE netted set on the wire, not four.

The bridge is the same unit for ze. Its reader takes the complete lines of one
read, carries any partial line into the next batch, and hands the batch to a
dispatcher. Two rules then decide what leaves.

| Rule | What it does |
|------|--------------|
| A withdrawal cancels an announce of the same route EARLIER in the batch | `announce X` then `withdraw X` puts neither on the wire |
| An announce cancels nothing | `withdraw X` then `announce X` puts both on the wire |
| Withdrawals dispatch before announces | whatever order the script wrote them in |
| A command that carries no route keeps its write order | an End-of-RIB names no route, so it cancels nothing and nothing cancels it |

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

## The selector travels with the command

After a batch's route commands, the bridge injects one flush per selector the
batch reached and blocks until each forward pool drains. The peer it flushes is
`Translation.Selector`, the selector the translator used when it built the
command.

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

<!-- source: internal/exabgp/bridge/bridge_batch.go -- AnswerBatch, BatchSelectors -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.batch -->

## `encoder` says which format a script reads

`encoder` is ExaBGP's leaf. Ze declares it in the same place, inside the process
block, so one bridge can feed a text script and a JSON script at once.

An absent leaf is `json`. The envelope ze declares is 6.0.0, and ExaBGP 6 answers
every process in JSON whatever its leaf says. So JSON is what an unstated encoder
means for a script written against that version. `ze exabgp migrate` writes the
word only when the ExaBGP config carried one. `respawn` follows the same rule.

A word that names neither format is REFUSED, by the config parser and by the
migration. Each of them names the process and the word. Until 2026-09-06 the
leaf was parsed by nothing at all: a config asking for text got JSON, with no
error and no log line.

<!-- source: internal/exabgp/bridge/bridge_encoder.go -- Encoder, ParseEncoder -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- parseEncoder, parseFeeds -->

<!-- source: internal/plugins/exabgp/bridgerun/fleet.go -- Fleet.Start, Fleet.Broadcast -->
<!-- source: internal/plugins/exabgp/bridgerun/script.go -- script.line, script.writeLoop -->

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

## Config root

The settings nest under a top-level `exabgp` root, on an owner decision of
2026-07-09, so the root stays available for later ExaBGP-related
configuration. The registry plugin name stays `exabgp-bridge`.
