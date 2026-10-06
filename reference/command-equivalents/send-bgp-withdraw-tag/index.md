# `send bgp withdraw tag`

Withdraw the announcements carrying a tag\.

## Ze command

- Registry path: `send bgp withdraw tag`
- Usage: `send bgp <selector> withdraw tag <key> [value <value>]`
- Mode: Daemon
- Wire method: `ze-bgp:withdraw-tag`
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

A key of \* withdraws every tagged announcement\. An absent value withdraws every value of the key\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `key` | string | yes | any value of this type | Tag key\, or \* for every tagged announcement | The value is the key an announcement was made with\, compared as text\. Ze withdraws each announcement made to the selector that carries it\. A key of \* withdraws every tagged announcement made to the selector\. |
| `value` | string | no | any value of this type | Tag value\, or every value of the key when absent | The value is the tag value an announcement was made with\, compared as text\. With it\, Ze withdraws the announcements under the key that carry this value alone\. Absent\, or \*\, every value of the key is withdrawn\. |

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
