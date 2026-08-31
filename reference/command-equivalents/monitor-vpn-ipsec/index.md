# `monitor vpn ipsec`

Watch IPsec SA events as they happen.

## Ze command

- Registry path: `monitor vpn ipsec`
- Usage: `monitor vpn ipsec`
- Mode: Read-only
- Wire method: `ze-monitor:vpn-ipsec`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Streams sa-up, sa-down, child-up, child-down, and child-rekey events. Use it to debug a tunnel flap or a rekey problem.

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
