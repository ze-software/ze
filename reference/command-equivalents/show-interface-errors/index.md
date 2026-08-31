# `show interface errors`

Show interfaces that have errors or drops.

## Ze command

- Registry path: `show interface errors`
- Usage: `show interface errors`
- Mode: Read-only
- Wire method: `ze-show:interface-errors`
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

Filters to only interfaces with non-zero Rx/Tx error or drop counters. It is the quick way to find troubled links.

## Arguments

No command-specific arguments listed.

## Mapping intents

### Interface counters and error counters

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
