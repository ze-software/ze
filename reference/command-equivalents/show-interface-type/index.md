# `show interface type`

Show only interfaces of a given type.

## Ze command

- Registry path: `show interface type`
- Usage: `show interface type <type>`
- Mode: Read-only
- Wire method: `ze-show:interface-type`
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

Types include ethernet, bridge, vxlan, wireguard, tunnel, bond, and more. If you pick an invalid type, the error lists all valid ones.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `type` | string | yes | any value of this type |

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
