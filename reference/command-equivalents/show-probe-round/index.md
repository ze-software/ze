# `show probe-round [<dest>] [<max-hops>] [<probes>] [<timeout>]`

## Ze command

- Syntax: `show probe-round [<dest>] [<max-hops>] [<probes>] [<timeout>]`
- Registry path: `show probe-round`
- Mode: Read-only
- Wire method: `ze-show:probe-round`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Run a parallel traceroute probe round to a target. Sends all probes concurrently for faster results than sequential traceroute. Returns per-hop RTT and IP. Use probes and max-hops to tune accuracy vs speed.

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
