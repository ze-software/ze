# `show env get`

Show one environment variable in detail\.

## Ze command

- Registry path: `show env get`
- Usage: `show env get <name>`
- Mode: Read-only
- Wire method: `ze-env:show-get`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: key, type, default, current, description
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns the variable name\, current value\, default\, and what it controls\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Environment variable name | The variable\'s full name as Ze registered it\, matched exactly\. A name Ze never declared is refused\, even when the process inherited it\. |

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
