# `show ospf ipv6 neighbor`

Show the OSPFv3 \(IPv6\) neighbors\.

## Ze command

- Registry path: `show ospf ipv6 neighbor`
- Usage: `show ospf ipv6 neighbor [address-family <ipv6-unicast\|ipv6-multicast\|ipv4-unicast\|ipv4-multicast>]`
- Mode: Read-only
- Wire method: `ze-ospf:show-ospfv3-neighbor`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`
- Answer shape: tab
- Address fields: none
- Column order: address-family, instance-id
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Each neighbor carries the link\-local address as its identity\, the adjacency state\, the DR and BDR by Router ID\, and the dead time\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `address-family` | enum | no | `ipv6-unicast`, `ipv6-multicast`, `ipv4-unicast`, `ipv4-multicast` | Limit the answer to one OSPFv3 address family\. | Without it the view answers for every running OSPFv3 address family\, and each row names its address\-family and instance\-id\. With it the view answers for that one family\. A family that is not running is refused\, and the refusal names the running ones\; show ospf ipv6 instance lists them too\. |

## Mapping intents

### OSPF neighbor state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
