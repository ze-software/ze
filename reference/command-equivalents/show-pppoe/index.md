# `show pppoe`

PPPoE session and protocol state.

## Ze command

- Registry path: `show pppoe`
- Usage: `show pppoe`
- Mode: Read-only
- Wire method: `ze-pppoe-api:summary`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `interfaces`, `session`, `sessions`, `statistics`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Without a subcommand, shows a summary of active sessions.

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
