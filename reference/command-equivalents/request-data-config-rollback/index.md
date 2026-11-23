# `request data config rollback`

Restore one stored revision through the configuration owner\.

## Ze command

- Registry path: `request data config rollback`
- Usage: `request data config rollback <name> <revision>`
- Mode: Daemon
- Wire method: `ze-config:data-config-rollback`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, when the answer has rows: match\, count\, first\, last\, display\, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Publish a selected stored revision as a new active version\, retaining the previous active version as rollback\. Revision numbers follow the existing history listing\: 1 is the most recent dated entry\. A staged candidate\, a pending confirmed window or an unfinished file publication refuses the write\. The running daemon\'s own configuration is staged and reloaded through its normal transaction\; other configurations use the same guarded version publication as local rollback\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | Stored config filename\. | Stored configuration name\, without a directory\. |
| `revision` | uint | yes | any value of this type | History revision number\, starting at 1 for the most recent\. | One\-based revision number in the same descending timestamp order as the existing configuration history list\. No other revision is substituted\. |

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
