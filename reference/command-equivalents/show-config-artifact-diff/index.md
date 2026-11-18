# `show config artifact diff`

Compare artifact configurations or a revision\.

## Ze command

- Registry path: `show config artifact diff`
- Usage: `show config artifact <artifact> diff <name> [other <other>] [revision <revision>]`
- Mode: Read\-only
- Wire method: `ze-config:artifact-diff`
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

Compare resolved configurations or one history revision\, with the native secret masking and ordered\-list semantics\. Select exactly one of other or revision\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `other` | string | no | any value of this type | Other artifact configuration\. | Another configuration in the same artifact\. Select this or revision\, never both\. |
| `revision` | uint | no | any value of this type | History revision to compare\. | One\-based native history revision of name\. Select this or other\, never both\. |
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
