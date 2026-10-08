# `show capture raw`

Control raw byte capture for protocol debugging\.

## Ze command

- Registry path: `show capture raw`
- Usage: `show capture raw [action <start\|stop\|dump>] [protocol <l2tp\|bgp>] [format <pcap\|json>] [count <count>]`
- Mode: Read-only
- Wire method: `ze-diag:show-capture-raw`
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

Actions\: start \(begin capturing\)\, stop \(halt\)\, dump \(retrieve\)\. Protocols\: l2tp\, bgp\. Output formats\: pcap \(for Wireshark\)\, json\. Limit with count \<N\>\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `action` | enum | no | `start`, `stop`, `dump` | Capture action\. | The verb the command performs\. It is required\, and a call without one is refused with the usage line\. |
| `protocol` | enum | no | `l2tp`, `bgp` | Protocol to capture\. | Narrows the action to one protocol\. Absent\, the action applies to every protocol with a raw capture\, and the answer carries one key for each\. |
| `format` | enum | no | `pcap`, `json` | Output format\. | Read by dump alone\. The default is json\. With pcap the answer carries a \<protocol\>\-pcap key holding the base64 file and a \<protocol\>\-packets count\. |
| `count` | uint | no | any value of this type | Maximum number of messages\. | Read by dump alone\. At most this many messages are returned for each protocol\. Absent or 0\, every captured message is returned\. |

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
