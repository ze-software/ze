# `show config running`

Show the configuration the running daemon applied\.

## Ze command

- Registry path: `show config running`
- Usage: `show config running`
- Mode: Read-only
- Wire method: `ze-config:show-config-running`
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

The running daemon answers with the configuration it applied\, from memory\, not from a file\. A reload the daemon refused leaves this answer unchanged\, while the config file already holds the refused text\. Secret values are replaced with a placeholder\.

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
