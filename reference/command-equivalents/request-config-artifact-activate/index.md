# `request config artifact activate`

Activate an artifact configuration path\.

## Ze command

- Registry path: `request config artifact activate`
- Usage: `request config artifact <artifact> activate <name> <text> [dry-run]`
- Mode: Daemon
- Wire method: `ze-config-cli:artifact-activate`
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

Apply one native activate statement to an artifact configuration\, preserving the complete surrounding artifact\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `artifact` | string | yes | any value of this type | Artifact\. | Artifact for this private operation\; ownership comes only from authentication\. |
| `dry-run` | flag | no | any value of this type | Preview without publishing\. | Return the native masked difference without changing the retained artifact\. |
| `name` | string | yes | any value of this type | Name\. | Name for this private operation\; ownership comes only from authentication\. |
| `text` | string | yes | any value of this type | Completed private editing text transfer\. | Completed private text transfer containing one editing statement\, or replacement configuration for replace\. The transfer belongs to the authenticated account\. |

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
