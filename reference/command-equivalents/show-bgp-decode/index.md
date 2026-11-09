# `show bgp decode`

Decode a hex\-encoded BGP message into readable JSON\.

## Ze command

- Registry path: `show bgp decode`
- Usage: `show bgp decode [type <open\|update\|notification\|keepalive>] [nlri <nlri>] [family <family>] [pcap <pcap>] [plugin <plugin>] [message <message>]`
- Mode: read-only, offline
- Wire method: `ze-bgp:show-decode`
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

Paste a hex BGP UPDATE and get back parsed attributes\, NLRI\, and withdrawn prefixes\. Use it to read pcap captures or to debug wire issues\. The web UI carries the same tool under tools\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `type` | enum | no | `open`, `update`, `notification`, `keepalive` | Message type to decode as | Reads the message as this type\. Without it\, a message with a BGP header is read as the type its header names\, and one without a header as an UPDATE body\. |
| `nlri` | string | no | any value of this type | Decode as NLRI of this family | Reads the payload as NLRI alone\, of this address family \(afi\/safi\)\. |
| `family` | string | no | any value of this type | Address family of an UPDATE | The address family an UPDATE body is decoded under \(afi\/safi\)\. |
| `pcap` | string | no | any value of this type | Capture file to decode | A packet capture to read every BGP message from\, in the order the messages completed on the wire\. |
| `plugin` | string | no | any value of this type | Plugin that decodes first | A registered plugin that decodes capabilities and NLRI before the other registered decoders\. The name alone forks the plugin\, ze\.\<name\> runs it in a goroutine and ze\-\<name\> calls it in\-process\. A name no registered plugin decodes under is refused\. A program path is taken only by the local ze bgp decode command\'s own option\, since it names a program on the host the decode runs on\. |
| `message` | string | no | any value of this type | Hexadecimal message | The message in hexadecimal\. Colons and spaces between the digits are stripped\, so it may span several words\. |

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
