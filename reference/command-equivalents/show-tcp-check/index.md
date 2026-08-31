# `show tcp-check`

Test TCP connectivity to a remote host and port.

## Ze command

- Registry path: `show tcp-check`
- Usage: `show tcp-check <host> <port> [source <source>] [timeout <timeout>]`
- Mode: Read-only
- Wire method: `ze-show:tcp-check`
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

Tries to open a TCP connection and reports success or failure with the connection time. Use 'source <IP>' to bind a specific local address. Quick way to verify a peer's BGP port is reachable.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `host` | string | yes | any value of this type |
| `port` | uint | yes | any value of this type |
| `source` | string | no | any value of this type |
| `timeout` | string | no | any value of this type |

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
