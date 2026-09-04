# ExaBGP Bridge Plugin

The bridge runs an ExaBGP-API script against ze's BGP engine. It has two
runners with one behavior: an external SDK-mode process, and an internal
in-process runner.

<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- runInternalBridge, familyDecls -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- exabgp bridge config parsing -->

## The bridge translates in two directions

An ExaBGP script writes text commands down to ze, and ze sends JSON events up
to the script. Each direction has its own translator.

Down, `ExabgpToZebgpCommand` reads one ExaBGP line and writes one ze CLI
command. `neighbor <address> announce route <prefix> next-hop <nh>` becomes
`peer <address> update text nhop <nh> nlri ipv4/unicast add <prefix>`. Fourteen
sites in `bridge_command.go` build a command of the shape
`peer <address> <verb> ...`.

Up, `bridge_event.go` renders ze's BGP messages as ExaBGP JSON. The envelope
version is 6.0.0, which is the syntax target for the bridge.

The ExaBGP side is an external contract. An operator writes a script against
ExaBGP, so a change to ze's own CLI does not reach that script through the
translated forms.

<!-- source: internal/exabgp/bridge/bridge_command.go -- ExabgpToZebgpCommand -->
<!-- source: internal/exabgp/bridge/bridge_event.go -- Version -->

## A line the translator does not recognize passes through unchanged

`ExabgpToZebgpCommand` matches `neighbor <address> <rest>` with a regular
expression. A line that does not match is returned unchanged, so it reaches
ze's CLI verbatim.

Passthrough is what makes ze's CLI reachable from a script. It does not make
the two command sets agree, and they do not agree.

ze declares a bare form for each `announce` and `withdraw` command, and the
bare form reaches every peer. The spellings are ze's own. ExaBGP writes
`announce route <prefix>` and `withdraw route <prefix>`, where ze declares
`announce unicast`, `announce blackhole`, `announce flowspec`, `withdraw tag`,
`withdraw id` and `withdraw all`.

So a bare ExaBGP line reaches ze's dispatcher and matches no command. `help`
is the only one of the nine exempt wire methods an ExaBGP script can reach
through passthrough. It is the only one spelled the same on both sides.

| Wire method | How a script reaches it | What a rename costs |
|---|---|---|
| `announce-unicast`, `announce-blackhole`, `announce-flowspec`, `withdraw-tag`, `withdraw-id`, `withdraw-all` | no ExaBGP script reaches these, because ExaBGP spells them `route` | nothing for a script, and an operator types them at ze's own CLI |
| `help` | passthrough, with one spelling on both sides | a break for a script that asks for help |
| `peer-update` | translator output, from `neighbor <address> announce` | one edit in the translator |
| `peer-raw` | neither, because the translator never writes it | one edit, and no script is affected |

Each is exempt from the verb-first grammar for one of two reasons. It starts
with a noun, or it is a line-protocol verb ze does not list as one of its own.

<!-- source: internal/component/command/grammar/checker.go -- bridgeSurface, ExemptCategory -->

## The peer address is recovered by re-parsing the translated command

After a route command, the bridge injects a flush and blocks until the forward
pool drains. It finds the peer to flush with `ExtractPeerAddress`, which reads
the address out of the command string the translator just built.

`ExtractPeerAddress` requires the literal prefix `peer ` and answers the empty
string for any other command. Its three callers read that empty string as
"nothing to flush" and continue. `IsRouteCommand` looks for `update text`
anywhere in the string, so it does not agree with that prefix test.

A change to the leading token therefore stops every flush. There is no error
and no log line. The translator holds the peer address at each of the fourteen
sites where it writes the command. It can pass that address on, and it does not
have to read the address back.

<!-- source: internal/exabgp/bridge/bridge_muxconn.go -- ExtractPeerAddress, IsRouteCommand -->

## The internal runner cannot read the `run` line

The external runner takes the script command from the process-manager `run`
line. A `RunEngine(conn net.Conn)` runner never sees that line, so the internal
runner reads the script command from the `exabgp { bridge { ... } }` config
root, delivered by the SDK `OnConfigure` callback at stage 2. This is the one
structural difference between the two runners; everything after it is shared.

## Stage ordering constrains the family declaration

Config is not available at stage-1 registration. `familyDecls` therefore
declares the CLI-default family at stage 1, and the `family` leaf refines the
ADD-PATH capability encoding at stage 3, once the config has arrived.

## Config root

The settings nest under a top-level `exabgp` root, on an owner decision of
2026-07-09, so the root stays available for later ExaBGP-related
configuration. The registry plugin name stays `exabgp-bridge`.
