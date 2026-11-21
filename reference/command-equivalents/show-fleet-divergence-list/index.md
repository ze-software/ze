# `show fleet divergence list`

The managed clients that hold\, and what the hub keeps for each\.

## Ze command

- Registry path: `show fleet divergence list`
- Usage: `show fleet divergence list`
- Mode: read\-only\, offline
- Wire method: `ze-managed:fleet-divergence-list`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: client\, declared\, device\-version\, hub\-version\, edited\-at\, received\-at\, revert\, unreadable
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

On a hub\: every managed client whose held configuration this hub keeps\, with both hashes\, the two times\, the revert decision and whether a server block still declares the client\.

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
