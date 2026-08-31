# `show yang tree`

Print the YANG tree for a module in a readable hierarchy.

## Ze command

- Registry path: `show yang tree`
- Usage: `show yang tree`
- Mode: Read-only
- Wire method: `ze-show:yang-tree`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows node types, data types, and config-vs-state annotations. Similar to 'pyang -f tree'. Useful for understanding the config or command structure.

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
