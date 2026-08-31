# `show system goroutines`

Dump goroutine stacks for debugging hangs or deadlocks.

## Ze command

- Registry path: `show system goroutines`
- Usage: `show system goroutines [mode <summary\|blocked\|full>]`
- Mode: Read-only
- Wire method: `ze-show:system-goroutines`
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

Modes: summary (groups by state), blocked (only lock/channel waiters), full (all stacks). Default: summary. Share the output with support when the daemon stops responding.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `mode` | enum | no | `summary`, `blocked`, `full` |

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
