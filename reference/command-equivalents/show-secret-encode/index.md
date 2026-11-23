# `show secret encode`

Encode a supplied private value in the reversible \$9\$ form\.

## Ze command

- Registry path: `show secret encode`
- Usage: `show secret encode content <content> [trim-line-ending]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-secret-encode`
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

Encode the literal supplied operand in the reversible \$9\$ form\. Empty values and values already carrying the \$9\$ prefix are refused\. The result is returned to the requesting caller with the standalone final newline\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Private operand in padded base64\, at most 195\,840 decoded bytes\. | Private operand encoded as padded base64\, at most 195\,840 decoded bytes\. The bytes are masked in history and accounting\. No host file or key is read\. |
| `trim-line-ending` | flag | no | any value of this type | Apply the standalone stdin trailing line\-ending rule\. | Remove at most one trailing LF and then one CR\, as the standalone secret command does for stdin\. Without this flag every input byte is preserved\. |

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
