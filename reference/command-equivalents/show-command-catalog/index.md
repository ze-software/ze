# `show command catalog`

List every command this binary carries with its summary\.

## Ze command

- Registry path: `show command catalog`
- Usage: `show command catalog [filter <filter>]`
- Mode: Read-only
- Wire method: `ze-help:show-command-catalog`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: map
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Walks the YANG command tree and the local command registry\, and prints one row per command with its summary\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `filter` | string | no | any value of this type | Text a command\'s path or description must hold\. | Keep only the commands whose path or description holds this text\. A bare first word is the filter too\. |

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
