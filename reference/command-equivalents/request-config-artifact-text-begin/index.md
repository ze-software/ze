# `request config artifact text begin`

Begin bounded configuration text upload\.

## Ze command

- Registry path: `request config artifact text begin`
- Usage: `request config artifact text begin [total <total>]`
- Mode: Daemon
- Wire method: `ze-config:artifact-text-begin`
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

Begin bounded configuration text upload\. The private owner checks the authenticated account\, cancellation and resource allowance\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `total` | uint | no | any value of this type | Total\. | Total for this private operation\; ownership comes only from authentication\. |

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
