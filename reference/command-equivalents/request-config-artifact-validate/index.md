# `request config artifact validate`

Validate supplied offline configuration text\.

## Ze command

- Registry path: `request config artifact validate`
- Usage: `request config artifact <artifact> validate <base> <name> <text>`
- Mode: Daemon
- Wire method: `ze-config:artifact-validate`
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

Validate supplied offline configuration text\. The private owner checks the authenticated account\, cancellation and resource allowance\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `artifact` | string | yes | any value of this type | Artifact\. | Artifact for this private operation\; ownership comes only from authentication\. |
| `base` | string | yes | any value of this type | Base\. | Base for this private operation\; ownership comes only from authentication\. |
| `name` | string | yes | any value of this type | Name\. | Name for this private operation\; ownership comes only from authentication\. |
| `text` | string | yes | any value of this type | Text\. | Text for this private operation\; ownership comes only from authentication\. |

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
