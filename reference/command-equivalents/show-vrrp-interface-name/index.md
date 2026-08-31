# `show vrrp interface name`

Show the VRRP virtual routers on one parent interface.

## Ze command

- Registry path: `show vrrp interface name`
- Usage: `show vrrp interface name [value <value>]`
- Mode: Read-only
- Wire method: `ze-show:vrrp-interface`
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

Pass the interface name: show vrrp interface name <interface>.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
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
