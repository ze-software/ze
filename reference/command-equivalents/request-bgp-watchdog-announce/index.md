# `request bgp watchdog announce`

Announce the routes of a watchdog\.

## Ze command

- Registry path: `request bgp watchdog announce`
- Usage: `request bgp watchdog announce <name> [med <med>] [peer <peer>]`
- Mode: Daemon
- Wire method: `ze-bgp:watchdog-announce`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Ze announces every route of the watchdog to each peer that carries it\, and the routes stay announced across a reconnect\. The peer is every peer\, \*\, or the one address typed after the name\. A watchdog whose routes a process gave refuses med\, because Ze cannot change the MED of an update text it did not write\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Watchdog name | The watchdog\'s name\. med is refused as a name\, because it is the keyword of the MED override\. |
| `med` | uint | no | any value of this type | MED override | The MED the configured routes are announced with\, in place of their own\. Absent\, each route keeps its configured MED\. |
| `peer` | string | no | any value of this type | Peer address\, or \* for every peer | The peer the routes are announced to\, or \* for every peer that carries the watchdog\. Absent\, the peer the command was sent for\. |

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
