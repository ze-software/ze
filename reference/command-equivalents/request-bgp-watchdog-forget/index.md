# `request bgp watchdog forget`

Withdraw and forget the routes an owner gave watchdogs\.

## Ze command

- Registry path: `request bgp watchdog forget`
- Usage: `request bgp watchdog forget <owner>`
- Mode: Daemon
- Wire method: `ze-bgp:watchdog-forget`
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

Ze takes every route the owner gave any watchdog off it\, and withdraws the ones it holds announced\. A later announce does not bring them back\. The ExaBGP bridge sends it when the process that gave the routes exits\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `owner` | string | yes | any value of this type | Owner of the given routes | The owner named when the routes were given\. |

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
