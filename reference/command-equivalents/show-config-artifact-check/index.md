# `show config artifact check`

Check a private artifact or binary source\.

## Ze command

- Registry path: `show config artifact check`
- Usage: `show config artifact <artifact> check [transfer <transfer>]`
- Mode: Read\-only
- Wire method: `ze-config:artifact-check`
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

Run the native frame and history integrity checker on exactly one private artifact or completed binary source\. Failed verdicts retain their finding rows\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `artifact` | string | no | any value of this type | Private artifact to check\. | Private artifact to inspect\; select this or transfer\, never both\. |
| `transfer` | string | no | any value of this type | Complete source to check\. | Completed private binary source\, including a corrupt artifact\; select this or artifact\, never both\. |

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
