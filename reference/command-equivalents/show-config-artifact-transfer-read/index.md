# `show config artifact transfer read`

Read one bounded immutable snapshot chunk\.

## Ze command

- Registry path: `show config artifact transfer read`
- Usage: `show config artifact transfer <transfer> read <offset>`
- Mode: Read\-only
- Wire method: `ze-config-cli:artifact-transfer-read`
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

Read one bounded immutable snapshot chunk\. The private owner checks the authenticated account\, cancellation and resource allowance\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `offset` | uint | yes | any value of this type | Offset\. | Offset for this private operation\; ownership comes only from authentication\. |
| `transfer` | string | yes | any value of this type | Transfer\. | Transfer for this private operation\; ownership comes only from authentication\. |

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
