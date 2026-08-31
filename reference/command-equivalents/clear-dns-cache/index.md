# `clear dns cache`

Flush all DNS cache entries and reset all DNS cache counters.

## Ze command

- Registry path: `clear dns cache`
- Usage: `clear dns cache`
- Mode: Daemon
- Wire method: `ze-clear:dns-cache`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `record`, `stats`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

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
