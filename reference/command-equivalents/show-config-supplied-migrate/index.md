# `show config supplied migrate`

Convert supplied configuration between its two written forms\.

## Ze command

- Registry path: `show config supplied migrate`
- Usage: `show config supplied migrate <content> [format <set|hierarchical>] [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-supplied-migrate`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
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

Convert current configuration bytes between set commands and nested blocks through the local converter\. No file or candidate is written\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Input bytes in padded base64\, at most 195\,840 bytes\. | Caller\-held bytes encoded as padded base64\. The whole input is bounded to 195\,840 decoded bytes and never recorded in command history or accounting\. |
| `format` | enum | no | `set`, `hierarchical` | Output form\: set or hierarchical\. | The output form\. Both forms preserve values and hash plaintext passwords through the same producer used by ze config migrate\. |
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
