# `show traffic usage`

Show per-interface traffic byte counters captured by eBPF TCX.

## Ze command

- Registry path: `show traffic usage`
- Usage: `show traffic usage [name <name>]`
- Mode: Read-only
- Wire method: `ze-show:traffic-usage`
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

Per destination/source port and protocol counters are always present; per-IP top-talker counters appear when track-ip is enabled. Without arguments, lists all monitored interfaces. With 'name <interface>', shows that one interface.

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
