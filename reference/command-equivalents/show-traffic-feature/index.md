# `show traffic feature`

Show neutral per-source traffic feature signals.

## Ze command

- Registry path: `show traffic feature`
- Usage: `show traffic feature [name <name>]`
- Mode: Read-only
- Wire method: `ze-show:traffic-feature`
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

The signals are fan-out (distinct destinations), out/in byte ratio (exfiltration), destination-port entropy, new-peer, rare-port/proto, and coarse beaconing. Without arguments, shows the top source entities. With 'name <address>', filters to one source.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | no | any value of this type |

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
