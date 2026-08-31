# `show ospf spf`

Show recent OSPF SPF runs.

## Ze command

- Registry path: `show ospf spf`
- Usage: `show ospf spf`
- Mode: Read-only
- Wire method: `ze-show:ospf-spf`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Returns the most recent per-area SPF runs with their timestamp, duration, node count, and pending state.

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
