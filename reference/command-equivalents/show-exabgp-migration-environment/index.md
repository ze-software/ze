# `show exabgp migration environment`

Convert supplied ExaBGP INI environment to Ze configuration\.

## Ze command

- Registry path: `show exabgp migration environment`
- Usage: `show exabgp migration environment <content> [dry-run] [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-exabgp:show-exabgp-migration-environment`
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

Parse\, validate and map an ExaBGP INI environment file to Ze configuration\. Values are returned\, never installed in the daemon environment\. The default diagnostic name is supplied\.env\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `dry-run` | flag | no | any value of this type | Report the unsupported dry\-run choice and return the conversion\. | Retain the standalone environment conversion behavior\: report that dry\-run is not supported for environment conversion and still return the converted bytes\. |
| `content` | string | yes | any value of this type | ExaBGP input bytes in padded base64\, at most 195\,840 bytes\. | ExaBGP source bytes in padded base64\, bounded to 195\,840 decoded bytes\. Content is withheld from command history and accounting\. No file is opened or written\. |
| `name` | string | no | any value of this type | Filename label for diagnostics\; no file is opened\. | Optional filename label without a directory or control characters\. Used only in diagnostics\; it never selects a daemon file\. |

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
