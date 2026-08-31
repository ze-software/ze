# `show ospf database`

Show the OSPF link-state database.

## Ze command

- Registry path: `show ospf database`
- Usage: `show ospf database`
- Mode: Read-only
- Wire method: `ze-show:ospf-database`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `asbr-summary`, `external`, `network`, `nssa-external`, `opaque-area`, `opaque-as`, `opaque-link`, `router`, `router-information`, `summary`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each LSA with its LS Type, Link State ID, Advertising Router, sequence number, age, and checksum.

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
