# `show system sockets`

Show open TCP and UDP sockets on this box.

## Ze command

- Registry path: `show system sockets`
- Usage: `show system sockets [protocol <tcp\|udp>] [state <state>] [port <port>]`
- Mode: Read-only
- Wire method: `ze-show:system-sockets`
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

Every filter is optional and they combine. States use kernel names (ESTABLISHED, LISTEN, TIME_WAIT). Linux only. Good for confirming listeners or spotting stuck connections.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `protocol` | enum | no | `tcp`, `udp` |
| `state` | string | no | any value of this type |
| `port` | uint | no | any value of this type |

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
