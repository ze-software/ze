# `debug ip ospf inject opaque`

Inject a crafted IPv4 opaque LSA into the local LSDB (RFC 5250).

## Ze command

- Registry path: `debug ip ospf inject opaque`
- Usage: `debug ip ospf inject opaque scope <link\|area\|as> id <opaque-id> [type <type>] [hex <body> ...] [tlv <type> <value-hex> ...] [withdraw]`
- Mode: Daemon
- Wire method: `ze-debug:ospf-inject`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `hex`, `id`, `scope`, `tlv`, `type`, `withdraw`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The default Opaque Type is Private-Use so a test LSA never collides with a standards-track consumer. Requires `debug ospf inject enable`.

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
