# `show arp`

## Ze command

- Syntax: `show arp`
- Registry path: `show arp`
- Mode: Read-only
- Wire method: `ze-show:arp`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show the IPv4 ARP table (shortcut for 'show neighbor ipv4'). Lists IPv4 ARP entries with MAC address and state. ARP is IPv4-only; use 'show neighbor' for both families or 'show neighbor ipv6' for the IPv6 ND table.

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
