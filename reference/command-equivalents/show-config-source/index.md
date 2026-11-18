# `show config source`

Display a file\'s masked source tree at an optional path\.

## Ze command

- Registry path: `show config source`
- Usage: `show config source <file> [path <path>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-source`
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

Read a confined configuration file and display its unexpanded source tree with parsed secret leaves masked\, optionally at a selected path\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `file` | string | yes | any value of this type | Configuration filename on the answering host\. | A configuration filename on the answering host\. The local route also accepts \- for stdin\. Daemon routes confine the path to configured directories\. |
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
