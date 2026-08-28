# `show ospf ipv6 interface`

## Ze command

- Syntax: `show ospf ipv6 interface`
- Registry path: `show ospf ipv6 interface`
- Mode: Read-only
- Wire method: `ze-show:ospf-ipv6-interface`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show OSPFv3 (IPv6-family) interfaces and their RFC 4552 IPsec status. Returns per interface whether IPsec is configured, the protocol (ah/esp) and SPI, and whether the kernel SA is installed. The key is never shown.

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
