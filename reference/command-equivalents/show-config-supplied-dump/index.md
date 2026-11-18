# `show config supplied dump`

Show the resolved tree from supplied configuration bytes\.

## Ze command

- Registry path: `show config supplied dump`
- Usage: `show config supplied dump <content> [strip-private] [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-supplied-dump`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Parse and resolve supplied configuration bytes with the same producer as show config dump\. Sensitive values retain its encoded or stripped display\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `strip-private` | flag | no | any value of this type | Replace sensitive values with a marker\. | Replace sensitive values with the same placeholder as the file\-based dump\, so the resolved tree can be shared without its secrets\. |
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
