# `request reload`

Reload the configuration without restarting.

## Ze command

- Registry path: `request reload`
- Usage: `request reload`
- Mode: Daemon
- Wire method: `ze-system:daemon-reload`
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

Ze reads the config from disk, verifies it with every plugin that asked for the config roots, and then applies it to each. Every plugin verifies before any plugin applies. A failure at any step answers 'reload failed: <error>'.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Reload, reboot, halt, or shut down

Category: Lifecycle

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `poweroff` (verified, vyos-cli)
  - Intent: Reload, reboot, halt, or shut down
- `reboot` (verified, vyos-cli)
  - Intent: Reload, reboot, halt, or shut down
