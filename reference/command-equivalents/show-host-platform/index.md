# `show host platform`

Show platform capabilities and constraints.

## Ze command

- Registry path: `show host platform`
- Usage: `show host platform`
- Mode: Read-only
- Wire method: `ze-show:host-platform`
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

Reports read-only root, privilege level, systemd presence, gokrazy update socket, reboot-allowed flag, persistent-storage writability, and fd limits. Helps you understand what operations are possible on this particular deployment.

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
