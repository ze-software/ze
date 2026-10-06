# `send bgp cached`

Send a cached UPDATE to the peers the selector matches\.

## Ze command

- Registry path: `send bgp cached`
- Usage: `send bgp <selector> cached <id>`
- Mode: Daemon
- Wire method: `ze-bgp:cache-forward`
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

The ID leaf accepts a comma\-separated list\, and the selector applies to each ID in that list\. The command sends a message the cache already holds\, so it names what goes on the wire and not the act of sending it\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `id` | string | yes | any value of this type | Cache entry ID\, or a comma\-separated list of IDs\. | One decimal cache entry ID\, as show cache lists it\, or several joined by commas\. A comma\-separated value runs the action once for each ID and reports a failure for each ID that is not in the cache\. A value that is not a decimal number is refused as an invalid cache id\. |

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
