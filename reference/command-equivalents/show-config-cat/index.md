# `show config cat`

Print the full text of a stored configuration snapshot\.

## Ze command

- Registry path: `show config cat`
- Usage: `show config cat <id>`
- Mode: read-only, offline
- Wire method: `ze-config-cli:show-config-cat`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: blob
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Outputs the config as\-is\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `id` | string | yes | any value of this type | Stored configuration snapshot id\. | The value is a key that show config list prints\. Ze reads that file from the configuration store and writes its bytes unchanged\. |

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
