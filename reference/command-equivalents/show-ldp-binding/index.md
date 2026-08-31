# `show ldp binding`

Show LDP FEC-to-label bindings.

## Ze command

- Registry path: `show ldp binding`
- Usage: `show ldp binding`
- Mode: Read-only
- Wire method: `ze-show:ldp-binding`
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

Lists local and remote label bindings for each FEC (prefix). Use this to verify label distribution is working.

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
