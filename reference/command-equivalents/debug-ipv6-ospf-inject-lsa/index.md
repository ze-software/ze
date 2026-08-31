# `debug ipv6 ospf inject lsa`

Inject a crafted OSPFv3 LSA into the local LSDB (RFC 5340).

## Ze command

- Registry path: `debug ipv6 ospf inject lsa`
- Usage: `debug ipv6 ospf inject lsa type <ls-type> id <link-state-id> [scope <link\|area\|as>] [hex <body> ...] [withdraw]`
- Mode: Daemon
- Wire method: `ze-debug:ospfv3-inject`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `hex`, `id`, `scope`, `type`, `withdraw`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The flooding scope is derived from the LS Type S2/S1 bits (a reserved scope is rejected). Requires `debug ospf inject enable`.

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
