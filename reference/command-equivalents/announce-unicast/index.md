# `announce unicast`

Originate a unicast route on demand.

## Ze command

- Registry path: `announce unicast`
- Usage: `announce unicast <prefix> [next-hop <address>] [community <value> ...] [tag <key> <value>] [for <duration>]`
- Mode: Daemon
- Wire method: `ze-bgp:announce-unicast`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `community`, `for`, `next-hop`, `tag`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

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
