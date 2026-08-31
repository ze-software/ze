# `show system file-descriptors`

Show how many file descriptors the daemon has open.

## Ze command

- Registry path: `show system file-descriptors`
- Usage: `show system file-descriptors [mode <summary\|detail>]`
- Mode: Read-only
- Wire method: `ze-show:system-file-descriptors`
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

Summary mode: totals by type (socket, pipe, file). Detail mode: every fd with its path and type. Linux only (reads /proc/self/fd). Check this when you suspect fd exhaustion.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `mode` | enum | no | `summary`, `detail` |

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
