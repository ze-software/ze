# `validate config`

Check a config for errors without applying it\.

## Ze command

- Registry path: `validate config`
- Usage: `validate config <file>`
- Mode: Read-only
- Wire method: `ze-config-cli:validate-config`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Both the grammar of the file and the meaning of its values are checked\, and each problem is reported with the diagnostic code that explains it\. Nothing is applied\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `file` | string | yes | any value of this type | Config file path\, or \- for stdin\. | The config file to check\, or \- to read it on stdin\. |

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
