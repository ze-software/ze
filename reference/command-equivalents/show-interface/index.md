# `show interface`

Show network interfaces on this box.

## Ze command

- Registry path: `show interface`
- Usage: `show interface`
- Mode: Read-only
- Wire method: `ze-show:interface`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `brief`, `errors`, `name`, `rate`, `scan`, `type`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Without arguments, returns all interfaces with full detail. Subcommands: brief, type <t>, errors, rate [<name>], name <name> detail, name <name> counters.

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
