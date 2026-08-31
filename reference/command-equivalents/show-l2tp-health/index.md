# `show l2tp health`

Find your worst L2TP sessions at a glance.

## Ze command

- Registry path: `show l2tp health`
- Usage: `show l2tp health`
- Mode: Read-only
- Wire method: `ze-show:l2tp-health`
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

Sorts sessions by echo loss ratio (worst first). Shows subscriber login, session state, echo count, average RTT, and CQM bucket count. Reports how many sessions are degraded.

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
