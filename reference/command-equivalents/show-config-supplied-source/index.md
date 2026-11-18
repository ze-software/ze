# `show config supplied source`

Read the masked source tree at an optional path\.

## Ze command

- Registry path: `show config supplied source`
- Usage: `show config supplied source <content> [name <name>] [path <path>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-supplied-source`
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

Display a source tree before inheritance resolution\, with parsed secret leaves masked\. An unparseable whole source is returned for repair\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Input bytes in padded base64\, at most 195\,840 bytes\. | Caller\-held bytes encoded as padded base64\. The whole input is bounded to 195\,840 decoded bytes and never recorded in command history or accounting\. |
| `name` | string | no | any value of this type | Filename label for diagnostics\; no file is opened\. | An optional filename label for diagnostics\, without a directory or control characters\. The default is supplied\.conf\. No file is opened or written\. |
| `path` | string | no | any value of this type | Configuration path\, such as bgp peer edge\. | A quoted sequence of configuration path words\, including quoted list keys where needed\. It selects a subtree and never names a file\. |

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
