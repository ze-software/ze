# `update serve pause`

Toggle the update server\'s pause\.

## Ze command

- Registry path: `update serve pause`
- Usage: `update serve pause`
- Mode: Daemon
- Wire method: `ze-update:update-serve-pause`
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

Each call toggles the pause\, and the answer says which state it left\. A paused server still answers\, and its manifest says paused\, so update clients hold off\. The update\-paused file beside the binary pauses it too\, whatever this state says\.

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
