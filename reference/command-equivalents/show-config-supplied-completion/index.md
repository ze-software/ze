# `show config supplied completion`

Complete literal editing input against supplied configuration\.

## Ze command

- Registry path: `show config supplied completion`
- Usage: `show config supplied completion content <content> [context <context>] [ghost]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-supplied-completion`
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

Complete editing input against supplied configuration with the native schema and completer\. Content is one JSON object with required configuration and input string fields\. Duplicate\, missing\, unknown and nonstring fields\, malformed UTF\-8 and trailing data are refused\. The combined envelope has the byte limit\. Input is literal\; plus characters are not changed to spaces\. No declared program is started to obtain a schema\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `content` | string | yes | any value of this type | Private configuration and editing input in one base64 JSON envelope\. | One padded base64 JSON envelope with configuration and input strings\, at most 195\,840 decoded bytes\. Both fields are masked in command records because editing input can itself contain a private value\. |
| `context` | string | no | any value of this type | Slash\-separated configuration context\. | Optional slash\-separated configuration path used by the native completer\, such as bgp\. Omission completes from the whole tree\. |
| `ghost` | flag | no | any value of this type | Return the native ghost suffix\. | Return the native ghost suffix followed by a newline instead of the completion document\. The suffix is the text that extends the current input\. |

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
