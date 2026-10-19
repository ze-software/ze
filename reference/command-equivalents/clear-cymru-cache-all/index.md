# `clear cymru cache all`

Remove every cached Cymru answer\.

## Ze command

- Registry path: `clear cymru cache all`
- Usage: `clear cymru cache all`
- Mode: Daemon
- Wire method: `ze-resolve:clear-cymru-cache-all`
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

Removes the address origins and the AS names\. The answer gives removed\, the entries removed in total\, and owners\, one row per owner\. The next lookup asks DNS again\.

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
