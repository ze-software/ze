# `clear bgp rib out-excluding-owner`

Replay while excluding one original destination owner\.

## Ze command

- Registry path: `clear bgp rib out-excluding-owner`
- Usage: `clear bgp rib out-excluding-owner owner <owner> [family <family>]`
- Mode: Daemon
- Wire method: `ze-bgp:rib-clear-out-excluding-owner`
- Backends: any backend
- Task support: forbidden\: the MCP server never answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: map
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Re\-advertise to current destinations except one captured original peer owner\. This narrows replay and grants no send permission\. The ordinary out command and its literal peer selectors keep their existing grammar\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `owner` | uint | yes | any value of this type | Original destination owner to exclude | The configured Peer owner whose original destination is left out of the replay\, typed after its keyword as owner 12\. It narrows the replay and grants no send permission\. |
| `family` | string | no | any value of this type | Address family to replay\, such as ipv4\/unicast | An address family such as ipv4\/unicast\, typed after its keyword\. Absent\, every family the session negotiated is replayed\. |

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
