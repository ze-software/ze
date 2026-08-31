# `update system firmware rollback`

Roll back to the previous firmware and restart.

## Ze command

- Registry path: `update system firmware rollback`
- Usage: `update system firmware rollback`
- Mode: Daemon
- Wire method: `ze-update:system-firmware-rollback`
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

Reverts to the prior image. Only available on platforms with A/B partitioning (e.g. gokrazy). Use this if the new version has issues.

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
