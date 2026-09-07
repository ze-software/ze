# `system dispatch`

Dispatch a text command through the command dispatcher.

## Ze command

- Registry path: `system dispatch`
- Usage: `system dispatch`
- Mode: Read-only
- Wire method: `ze-system:dispatch`
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

Ze joins the words after the keyword into one command string and runs it through the text dispatcher the CLI uses. A plugin command is reachable this way. With no dispatcher, the command fails with 'dispatcher not available'.

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
