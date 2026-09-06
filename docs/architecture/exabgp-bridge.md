# ExaBGP Bridge Plugin

The bridge runs ExaBGP-API scripts against ze's BGP engine. It has two runners
with one behavior: an external SDK-mode process, and an internal in-process
runner. Both hand their scripts to one `bridgerun.Fleet`, so the fork, the
event fan-out, the ack and the respawn are written once.

<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- runInternalBridge, familyDecls -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- exabgp bridge config parsing -->
<!-- source: internal/plugins/exabgp/bridgerun/fleet.go -- Fleet, Script, Broadcast -->

## The bridge translates in two directions

An ExaBGP script writes text commands down to ze, and ze sends JSON events up
to the script. Each direction has its own translator.

Down, `TranslateLine` reads one ExaBGP line and writes one ze CLI command.
`neighbor <address> announce route <prefix> next-hop <nh>` becomes
`send bgp <address> update text nhop <nh> nlri ipv4/unicast add <prefix>`.
Twelve sites in `bridge_command.go` build a command of the shape
`send bgp <selector> update text ...`, where the selector is one address or `*`.

`TranslateLine` answers a `Translation`, not a string. It carries the command,
the selector that command addresses, and whether the command puts an UPDATE on a
wire. The caller flushes with that selector. It does not read the selector back
out of the command, and the next section says what that cost.

Up, `bridge_event.go` renders ze's BGP messages as ExaBGP JSON. The envelope
version is 6.0.0, which is the syntax target for the bridge.

The ExaBGP side is an external contract. An operator writes a script against
ExaBGP, so a change to ze's own CLI does not reach that script through the
translated forms.

<!-- source: internal/exabgp/bridge/bridge_command.go -- TranslateLine, Translation, convertRoute -->
<!-- source: internal/exabgp/bridge/bridge_event.go -- Version -->

## A line that names no neighbor goes to every peer

`TranslateLine` matches `neighbor <address> <rest>` with a regular
expression. A line that does not match names no destination, and ExaBGP sends
such a line to every neighbor. The bridge reads it the same way. It translates
the line with the wildcard peer selector, so `announce route <prefix>` becomes
`send bgp * update text nlri ipv4/unicast add <prefix>`.

`convertRoute` is the one place that decides what the bridge translates, and it
answers the same set of forms for a line that names a neighbor and for a line
that does not. The set is the ExaBGP vocabulary: `announce route`,
`withdraw route`, `announce ipv[46] <safi>` and `withdraw ipv[46] <safi>`, with
the SR-Policy shape of each.

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
| `create neighbor <ip> ...` | refused: ze creates no BGP peer at runtime |
| `neighbor <ip> receive update ...` | refused: this is an event, not a command |

The teardown carries a BGP cease subcode, and that subcode is what goes on the
wire. ze sends Cease, which is RFC 4271 error code 6, and it supplies the RFC
8203 shutdown communication itself when the command gives none. So the ExaBGP
grammar, which carries a subcode and nothing else, needs no message added to it.

`create neighbor` has no ze command behind it. `ze-bgp:peer-add` is declared in
`ze-bgp-api.yang`, no handler registers it, and `./le command list` shows no CLI
path that reaches it. A peer is created by an edit to the config tree and a
commit, which is not one line a script writes. The bridge refuses the line by
name. It does not map the line to `delete bgp peer` or to a teardown, because a
command that does something else would acknowledge the script for a session that
was never created.

`receive` is not a command. It is ExaBGP's own EVENT vocabulary, written by the
text response encoder, and it travels UP from the daemon to the script. The
`api-check` script reads such a line on its stdin. The refusal names the
direction, so that a reader opens the event encoder rather than the translator.

<!-- source: internal/exabgp/bridge/bridge_neighbor.go -- ConvertNeighborControl, convertTeardown -->

## The selector travels with the command

After a route command, the bridge injects a flush and blocks until the forward
pool drains. The peer it flushes is `Translation.Selector`, the selector the
translator used when it built the command.

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
        process one-shot {
            run "./run/api-no-respawn-1.run"
            respawn disable
        }
        process respawn {
            run "./run/api-no-respawn-2.run"
        }
    }
}
```

The block name is the ExaBGP process name, because that is the name a neighbor's
`api { processes [ ... ] }` list refers to. A block with no `run` command is
REFUSED and the refusal names it.

The bridge forks one child for each block. One ze event is translated once and
queued for EVERY child's stdin, and commands are read from EVERY child's stdout
and dispatched. The ack is per child: each script writes a line and blocks for
its own `done`, and `disable-ack` from one script silences that script alone.

Until 2026-09-06 the config carried one `run` leaf and the bridge forked one
child, so a config declaring two processes ran one of them and said nothing.
That is the silently-wrong answer `ai/rules/principles.md` exists to prevent,
and the three ported compatibility tests that name two processes
(`api-no-respawn`, `api-multiple-api`, `api-api`) are what measured it.

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

## Stage ordering constrains the family declaration

Config is not available at stage-1 registration. `familyDecls` therefore
declares the CLI-default family at stage 1, and the `family` leaf refines the
ADD-PATH capability encoding at stage 3, once the config has arrived.

## Config root

The settings nest under a top-level `exabgp` root, on an owner decision of
2026-07-09, so the root stays available for later ExaBGP-related
configuration. The registry plugin name stays `exabgp-bridge`.
