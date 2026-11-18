# `show exabgp migration config`

Convert supplied ExaBGP configuration to Ze configuration\.

## Ze command

- Registry path: `show exabgp migration config`
- Usage: `show exabgp migration config <content> [dry-run] [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-exabgp:show-exabgp-migration-config`
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

Convert an ExaBGP configuration with the existing ExaBGP parser\, migration and serializer\. The default diagnostic name is supplied\.conf\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `dry-run` | flag | no | any value of this type | Preview conversion diagnostics without returning converted bytes\. | Run the conversion and retain its warnings and process information\, but answer no converted bytes\, as ze exabgp migrate \-\-dry\-run does\. |
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
