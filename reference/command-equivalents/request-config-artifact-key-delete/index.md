# `request config artifact key delete`

Remove one raw artifact key\.

## Ze command

- Registry path: `request config artifact key delete`
- Usage: `request config artifact <artifact> key <key> delete`
- Mode: Daemon
- Wire method: `ze-config-cli:artifact-key-delete`
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

Remove exactly the selected raw key through the native store operation\, preserving every unrelated key\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `artifact` | string | yes | any value of this type | Artifact\. | Artifact for this private operation\; ownership comes only from authentication\. |
| `key` | string | yes | any value of this type | Explicit raw storage key\. | One explicit raw storage key\. The native store validates its complete key grammar before reading or writing it\. |

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
