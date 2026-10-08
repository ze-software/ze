# `update bgp peer prefix`

Refresh max\-prefix limits from PeeringDB\.

## Ze command

- Registry path: `update bgp peer prefix`
- Usage: `update bgp peer <selector> prefix`
- Mode: Daemon
- Wire method: `ze-bgp:update-peer-prefix`
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

Queries PeeringDB for each matched peer\'s ASN\, applies the configured margin\, and writes the result to the config draft\. Run \'config commit\' to apply\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'prefix\' refreshes the maximums of each peer it matches\. |

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
