# `show command reference`

Print the agent reference this binary builds from its own registries\.

## Ze command

- Registry path: `show command reference`
- Usage: `show command reference [cli] [api] [mcp] [dispatch] [all]`
- Mode: Read-only
- Wire method: `ze-help:show-command-reference`
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

The reference is generated from the registries of this binary\, so it always matches the running version\. With no section it prints a summary with counts\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `cli` | flag | no | any value of this type | CLI subcommands\. | Print the cli section\: CLI subcommands\. Sections combine\. |
| `api` | flag | no | any value of this type | Daemon API commands with parameters\, update syntax\, families and plugins\. | Print the api section\: Daemon API commands with parameters\, update syntax\, families and plugins\. Sections combine\. |
| `mcp` | flag | no | any value of this type | MCP tools with parameters and examples\. | Print the mcp section\: MCP tools with parameters and examples\. Sections combine\. |
| `dispatch` | flag | no | any value of this type | Dispatch keys for daemon commands\. | Print the dispatch section\: Dispatch keys for daemon commands\. Sections combine\. |
| `all` | flag | no | any value of this type | Every section combined\. | Print the all section\: Every section combined\. Sections combine\. |

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
