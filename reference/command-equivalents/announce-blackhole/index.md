# `announce blackhole`

Originate a blackhole route on demand.

## Ze command

- Registry path: `announce blackhole`
- Usage: `announce blackhole <prefix> [tag <key> <value>] [for <duration>]`
- Mode: Daemon
- Wire method: `ze-bgp:announce-blackhole`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `for`, `tag`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

This command attaches the BLACKHOLE community itself, so RFC 7999 Section 3.1 narrows the fan-out to the sessions that recorded the agreement.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `prefix` | string | yes | any value of this type |

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
