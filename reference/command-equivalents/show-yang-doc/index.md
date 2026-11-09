# `show yang doc`

Generate command reference docs from YANG schemas\.

## Ze command

- Registry path: `show yang doc`
- Usage: `show yang doc [list] [command <command>]`
- Mode: read-only, offline
- Wire method: `ze-config-yang:show-yang-doc`
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

Produces structured documentation with descriptions\, arguments\, and usage patterns for every registered command\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `list` | flag | no | any value of this type | List every documented command | Answers the list of every documented command instead of one command\'s page\. |
| `command` | string | no | any value of this type | Command to document | The command whose page is answered\, as its words are typed\. |

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
