# `show traffic control`

Show traffic control (QoS) configuration per interface.

## Ze command

- Registry path: `show traffic control`
- Usage: `show traffic control`
- Mode: Read-only
- Wire method: `ze-show:traffic`
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

Without arguments, lists every interface with its qdisc type and class/filter counts. With an interface name, shows the full qdisc and class breakdown. Use this to verify your shaping is applied.

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
