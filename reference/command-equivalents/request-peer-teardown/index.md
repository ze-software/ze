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

The subcode is the NOTIFICATION subcode for error code 6 and it takes 0 to 255\. Ze sends an RFC 9003 Shutdown Communication with subcode 2\, Administrative Shutdown\, and with subcode 4\, Administrative Reset\, and with no other subcode\. The message follows the subcode on the command line\, and the answer echoes the truncated text that went on the wire\. With no message\, Ze sends the subcode\'s name\, such as Administrative Shutdown\, as the communication\. The selector names one peer\: \*\, an exclusion and a selector matching several peers are refused\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | An address or peer name\; \* or a pattern for refresh\, borr\, eorr and flush | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'refresh\'\, \'borr\'\, \'eorr\' and \'flush\' act on every peer it matches\. \'teardown\'\, \'notify\'\, \'disable\'\, \'enable\'\, \'pause\'\, \'resume\' and \'clear soft\' act on one session\: they refuse \*\, an exclusion such as \!edge1\, and a selector that matches more than one peer\, and they accept a pattern or glob that matches exactly one peer\. |
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
