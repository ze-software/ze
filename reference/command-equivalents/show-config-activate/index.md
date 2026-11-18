# `show config activate`

Return a detached file transformed by one activate command\.

## Ze command

- Registry path: `show config activate`
- Usage: `show config activate <content> <file> [dry-run]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-activate`
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

Read a confined source file and apply one base64\-encoded activate command\. The transformed text is returned without changing the source or live candidate\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Input bytes in padded base64\, at most 195\,840 bytes\. | Caller\-held bytes encoded as padded base64\. The whole input is bounded to 195\,840 decoded bytes and never recorded in command history or accounting\. |
| `dry-run` | flag | no | any value of this type | Return only the masked change preview\. | Return the masked difference without serializing a changed configuration\. Every form remains detached and never publishes into a file or live candidate\. |
| `file` | string | yes | any value of this type | Configuration filename on the answering host\. | A configuration filename on the answering host\. The local route also accepts \- for stdin\. Daemon routes confine the path to configured directories\. |

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
