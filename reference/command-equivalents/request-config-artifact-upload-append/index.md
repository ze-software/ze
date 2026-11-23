# `request config artifact upload append`

Append bytes at the expected upload offset\.

## Ze command

- Registry path: `request config artifact upload append`
- Usage: `request config artifact upload append content <content> <offset> <transfer>`
- Mode: Daemon
- Wire method: `ze-config:artifact-upload-append`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
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

Append bytes at the expected upload offset\. The private owner checks the authenticated account\, cancellation and resource allowance\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Padded base64 bytes\, at most 195\,840 decoded bytes\. | Padded base64 content\, at most 195\,840 decoded bytes\. Content is masked from history and accounting\. Empty content is a valid empty append\. |
| `offset` | uint | yes | any value of this type | Offset\. | Offset for this private operation\; ownership comes only from authentication\. |
| `transfer` | string | yes | any value of this type | Transfer\. | Transfer for this private operation\; ownership comes only from authentication\. |

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
