# `show vrrp interface`

Show the VRRP virtual routers hosted on one interface.

## Ze command

- Registry path: `show vrrp interface`
- Usage: `show vrrp interface name <interface>`
- Mode: Read-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `name`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One interface can host several virtual routers. Each one has its own VRID and its own election. This view can therefore report more than one state for one link.

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
