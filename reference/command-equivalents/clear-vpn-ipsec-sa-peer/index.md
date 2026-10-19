# `clear vpn ipsec sa peer`

Tear down one peer\'s IKE Security Associations\.

## Ze command

- Registry path: `clear vpn ipsec sa peer`
- Usage: `clear vpn ipsec sa peer <name>`
- Mode: Daemon
- Wire method: `ze-ike:clear-vpn-ipsec-sa-peer`
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

Only the named peer\'s SAs are terminated\. A name that matches no configured peer is refused\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Peer name\. | The peer name as configured under the IPsec connection\. |

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
