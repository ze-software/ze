# `show bgp rib rpf`

Reverse-path forwarding lookup in the Loc-RIB.

## Ze command

- Registry path: `show bgp rib rpf`
- Usage: `show bgp rib rpf`
- Mode: Read-only
- Wire method: `ze-rib-api:rpf`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: source, next-hop
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Performs a longest-prefix-match and returns the best-path entry. Use this to verify RPF checks would pass for a given source.

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
