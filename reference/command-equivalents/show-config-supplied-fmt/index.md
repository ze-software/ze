# `show config supplied fmt`

Format supplied configuration without writing a file\.

## Ze command

- Registry path: `show config supplied fmt`
- Usage: `show config supplied fmt <content> [check] [diff] [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-supplied-fmt`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: blob
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Format supplied configuration through the existing formatter\. The answer is formatted configuration text\, or the selected check or diff diagnostic\. This command has no file\-write option\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `check` | flag | no | any value of this type | Report whether formatting is needed\. | Report whether formatting would change the supplied bytes\, with the same verdict as show config fmt check\. |
| `diff` | flag | no | any value of this type | Show what formatting would change\. | Show the formatting changes as the same diagnostic hunks as show config fmt diff\, using the supplied name as a label\. |
| `content` | string | yes | any value of this type | Input bytes in padded base64\, at most 195\,840 bytes\. | Caller\-held bytes encoded as padded base64\. The whole input is bounded to 195\,840 decoded bytes and never recorded in command history or accounting\. |
| `name` | string | no | any value of this type | Filename label for diagnostics\; no file is opened\. | An optional filename label for diagnostics\, without a directory or control characters\. The default is supplied\.conf\. No file is opened or written\. |

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
