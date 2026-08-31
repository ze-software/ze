# `show pppoe statistics`

Show PPPoE protocol message counters.

## Ze command

- Registry path: `show pppoe statistics`
- Usage: `show pppoe statistics`
- Mode: Read-only
- Wire method: `ze-pppoe-api:statistics`
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

Returns PADI, PADO, PADR, PADS, PADT counts, active sessions, and errors. A rising PADI count with flat PADS means sessions are not completing.

## Arguments

No command-specific arguments listed.

## Mapping intents

### PPPoE subscriber sessions

Category: VPN and access

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
