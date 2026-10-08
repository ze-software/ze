# `support`

Collect logs\, config\, state and diagnostics into one archive\.

## Ze command

- Registry path: `support`
- Usage: `support`
- Mode: Daemon
- Wire method: `ze-support:support`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

The archive holds logs\, config\, state and diagnostics\, and is what to send when you report an issue\. Secrets are redacted unless asked for\. Modules can be selected or excluded\, and a time window narrows the logs the archive holds\.

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
