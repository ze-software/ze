# `show crashes`

View saved crash reports, and whether kernel capture is armed.

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

Without arguments, lists every stored report with its kind: 'panic' for a Go panic this daemon caught, 'kernel' for a kernel fault recovered from the reserved region on the following boot. The listing also carries a readiness block, which reports configured and armed separately because the reservation is a kernel boot argument and a commit alone does not arm it. Use 'latest' to see the newest report or 'name <filename>' to print one. Send the output to support when reporting a crash.

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
