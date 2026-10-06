# `send bgp withdraw id`

Withdraw one announcement by the id that show announcements reports\.

## Ze command

- Registry path: `send bgp withdraw id`
- Usage: `send bgp <selector> withdraw id <id>`
- Mode: Daemon
- Wire method: `ze-bgp:withdraw-id`
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

Only a tagged announcement carries an id\. The withdraw acts only when the selector typed after bgp is the one the announcement was made with\, compared as text\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `id` | string | yes | any value of this type | Announcement id\, as show announcements reports it | The value is the whole number show announcements reports in its id column\. A value that is not a number is refused\, and an id that names no announcement of the selector answers \'not found\'\. |

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
