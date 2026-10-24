# `delete bgp peer`

Remove a peer from the running config\.

## Ze command

- Registry path: `delete bgp peer`
- Usage: `delete bgp peer <selector>`
- Mode: Daemon
- Wire method: `ze-bgp:peer-delete`
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

Tears down the TCP session and deletes the peer from the running configuration\. Does not modify the config file on disk\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | One peer\: its address or peer name | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\. It names one peer\: \*\, an exclusion such as \!edge1\, and a selector that matches more than one peer are refused\, and a pattern or glob that matches exactly one peer is accepted\. Ze tears down and deletes that peer\. |

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
