# `request interface migrate`

Move IP addresses between interfaces with minimal downtime.

## Ze command

- Registry path: `request interface migrate`
- Usage: `request interface migrate from <source> to <destination> address <prefix> [create <dummy\|veth\|bridge>] [timeout <duration>]`
- Mode: Daemon
- Wire method: `ze-iface:interface-migrate`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `address`, `create`, `from`, `timeout`, `to`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Takes a source interface, a target interface, and the address to move. Adds addresses to the target before removing them from the source (make-before-break).

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
