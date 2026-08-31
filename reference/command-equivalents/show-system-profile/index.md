# `show system profile`

Capture a runtime profile for performance analysis.

## Ze command

- Registry path: `show system profile`
- Usage: `show system profile [type <cpu\|heap\|goroutine\|allocs>] [duration <duration>]`
- Mode: Read-only
- Wire method: `ze-show:system-profile`
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

Types: cpu (requires duration, e.g. 30s), heap, goroutine, allocs (instant snapshots). Output is pprof format you can open with 'go tool pprof'. Send the file to support for deep analysis.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `type` | enum | no | `cpu`, `heap`, `goroutine`, `allocs` |
| `duration` | string | no | any value of this type |

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
