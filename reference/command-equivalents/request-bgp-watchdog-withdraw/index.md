# `request bgp watchdog withdraw`

Withdraw the routes of a watchdog\.

## Ze command

- Registry path: `request bgp watchdog withdraw`
- Usage: `request bgp watchdog withdraw <name> [peer <peer>]`
- Mode: Daemon
- Wire method: `ze-bgp:watchdog-withdraw`
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

Ze withdraws every route of the watchdog from each peer that carries it\, and holds them withdrawn across a reconnect until the next announce\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Watchdog name | The watchdog\'s name\. med is refused as a name\. |
| `peer` | string | no | any value of this type | Peer address\, or \* for every peer | The peer the routes are withdrawn from\, or \* for every peer that carries the watchdog\. Absent\, the peer the command was sent for\. |

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
