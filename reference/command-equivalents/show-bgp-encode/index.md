# `show bgp encode`

Turn a route announcement into wire\-format hex\.

## Ze command

- Registry path: `show bgp encode`
- Usage: `show bgp encode [family <family>] [local-as <local-as>] [peer-as <peer-as>] [path-id <include\|omit>] [part <message\|body\|nlri>] [asn-width <two-octet\|four-octet>] [announcement <announcement>]`
- Mode: read-only, offline
- Wire method: `ze-bgp:show-encode`
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

Takes a route in API syntax and returns the BGP UPDATE as a hex string\. Use it to build a test payload\, to feed ze\-test\, or to verify that an announcement encodes correctly\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `family` | string | no | any value of this type | Address family of the route | The address family the route is encoded for \(afi\/safi\)\. Without it\, ipv4\/unicast\. |
| `local-as` | uint | no | any value of this type | Local AS number | The local AS number the UPDATE is built for\. Without it\, 65533\. |
| `peer-as` | uint | no | any value of this type | Peer AS number | The peer AS number\. Equal to local\-as\, the UPDATE is built for an iBGP session\. Without it\, 65533\. |
| `path-id` | enum | no | `include`, `omit` | ADD\-PATH path identifier | Whether each NLRI carries an ADD\-PATH path identifier \(RFC 7911\)\. Without it\, omit\. |
| `part` | enum | no | `message`, `body`, `nlri` | Part of the UPDATE to answer | Which part of the UPDATE the answer carries\: the whole message with its 19\-octet header\, the body without the header\, or the NLRI alone\. Without it\, message\. |
| `asn-width` | enum | no | `two-octet`, `four-octet` | AS number encoding width | How AS numbers are encoded\: in two octets \(RFC 4271\) or in four \(RFC 6793\)\. Without it\, four\-octet\. |
| `announcement` | string | no | any value of this type | Route to encode | The route in the API announcement syntax\, such as route 10\.0\.0\.0\/24 next\-hop 192\.0\.2\.1\. |

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
