# `show data cat`

Print the value of a storage key\.

## Ze command

- Registry path: `show data cat`
- Usage: `show data cat <key>`
- Mode: read-only, offline
- Wire method: `ze-config-storage:show-data-cat`
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

Outputs the decoded value for the given storage key\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `key` | string | yes | any value of this type | Storage key to print | The key of one entry\, as show data list prints it\. The bytes stored under it are written out unchanged\, and a key the store does not hold is an error\. |

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
