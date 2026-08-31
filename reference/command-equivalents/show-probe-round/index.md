# `show probe-round`

Run a parallel traceroute probe round to a target.

## Ze command

- Registry path: `show probe-round`
- Usage: `show probe-round [dest <dest>] [probes <probes>] [max-hops <max-hops>] [timeout <timeout>]`
- Mode: Read-only
- Wire method: `ze-show:probe-round`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Sends all probes concurrently for faster results than sequential traceroute. Returns per-hop RTT and IP. Use probes and max-hops to tune accuracy vs speed.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `dest` | string | no | any value of this type |
| `probes` | uint | no | any value of this type |
| `max-hops` | uint | no | any value of this type |
| `timeout` | string | no | any value of this type |

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
