# `request peer teardown`

Tear down a peer session.

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
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
| `cease-subcode` | uint | yes | any value of this type |

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
