# `show vpn ipsec dataplane policy`

Show the Security Policy Database the kernel holds.

## Ze command

- Registry path: `show vpn ipsec dataplane policy`
- Usage: `show vpn ipsec dataplane policy`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-dataplane-policy`
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

Lists each policy with its selector prefixes and ports, direction, priority, upper-layer protocol, if_id, tunnel endpoints, and the peer that installed it. A policy with no matching SA is the failure this command exists to show. RFC 4301 Section 4.4 keeps the SPD and the SAD separate, and so does this tree.

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
