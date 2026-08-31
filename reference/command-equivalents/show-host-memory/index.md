# `show host memory`

Show installed memory and ECC health.

## Ze command

- Registry path: `show host memory`
- Usage: `show host memory`
- Mode: Read-only
- Wire method: `ze-show:host-memory`
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

Returns DIMM sizes and, when the edac driver is present, correctable and uncorrectable error counters. Non-zero ECC counts mean you should plan a DIMM replacement.

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
