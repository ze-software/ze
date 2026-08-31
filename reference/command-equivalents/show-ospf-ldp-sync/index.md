# `show ospf ldp-sync`

Show OSPF LDP-IGP synchronization state (RFC 5443, RFC 6138).

## Ze command

- Registry path: `show ospf ldp-sync`
- Usage: `show ospf ldp-sync`
- Mode: Read-only
- Wire method: `ze-show:ospf-ldp-sync`
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

Lists each ldp-sync interface with its state (not-synchronized / hold-down / synchronized), remaining hold-down, effective metric, and whether it is stuck not-synchronized after having been synchronized.

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
