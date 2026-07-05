# `show ospf ipv6`

## Ze command

- Syntax: `show ospf ipv6`
- Registry path: `show ospf ipv6`
- Mode: Read-only
- Wire method: `ze-show:ospf-ipv6`
- Global pipes: yes

Show the OSPFv3 (IPv6) address-family instances (RFC 5838). Lists each configured address family (ipv6-unicast, ipv6-multicast, ipv4-unicast, ipv4-multicast) with its Instance ID, router-id, and neighbor/interface counts, so multiple AF instances on a link are distinguishable.

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
