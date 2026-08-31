# `show neighbor`

Show the ARP and neighbor discovery table.

## Ze command

- Registry path: `show neighbor`
- Usage: `show neighbor [family <ipv4\|ipv6\|any\|all>]`
- Mode: Read-only
- Wire method: `ze-show:neighbor`
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

Lists IPv4 ARP and IPv6 ND entries with MAC addresses and states. Pass ipv4 or ipv6 to filter by address family. No argument shows both. For the IPv4-only view, 'show arp' is a shortcut.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `family` | enum | no | `ipv4`, `ipv6`, `any`, `all` |

## Mapping intents

### ARP and neighbor cache

Category: Neighbors

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show arp` (verified, vyos-cli)
  - Intent: ARP and neighbor cache
