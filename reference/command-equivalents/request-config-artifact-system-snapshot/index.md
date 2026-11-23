# `request config artifact system snapshot`

Create a complete private system backup transfer\.

## Ze command

- Registry path: `request config artifact system snapshot`
- Usage: `request config artifact system snapshot`
- Mode: Daemon
- Wire method: `ze-config:artifact-system-snapshot`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Create a complete immutable ZeFS transfer from the daemon\-owned store\, including credentials\, history and saved drafts\. Requires edit access\; read the returned transfer and abort it afterward\. No source configuration is changed\.

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
