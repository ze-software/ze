# `clear bgp rib in`

Remove all routes received from a peer\.

## Ze command

- Registry path: `clear bgp rib in`
- Usage: `clear bgp rib in <selector>`
- Mode: Daemon
- Wire method: `ze-bgp:rib-clear-in`
- Backends: any backend
- Task support: forbidden: the MCP server never answers with a task handle
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

Wipes the Adj\-RIB\-In for the peers the selector matches\. They will need to re\-advertise everything \(or you can send a route\-refresh\)\. The selector is mandatory\: clear bgp rib in alone is refused\, and \* is the explicit form for every peer\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | \* for every peer\, or an address\, a peer name or an AS pattern | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The Adj\-RIB\-In of each peer it matches is wiped\. |

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
