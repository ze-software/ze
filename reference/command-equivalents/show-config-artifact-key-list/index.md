# `show config artifact key list`

List raw artifact keys\.

## Ze command

- Registry path: `show config artifact key list`
- Usage: `show config artifact <artifact> key list [prefix <prefix>]`
- Mode: Read\-only
- Wire method: `ze-config:artifact-key-list`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

List literal\-prefix raw keys from the existing native store inventory\. This private answer requires edit authorization\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `prefix` | string | no | any value of this type | Literal raw\-key prefix\. | Optional literal raw\-key prefix\. Omission lists the entire selected artifact\. |
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
