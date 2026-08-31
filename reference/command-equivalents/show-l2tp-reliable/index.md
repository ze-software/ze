# `show l2tp reliable`

Show the reliable transport window for a tunnel.

## Ze command

- Registry path: `show l2tp reliable`
- Usage: `show l2tp reliable`
- Mode: Read-only
- Wire method: `ze-l2tp-api:reliable`
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

Returns send and receive sequence numbers, window size, and retransmit queue depth. Check here when tunnel control messages stop moving.

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
