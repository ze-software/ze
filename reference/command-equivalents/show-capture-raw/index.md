# `show capture raw`

Control raw byte capture for protocol debugging.

## Ze command

- Registry path: `show capture raw`
- Usage: `show capture raw [action <start\|stop\|dump>] [protocol <l2tp\|bgp>] [format <pcap\|json>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-show:capture-raw`
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

Actions: start (begin capturing), stop (halt), dump (retrieve). Protocols: l2tp, bgp. Output formats: pcap (for Wireshark), json. Limit with count <N>.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `action` | enum | no | `start`, `stop`, `dump` |
| `protocol` | enum | no | `l2tp`, `bgp` |
| `format` | enum | no | `pcap`, `json` |
| `count` | uint | no | any value of this type |

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
