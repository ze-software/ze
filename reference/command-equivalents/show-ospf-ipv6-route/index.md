# `show ospf ipv6 route`

Show the OSPFv3\-computed routes of every address family \(RFC 5838\)\.

## Ze command

- Registry path: `show ospf ipv6 route`
- Usage: `show ospf ipv6 route [address-family <ipv6-unicast|ipv6-multicast|ipv4-unicast|ipv4-multicast>]`
- Mode: Read\-only
- Wire method: `ze-ospf:show-ospfv3-route`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: address\-family\, instance\-id
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each prefix an OSPFv3 SPF run computed\, with its path type\, cost\, next\-hops and area\. Each row names the address\-family and instance\-id of the engine that computed it\, so an IPv4\-unicast instance\'s routes sit beside the IPv6\-unicast ones\. show ospf route answers the OSPFv2 routes\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `address-family` | enum | no | `ipv6-unicast`, `ipv6-multicast`, `ipv4-unicast`, `ipv4-multicast` | Limit the answer to one OSPFv3 address family\. | Without it the view answers for every running OSPFv3 address family\, and each row names its address\-family and instance\-id\. With it the view answers for that one family\. A family that is not running is refused\, and the refusal names the running ones\; show ospf ipv6 instance lists them too\. |

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
