# `show ospf instance`

Show the configured OSPFv2 instances (RFC 6549 Multi-Instance).

## Ze command

- Registry path: `show ospf instance`
- Usage: `show ospf instance`
- Mode: Read-only
- Wire method: `ze-show:ospf-instance`
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

Lists each Instance ID with its router-id and the size of its isolated area, interface, neighbor, and link-state database state.

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
