# `show config graph`

Show how components and peers depend on each other\, as JSON\.

## Ze command

- Registry path: `show config graph`
- Usage: `show config graph <file>`
- Mode: Read-only
- Wire method: `ze-config-cli:show-config-graph`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Prints the dependency graph of a config file as JSON\. Inactive blocks are left out\, so the graph is the one the daemon would build\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `file` | string | yes | any value of this type | Config file path\, or \- for stdin\. | The config file to read\, or \- to read it on stdin\. |

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
