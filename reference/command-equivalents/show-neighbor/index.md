# `show neighbor [<family>]`

## Ze command

- Syntax: `show neighbor [<family>]`
- Registry path: `show neighbor`
- Mode: Read-only
- Wire method: `ze-show:neighbor`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show the ARP and neighbor discovery table. Lists IPv4 ARP and IPv6 ND entries with MAC addresses and states. Pass ipv4 or ipv6 to filter by address family; no argument shows both. For the IPv4-only view, 'show arp' is a shortcut.

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
