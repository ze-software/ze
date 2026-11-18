# `show config migrate`

Convert a configuration file between its two written forms\.

## Ze command

- Registry path: `show config migrate`
- Usage: `show config migrate <file> [format <set|hierarchical>]`
- Mode: read\-only\, offline
- Wire method: `ze-config-cli:show-config-migrate`
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

Read a confined configuration file and convert its written form\. The result is returned as text\; the source file is never rewritten\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `file` | string | yes | any value of this type | Configuration filename on the answering host\. | A configuration filename on the answering host\. The local route also accepts \- for stdin\. Daemon routes confine the path to configured directories\. |
| `format` | enum | no | `set`, `hierarchical` | Output form\: set or hierarchical\. | The output form\. Both forms preserve values and hash plaintext passwords through the same producer used by ze config migrate\. |

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
