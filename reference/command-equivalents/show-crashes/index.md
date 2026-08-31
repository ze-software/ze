# `show crashes`

View saved crash reports from panics.

## Ze command

- Registry path: `show crashes`
- Usage: `show crashes [name <name>]`
- Mode: Read-only
- Wire method: `ze-show:crashes`
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

Without arguments, lists available crash files. Use 'latest' to see the newest crash or 'name <filename>' to print one specific report. Send the output to support when reporting a crash.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | no | any value of this type |

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
