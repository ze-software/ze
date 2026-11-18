# `request config artifact session commit`

Commit this account\'s artifact session\.

## Ze command

- Registry path: `request config artifact session commit`
- Usage: `request config artifact <artifact> session commit <name> [force]`
- Mode: Daemon
- Wire method: `ze-config-cli:artifact-session-commit`
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

Validate and publish this account\'s native saved session through candidate promotion\, preserving other owners and reporting conflicts and warnings\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `force` | flag | no | any value of this type | Accept native advisories and conflict overrides\. | Explicitly accept native validation advisories and native saved\-session conflict override behavior\. |
| `artifact` | string | yes | any value of this type | Artifact\. | Artifact for this private operation\; ownership comes only from authentication\. |
| `name` | string | yes | any value of this type | Name\. | Name for this private operation\; ownership comes only from authentication\. |

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
