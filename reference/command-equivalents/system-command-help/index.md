# `system command help`

Show the detailed help for one command\.

## Ze command

- Registry path: `system command help`
- Usage: `system command help <name>`
- Mode: Read-only
- Wire method: `ze-plugin:system-command-help`
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

The answer carries command\, short\-help\, description and source\. A command a plugin registered also carries args and its timeout\. An unknown name fails with \'unknown command\: \<name\>\'\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Command name | The full command name\, as command list prints it\. A name in neither registry fails with unknown command\. |

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
