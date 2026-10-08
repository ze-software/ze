# `show log recent`

Show recent log entries from the in\-memory ring\.

## Ze command

- Registry path: `show log recent`
- Usage: `show log recent [level <disabled\|debug\|info\|warn\|err>] [component <component>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-log:bgp-log-recent`
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

Filters \(all optional\)\: level \<lvl\>\, component \<name\>\, count \<N\>\. Newest entries first\. Useful when you cannot access the log file directly\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `level` | enum | no | `disabled`, `debug`, `info`, `warn`, `err` | Filter by log level | Keeps the entries whose level equals this value\, and no other level\. Without it every level is shown\. |
| `component` | string | no | any value of this type | Filter by component name | Keeps the entries whose component name equals this value exactly\. Without it every component is shown\. |
| `count` | uint | no | any value of this type | Maximum number of entries | Stops the answer after this many entries\, newest first\. It MUST be 1 or more\, and without it every buffered entry is shown\. |

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
