# `show ospf ipv6 database`

## Ze command

- Syntax: `show ospf ipv6 database`
- Registry path: `show ospf ipv6 database`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-database`
- Global pipes: yes

Show the OSPFv3 (IPv6) link-state database with each native scope-aware LSA decoded (RFC 5340). Base types decode into named fields; unknown function codes fall back to a scope-aware header + body-hex view (spec-ospf-ext-14).

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
