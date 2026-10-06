# `request peer teardown`

Tear down a peer session\.

## Ze command

- Registry path: `request peer teardown`
- Usage: `request peer <selector> teardown <cease-subcode>`
- Mode: Daemon
- Wire method: `ze-bgp:peer-teardown`
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

The subcode is the NOTIFICATION subcode for error code 6 and it takes 0 to 255\. Ze sends an RFC 8203 shutdown communication message with subcode 2\, administrative shutdown\, and with subcode 4\, administrative reset\, and with no other subcode\. The message follows the subcode on the command line\, and the answer echoes the truncated text that went on the wire\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'pause\' and \'resume\' refuse a selector that matches more than one peer\, and the other commands act on each peer it matches\. |
| `cease-subcode` | uint | yes | any value of this type | BGP cease NOTIFICATION subcode | The value is a whole number from 0 to 255\, and a value outside that range is refused\. Ze puts it in the Cease NOTIFICATION it sends before it closes the session\. |

## Mapping intents

### Hard reset one BGP peer

Category: BGP

## Vendor equivalents

### Junos MX
- `clear bgp neighbor <peer>` (verified, junos-clear-bgp)
  - Intent: Hard reset one BGP peer

### IOS XR
- `clear bgp ipv4 unicast <peer>` (verified, iosxr-bgp-commands)
  - Intent: Hard reset one BGP peer

### SR OS

No equivalent listed.

### VyOS
- `reset bgp <peer>` (verified, vyos-cli)
  - Intent: Hard reset one BGP peer
