# `show data check`

Check the integrity of the store\'s entries and config history\.

## Ze command

- Registry path: `show data check`
- Usage: `show data check`
- Mode: read\-only\, offline
- Wire method: `ze-config-storage:show-data-check`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: path\, entries\, corrupt\-entries\, vanished\-entries\, history\-checked\, findings\; severity\, check\, kind\, key\, detail
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Checks every entry of the store and\, when every entry decodes\, its config history\. The answer is the entry count and one row per corrupt entry and per history finding\, with an error status when any row is an error\. It reads the store and changes nothing\, so it runs while ze runs\; the daemon holds its own writes for the walk\. ze data check prints the same rows on the host that holds the store\.

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
