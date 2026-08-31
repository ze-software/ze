# `show vpn ipsec sa`

Show all IKE and Child Security Associations.

## Ze command

- Registry path: `show vpn ipsec sa`
- Usage: `show vpn ipsec sa`
- Mode: Read-only
- Wire method: `ze-show:vpn-ipsec-sa`
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

Lists every SA with peer, negotiated algorithms, byte counts, rekey timers, and uptime. Includes SPIs, NAT detection, and child SA traffic selectors. It is the main IPsec status command.

## Arguments

No command-specific arguments listed.

## Mapping intents

### IPsec security associations

Category: VPN and access

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
