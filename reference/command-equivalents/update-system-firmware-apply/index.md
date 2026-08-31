# `update system firmware apply`

Full upgrade: download, verify, stage, and restart.

## Ze command

- Registry path: `update system firmware apply`
- Usage: `update system firmware apply`
- Mode: Daemon
- Wire method: `ze-update:system-firmware-apply`
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

Runs the complete update cycle in one command. Only available on platforms where Ze owns the update lifecycle (e.g. gokrazy). The box will reboot into the new version.

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
