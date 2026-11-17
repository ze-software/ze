# `request restart`

Restart the daemon with graceful\-restart state while keeping the host running\.

## Ze command

- Registry path: `request restart`
- Usage: `request restart`
- Mode: Daemon
- Wire method: `ze-plugin:system-daemon-restart`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Ze writes \'restart initiated\' before it prepares an in\-place daemon restart and signals the daemon stop\. A BGP daemon writes its graceful\-restart marker and closes sessions without a shutdown notification\, through the same action SSH restart uses\. Starting the daemon again requires a supervisor configured to restart it after this stop\. This command does not reboot the host and takes no arguments\.

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
