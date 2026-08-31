# `show l2tp session traffic`

Show traffic counters for a subscriber's PPP interface.

## Ze command

- Registry path: `show l2tp session traffic`
- Usage: `show l2tp session traffic`
- Mode: Read-only
- Wire method: `ze-l2tp-api:session-traffic`
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

Returns byte and packet counts, error counters, and current rates. Compare with CQM data to get the full picture of subscriber health.

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
