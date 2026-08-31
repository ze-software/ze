# `show ldp neighbor`

Show LDP neighbors and their session state.

## Ze command

- Registry path: `show ldp neighbor`
- Usage: `show ldp neighbor`
- Mode: Read-only
- Wire method: `ze-show:ldp-neighbor`
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

Returns peer address, transport address, session state, and hold time for each LDP neighbor.

## Arguments

No command-specific arguments listed.

## Mapping intents

### MPLS, LDP, and RSVP-TE state

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
