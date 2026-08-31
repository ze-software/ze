# `show vrrp`

Show every VRRP virtual router.

## Ze command

- Registry path: `show vrrp`
- Usage: `show vrrp`
- Mode: Read-only
- Wire method: `ze-show:vrrp`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `interface`, `statistics`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Per router: its group name, VRID, address family, state (initialize, backup, master), configured and effective priority, virtual addresses, and the macvlan device that carries the virtual MAC.

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
