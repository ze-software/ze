# `request log level`

Change a subsystem's log level without restarting.

## Ze command

- Registry path: `request log level`
- Usage: `request log level <logger> <disabled\|debug\|info\|warn\|err>`
- Mode: Daemon
- Wire method: `ze-bgp:log-set`
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

Takes effect immediately. Set to debug when troubleshooting, then back to info when you are done.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `logger` | string | yes | any value of this type |
| `target` | enum | yes | `disabled`, `debug`, `info`, `warn`, `err` |

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
