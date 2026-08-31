# `show ospf ipv6 interface`

Show OSPFv3 (IPv6-family) interfaces and their RFC 4552 IPsec status.

## Ze command

- Registry path: `show ospf ipv6 interface`
- Usage: `show ospf ipv6 interface`
- Mode: Read-only
- Wire method: `ze-show:ospf-ipv6-interface`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns per interface whether IPsec is configured, the protocol (ah/esp) and SPI, and whether the kernel SA is installed. The key is never shown.

## Arguments

No command-specific arguments listed.

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
