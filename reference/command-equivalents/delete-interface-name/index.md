# `delete interface name`

Delete an interface from the kernel.

## Ze command

- Registry path: `delete interface name`
- Usage: `delete interface name <name>`
- Mode: Daemon
- Wire method: `ze-iface:interface-delete`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `address`, `unit`
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
| `name` | string | yes | any value of this type |

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
