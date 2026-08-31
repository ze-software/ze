# `show rsvp-te fast-reroute`

Show RSVP-TE Fast Reroute (RFC 4090) protection state.

## Ze command

- Registry path: `show rsvp-te fast-reroute`
- Usage: `show rsvp-te fast-reroute`
- Mode: Read-only
- Wire method: `ze-show:rsvp-te-fast-reroute`
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

Returns each configured facility-backup bypass LSP and each protected LSP with its armed bypass, mode, and whether local protection is available and in use.

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
