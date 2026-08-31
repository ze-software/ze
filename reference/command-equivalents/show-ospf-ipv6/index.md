# `show ospf ipv6`

Show the OSPFv3 (IPv6) address-family instances (RFC 5838).

## Ze command

- Registry path: `show ospf ipv6`
- Usage: `show ospf ipv6`
- Mode: Read-only
- Wire method: `ze-show:ospf-ipv6`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `database`, `graceful-restart`, `instance`, `interface`, `neighbor`, `segment-routing`, `spf`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each configured address family (ipv6-unicast, ipv6-multicast, ipv4-unicast, ipv4-multicast) with its Instance ID, router-id, and neighbor/interface counts, so multiple AF instances on a link are distinguishable.

## Arguments

No command-specific arguments listed.

## Mapping intents

### OSPF overview

Category: Routing protocols

Ze keeps OSPFv2 and OSPFv3 under one object-rooted OSPF command tree; IPv6 is a family selector.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
