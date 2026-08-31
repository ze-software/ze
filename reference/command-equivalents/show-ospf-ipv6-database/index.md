# `show ospf ipv6 database`

Show the OSPFv3 (IPv6) link-state database with each native scope-aware LSA decoded (RFC 5340).

## Ze command

- Registry path: `show ospf ipv6 database`
- Usage: `show ospf ipv6 database`
- Mode: Read-only
- Wire method: `ze-show:ospfv3-database`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`, `extended`, `router`, `router-information`, `scope`, `segment-routing`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Base types decode into named fields; unknown function codes fall back to a scope-aware header + body-hex view (spec-ospf-ext-14).

## Arguments

No command-specific arguments listed.

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
