# `peer withdraw tag`

Withdraw the announcements carrying a tag.

## Ze command

- Registry path: `peer withdraw tag`
- Usage: `peer <selector> withdraw tag <key> [value <value>]`
- Mode: Daemon
- Wire method: `ze-bgp:withdraw-tag`
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

A key of * withdraws every tagged announcement. An absent value withdraws every value of the key.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
| `key` | string | yes | any value of this type |
| `value` | string | no | any value of this type |

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
