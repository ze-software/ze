# `send bgp raw`

Send raw bytes into a peer\'s TCP stream \(dangerous\)\.

## Ze command

- Registry path: `send bgp raw`
- Usage: `send bgp <selector> raw <hex\|b64> <data> [type <open\|update\|notification\|keepalive\|route-refresh>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-raw`
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

Injects arbitrary bytes with no BGP framing and no validation\. It is for conformance testing and fuzzing only\, and it will break the session if it is used carelessly\. With a type keyword ze writes the marker and the header\, and the data carries the message body alone\. Without a type keyword the data carries the whole packet\, marker and header included\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `encoding` | enum | yes | `hex`, `b64` | How the data below is encoded\. | The word is required and it is typed before the data\. Ze decodes the data with it\, and data the decoder refuses is an error\. |
| `data` | string | yes | any value of this type | The bytes to send\, in the encoding above\. | The value is one token\, the octets in the encoding named before it\. With a type keyword after it\, the octets are the message body alone\. Without one\, they are the whole packet\, marker and header included\. |
| `type` | enum | no | `open`, `update`, `notification`, `keepalive`, `route-refresh` | The BGP message type ze writes a header for\. | The word is optional and any other word there is refused\. With it\, Ze writes the 16\-byte marker and the length and type fields\, and the data is the body\. Absent\, the data is sent as it is\. |

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
