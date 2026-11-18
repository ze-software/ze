# `show container volume list`

Show the stored volumes\, their size and their use\.

## Ze command

- Registry path: `show container volume list`
- Usage: `show container volume list`
- Mode: Read\-only
- Wire method: `ze-container:show-volume-list`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: instance\, name\, size\, used\, state
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One row for each stored volume\: the container\, the volume\, its cap\, its use\, and whether the configuration still declares it \(in\-use\) or not \(orphan\)\.

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
