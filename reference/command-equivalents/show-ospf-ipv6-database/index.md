# `show ospf ipv6 database`

Show the OSPFv3 \(IPv6\) link\-state database with each native scope\-aware LSA decoded \(RFC 5340\)\.

## Ze command

- Registry path: `show ospf ipv6 database`
- Usage: `show ospf ipv6 database [address-family <ipv6-unicast\|ipv6-multicast\|ipv4-unicast\|ipv4-multicast>]`
- Mode: Read-only
- Wire method: `ze-ospf:show-ospfv3-database`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`, `extended`, `router`, `router-information`, `scope`, `segment-routing`
- Answer shape: tab
- Address fields: none
- Column order: address-family, instance-id
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Base types decode into named fields\; unknown function codes fall back to a scope\-aware header \+ body\-hex view \(spec\-ospf\-ext\-14\)\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `address-family` | enum | no | `ipv6-unicast`, `ipv6-multicast`, `ipv4-unicast`, `ipv4-multicast` | Limit the answer to one OSPFv3 address family\. | Without it the view answers for every running OSPFv3 address family\, and each row names its address\-family and instance\-id\. With it the view answers for that one family\. A family that is not running is refused\, and the refusal names the running ones\; show ospf ipv6 instance lists them too\. |

## Mapping intents

### OSPF link-state database

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
