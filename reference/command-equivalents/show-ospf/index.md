# `show ospf`

OSPFv2 process summary: router-id, areas, ABR/ASBR status, and stub-router (max-metric) state (RFC 2328).

## Ze command

- Registry path: `show ospf`
- Usage: `show ospf`
- Mode: Read-only
- Wire method: `ze-show:ospf`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `border-routers`, `database`, `graceful-restart`, `instance`, `interface`, `ipv6`, `ldp-sync`, `neighbor`, `route`, `segment-routing`, `spf`, `te-database`, `virtual-links`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

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
