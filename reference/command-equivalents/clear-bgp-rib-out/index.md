# `clear bgp rib out`

Re\-advertise all routes to a peer\.

## Ze command

- Registry path: `clear bgp rib out`
- Usage: `clear bgp rib out <selector> [family <family>]`
- Mode: Daemon
- Wire method: `ze-bgp:rib-clear-out`
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

Triggers a full Adj\-RIB\-Out replay to the selected peers\. Useful after a policy change to push updated attributes without tearing down the session\. The selector is mandatory\: clear bgp rib out alone is refused\, and \* is the explicit form for every peer\. A family typed after its keyword\, as family ipv4\/unicast\, limits the replay to that family\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | \* for every peer\, or an address\, a peer name or an AS pattern | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The Adj\-RIB\-Out of each peer it matches is replayed\. |
| `family` | string | no | any value of this type | Address family to replay\, such as ipv4\/unicast | An address family such as ipv4\/unicast\. Absent\, every family the session negotiated is replayed\. |

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
