# `show vrrp statistics`

Show per-virtual-router counters.

## Ze command

- Registry path: `show vrrp statistics`
- Usage: `show vrrp statistics`
- Mode: Read-only
- Wire method: `ze-show:vrrp-statistics`
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

Advertisements sent and received, priority-zero advertisements, gratuitous ARP and unsolicited neighbor advertisement bursts, receive-validation errors by reason, and the derived skew and master-down timers in microseconds (a VRRPv3 skew is sub-millisecond).

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
