# `show log recent`

Show recent log entries from the in-memory ring.

## Ze command

- Registry path: `show log recent`
- Usage: `show log recent [level <disabled\|debug\|info\|warn\|err>] [component <component>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-bgp:log-recent`
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

Filters (all optional): level <lvl>, component <name>, count <N>. Newest entries first. Useful when you cannot access the log file directly.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `level` | enum | no | `disabled`, `debug`, `info`, `warn`, `err` |
| `component` | string | no | any value of this type |
| `count` | uint | no | any value of this type |

## Mapping intents

### Logs, warnings, and errors

Category: Operations

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show log` (verified, vyos-cli)
  - Intent: Logs, warnings, and errors
