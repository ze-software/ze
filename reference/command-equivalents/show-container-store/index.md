# `show container store`

Show the image store\'s place\, its space and its layers\.

## Ze command

- Registry path: `show container store`
- Usage: `show container store`
- Mode: Read\-only
- Wire method: `ze-container:show-store`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The store\'s folder\, the total and free bytes of its filesystem\, the reserve a fetch never takes\, whether the kernel enforces the size caps\, and one row for each writable layer with its cap and its use\.

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
