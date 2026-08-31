# `show isis route ipv6`

Show IS-IS-computed IPv6 routes (RFC 5308).

## Ze command

- Registry path: `show isis route ipv6`
- Usage: `show isis route ipv6`
- Mode: Read-only
- Wire method: `ze-show:isis-route-ipv6`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each IPv6 prefix the SPF installed with its metric, level, and next-hops (link-local address and outgoing interface).

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
