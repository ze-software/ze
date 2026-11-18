# `request config artifact bytes begin`

Begin a private binary source transfer\.

## Ze command

- Registry path: `request config artifact bytes begin`
- Usage: `request config artifact bytes begin <name> [total <total>]`
- Mode: Daemon
- Wire method: `ze-config:artifact-bytes-begin`
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

Begin a bounded binary source transfer\. Unlike a workspace upload\, finishing retains arbitrary bytes for raw\-key writes or native check and repair\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Private source display label\. | Display label without a directory or control characters\. |
| `total` | uint | no | any value of this type | Optional exact binary length\. | Optional exact byte count\, checked before completion\. Omission permits bounded unknown\-length streaming\. |

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
