# `show capture interface`

Capture live packets on an interface (like tcpdump).

## Ze command

- Registry path: `show capture interface`
- Usage: `show capture interface [iface <iface>] [count <count>] [duration <duration>] [snap-len <snap-len>] [format <pcap\|text>] [protocol <protocol>]`
- Mode: Read-only
- Wire method: `ze-show:capture-interface`
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

Uses AF_PACKET for zero-copy capture. Filter by protocol and port. Limit with count or duration. Output as pcap (for Wireshark) or text. Snap-len controls how many bytes per packet are captured.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `iface` | string | no | any value of this type |
| `count` | uint | no | any value of this type |
| `duration` | string | no | any value of this type |
| `snap-len` | uint | no | any value of this type |
| `format` | enum | no | `pcap`, `text` |
| `protocol` | string | no | any value of this type |

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
