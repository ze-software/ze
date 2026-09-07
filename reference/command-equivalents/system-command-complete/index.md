# `system command complete`

List the completion candidates for a partial command.

## Ze command

- Registry path: `system command complete`
- Usage: `system command complete`
- Mode: Read-only
- Wire method: `ze-system:command-complete`
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

Ze completes a command NAME from the partial text. Write 'args' before the command name to complete an ARGUMENT of that command instead, in the form 'system command complete args <cmd> [<completed>...] <partial>'.

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
