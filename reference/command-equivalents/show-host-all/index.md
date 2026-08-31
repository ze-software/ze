# `show host all`

Show the full hardware inventory in one shot.

## Ze command

- Registry path: `show host all`
- Usage: `show host all`
- Mode: Read-only
- Wire method: `ze-show:host-all`
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

Returns every section (cpu, nic, dmi, memory, thermal, storage, kernel, platform) as a single JSON response. Ideal for support bundles or automated inventory collection.

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
