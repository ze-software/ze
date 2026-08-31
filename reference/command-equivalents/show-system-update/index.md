# `show system update`

Check if a firmware update is available.

## Ze command

- Registry path: `show system update`
- Usage: `show system update`
- Mode: Read-only
- Wire method: `ze-show:system-update`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `history`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows the running version, latest available version, and when the last check ran. Use 'update system firmware check' to trigger an immediate re-check.

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
