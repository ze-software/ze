# `request bgp rib fastpath`

Switch or report the zero\-copy forward\-handle fast path\.

## Ze command

- Registry path: `request bgp rib fastpath`
- Usage: `request bgp rib fastpath <enable\|disable\|status>`
- Mode: Daemon
- Wire method: `ze-rib-api:fastpath`
- Backends: any backend
- Task support: forbidden: the MCP server never answers with a task handle
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

The fast path hands a forward handle to the Loc\-RIB consumer without copying the route\. enable and disable switch it\, and status reports the counters\. Each of the three answers the same counter snapshot\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `action` | enum | yes | `enable`, `disable`, `status` | enable\, disable or status | One of the three words\. Any other word is refused by name\. |

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
