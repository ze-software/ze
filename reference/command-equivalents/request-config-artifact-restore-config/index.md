# `request config artifact restore config`

Restore one selected artifact configuration\.

## Ze command

- Registry path: `request config artifact restore config`
- Usage: `request config artifact <artifact> restore config [name <name>] [client <client>]`
- Mode: Daemon
- Wire method: `ze-config:artifact-restore-config`
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

Read a checked active configuration from the private artifact and publish it through the current live reload or configured managed\-client owner\. Refuse ambiguous source selection and targets without a reload owner\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | no | any value of this type | Configuration name inside the artifact\. | Optional source configuration basename\. Omission uses native restore selection\: only configuration\, then the current target\'s name\, otherwise refusal\. |
| `client` | string | no | any value of this type | Configured managed client to restore\. | Optional managed client this hub is configured to serve\. Its configuration is published without reloading the hub\. |
| `artifact` | string | yes | any value of this type | Artifact\. | Artifact for this private operation\; ownership comes only from authentication\. |

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
