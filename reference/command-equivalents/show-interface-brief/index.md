# `show interface brief`

One-line summary per interface: name, state, IP, and MTU.

## Ze command

- Registry path: `show interface brief`
- Usage: `show interface brief`
- Mode: Read-only
- Wire method: `ze-show:interface-brief`
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

It is the quick way to see what is up and what addresses are assigned.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Interface list and brief status

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show interfaces` (verified, vyos-cli)
  - Intent: Interface list and brief status
