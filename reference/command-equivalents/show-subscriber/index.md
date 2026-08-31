# `show subscriber`

Show a summary of all subscriber sessions.

## Ze command

- Registry path: `show subscriber`
- Usage: `show subscriber`
- Mode: Read-only
- Wire method: `ze-subscriber-api:summary`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `id`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Counts by access type (PPPoE, L2TP, IPoE) with totals. It is the quick way to see how many subscribers are online.

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
