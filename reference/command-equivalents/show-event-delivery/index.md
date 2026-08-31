# `show event delivery`

Show which peers feed which attached processes.

## Ze command

- Registry path: `show event delivery`
- Usage: `show event delivery`
- Mode: Read-only
- Wire method: `ze-show:event-delivery`
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

One block per peer, one row per `attach process` block on it: the event types that process is fed, and the message types it may send toward that peer. A token the event registry does not know is listed as unresolved and carries no edge. Answer to 'why does my program see nothing from this peer'.

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
