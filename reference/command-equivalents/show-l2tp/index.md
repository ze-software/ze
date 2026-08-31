# `show l2tp`

L2TP tunnel, session, and subscriber state.

## Ze command

- Registry path: `show l2tp`
- Usage: `show l2tp`
- Mode: Read-only
- Wire method: `ze-l2tp-api:summary`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `config`, `cqm`, `echo`, `health`, `listeners`, `observer`, `reliable`, `session`, `sessions`, `statistics`, `tunnel`, `tunnels`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Without a subcommand, shows a summary of tunnels and sessions.

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
