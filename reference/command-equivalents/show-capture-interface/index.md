# `show capture interface`

Capture live packets on an interface \(like tcpdump\)\.

## Ze command

- Registry path: `show capture interface`
- Usage: `show capture interface [iface <iface>] [count <count>] [duration <duration>] [snap-len <snap-len>] [format <pcap\|text>] [protocol <protocol>]`
- Mode: Read-only
- Wire method: `ze-diag:show-capture-interface`
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

Uses AF\_PACKET for zero\-copy capture\. Filter by protocol and port\. Limit with count or duration\. Output as pcap \(for Wireshark\) or text\. Snap\-len controls how many bytes per packet are captured\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `iface` | string | no | any value of this type | Interface name\. | The interface name as the configuration spells it\. Ze resolves it to the kernel device before it opens the socket\. One capture at a time runs per interface\, and a second call on the same name is refused\. |
| `count` | uint | no | any value of this type | Packet count\. | The capture ends when this many packets have been read or when the duration has elapsed\, whichever comes first\. The default is 100\. |
| `duration` | string | no | any value of this type | Capture duration\. | The time the socket stays open\, from 1s to 60s\. The default is 10s\. The capture ends earlier when the packet count is reached\. |
| `snap-len` | uint | no | any value of this type | Snap length in bytes\. | The number of bytes kept from the start of each packet\. The default is 65535\, which keeps whole packets\. The value is written into the pcap file header\. |
| `format` | enum | no | `pcap`, `text` | Output format\. | The default is pcap\. Both forms carry a packets count with the data\. |
| `protocol` | string | no | any value of this type | Protocol filter\. | A pcap\-filter word such as tcp or udp\. Ze joins it to every other word after the interface name and compiles the whole expression to a BPF program on the socket\. |

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
