# `show doctor`

Check if this box is ready to run Ze.

## Ze command

- Registry path: `show doctor`
- Usage: `show doctor`
- Mode: Read-only
- Wire method: `ze-show:doctor`
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

Verifies runtime dependencies: required files, sockets, ports, and kernel modules. Each check reports pass or fail with a reason. Run this before first start or after changing the platform setup.

## Arguments

No command-specific arguments listed.

## Mapping intents

### CPU, memory, platform, and host health

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show hardware cpu` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
- `show system memory` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
