# `show ddos incidents`

Show the recent DDoS incident ring (newest first).

## Ze command

- Registry path: `show ddos incidents`
- Usage: `show ddos incidents`
- Mode: Read-only
- Wire method: `ze-show:ddos-incidents`
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

Per incident: the target vector (prefix/proto/port), attack family, top source addresses, peak pps/bps, start/end time, and whether it is still active.

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
