# `request config artifact repair`

Salvage a source into a new private artifact\.

## Ze command

- Registry path: `request config artifact repair`
- Usage: `request config artifact repair <transfer>`
- Mode: Daemon
- Wire method: `ze-config:artifact-repair`
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

Salvage a completed binary source into a new independent private artifact\. The original source stays retained\; the native frame and history repair grade determines success\, warning or error with complete finding rows\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
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
