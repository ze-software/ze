# `show interface rate`

Show per-second traffic rates on your interfaces.

## Ze command

- Registry path: `show interface rate`
- Usage: `show interface rate`
- Mode: Read-only
- Wire method: `ze-show:interface-rate`
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

Returns rx/tx bytes and packets per second. Pass an interface name to narrow the output. Requires the rate tracker. For continuous monitoring, use 'monitor interface rate' instead.

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
