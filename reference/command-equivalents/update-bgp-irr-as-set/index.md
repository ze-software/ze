# `update bgp irr as-set`

Refresh IRR prefix\-list for a specific AS\-SET\.

## Ze command

- Registry path: `update bgp irr as-set`
- Usage: `update bgp irr as-set <as-set>`
- Mode: Daemon
- Wire method: `ze-bgp:update-irr-as-set`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Re\-queries the IRR server for all peers using the given AS\-SET name\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `as-set` | string | yes | any value of this type | AS\-SET name\. | The AS\-SET name as the peer configuration spells it\, compared as an exact string\. Every enrolled ASN that resolves through this AS\-SET is refreshed\, and a name no enrolled ASN uses is refused\. |

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
