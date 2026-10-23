# `clear bgp rib advertised`

Withdraw every route advertised to a peer\.

## Ze command

- Registry path: `clear bgp rib advertised`
- Usage: `clear bgp rib advertised <selector> [family <family>]`
- Mode: Daemon
- Wire method: `ze-bgp:rib-clear-advertised`
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

Withdraws every route the selected peers\' sessions hold in their Adj\-RIB\-Out\, configured routes included\, and the peers are left with nothing from Ze until routes are announced again\. clear bgp rib out resends the same table instead\. The selector is mandatory\: clear bgp rib advertised alone is refused\, and \* is the explicit form for every peer\. A family typed after its keyword\, as family ipv4\/unicast\, limits the withdrawal to that family\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | \* for every peer\, or an address\, a peer name or an AS pattern | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. A selector that matches no peer is refused\. |
| `family` | string | no | any value of this type | Address family to withdraw\, such as ipv4\/unicast | An address family such as ipv4\/unicast\. Absent\, every family is withdrawn\. |

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
