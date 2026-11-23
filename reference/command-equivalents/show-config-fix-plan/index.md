# `show config fix-plan`

Plan configuration repairs without applying them\.

## Ze command

- Registry path: `show config fix-plan`
- Usage: `show config fix-plan <file>`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-fix-plan`
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

Read a confined configuration file and return its validation diagnostics and repair candidates without applying a repair\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
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
